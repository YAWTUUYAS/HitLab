package main

import (
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	kafka "github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/plain"

	"github.com/yourname/hitlab/ingestion/deezer"
	"github.com/yourname/hitlab/ingestion/itunes"
	"github.com/yourname/hitlab/ingestion/lastfm"
	"github.com/yourname/hitlab/ingestion/track"
)

var countries = []string{"US", "GB", "FR", "DE", "BR", "MX", "NG", "JP", "ES", "AU"}

const enrichWorkers = 5

// urlToDSN converts a postgres:// URL into a lib/pq key=value DSN.
// This avoids URL-parsing edge cases with dots in usernames or special
// characters in passwords (e.g. Supabase pooler: user=postgres.PROJECT_REF).
func urlToDSN(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL // fallback: use as-is
	}
	user := u.User.Username()
	pass, _ := u.User.Password()
	host := u.Hostname()
	port := u.Port()
	if port == "" {
		port = "5432"
	}
	dbname := strings.TrimPrefix(u.Path, "/")
	sslmode := u.Query().Get("sslmode")
	if sslmode == "" {
		sslmode = "require"
	}
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, pass, dbname, sslmode,
	)
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found, reading from environment")
	}

	lf := lastfm.NewClient(os.Getenv("LASTFM_API_KEY"))
	dz := deezer.NewClient()
	it := itunes.NewClient()
	log.Println("clients ready (lastfm + deezer + itunes)")

	// DIRECT_DB mode: no Kafka — write straight to PostgreSQL.
	// Activated when DATABASE_URL is set and KAFKA_BROKER is not.
	dbURL := os.Getenv("DATABASE_URL")
	kafkaBroker := os.Getenv("KAFKA_BROKER")

	if dbURL != "" && kafkaBroker == "" {
		log.Println("mode: direct-to-db (no Kafka)")
		db, err := sql.Open("postgres", urlToDSN(dbURL))
		if err != nil {
			log.Fatalf("db open: %v", err)
		}
		defer db.Close()
		if err := db.Ping(); err != nil {
			log.Fatalf("db ping: %v", err)
		}
		log.Println("db connected")

		if os.Getenv("RUN_ONCE") == "true" {
			ingestDirect(lf, dz, it, db)
			return
		}
		for {
			ingestDirect(lf, dz, it, db)
			log.Println("sleeping 1 minute...")
			time.Sleep(1 * time.Minute)
		}
	}

	// Kafka mode (local / Docker).
	writer := newKafkaWriter()
	defer writer.Close()

	if os.Getenv("RUN_ONCE") == "true" {
		ingest(lf, dz, it, writer)
		return
	}
	for {
		ingest(lf, dz, it, writer)
		log.Println("sleeping 1 minute...")
		time.Sleep(1 * time.Minute)
	}
}

// newKafkaWriter builds a kafka.Writer with optional SASL/TLS for Upstash.
// When KAFKA_USERNAME is set the writer uses SASL PLAIN over TLS.
func newKafkaWriter() *kafka.Writer {
	transport := &kafka.Transport{}

	if user := os.Getenv("KAFKA_USERNAME"); user != "" {
		transport.TLS = &tls.Config{}
		transport.SASL = plain.Mechanism{
			Username: user,
			Password: os.Getenv("KAFKA_PASSWORD"),
		}
		log.Println("kafka: SASL/TLS enabled (Upstash)")
	}

	return &kafka.Writer{
		Addr:      kafka.TCP(os.Getenv("KAFKA_BROKER")),
		Topic:     os.Getenv("KAFKA_TOPIC"),
		Balancer:  &kafka.LeastBytes{},
		Transport: transport,
	}
}

// ingest runs a full collection cycle: fetch → enrich → publish.
func ingest(lf *lastfm.Client, dz *deezer.Client, it *itunes.Client, writer *kafka.Writer) {
	tracks := fetchAllCountries(lf)
	if len(tracks) == 0 {
		log.Println("no tracks fetched, skipping")
		return
	}

	enrichAll(tracks, dz, it)
	publish(tracks, writer)
}

// fetchAllCountries pulls top tracks for every country in parallel.
func fetchAllCountries(lf *lastfm.Client) []track.Track {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var all []track.Track

	for _, country := range countries {
		wg.Add(1)
		go func(c string) {
			defer wg.Done()
			ts, err := lf.TopTracks(c)
			if err != nil {
				log.Printf("lastfm error %s: %v", c, err)
				return
			}
			mu.Lock()
			all = append(all, ts...)
			mu.Unlock()
			log.Printf("lastfm: %d tracks for %s", len(ts), c)
		}(country)
	}
	wg.Wait()
	return all
}

// enrichAll runs Deezer + iTunes enrichment via a worker pool.
// Workers process tracks in parallel; each track is enriched sequentially within a worker.
func enrichAll(tracks []track.Track, dz *deezer.Client, it *itunes.Client) {
	indexCh := make(chan int, len(tracks))
	var wg sync.WaitGroup

	for w := 0; w < enrichWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range indexCh {
				if err := dz.Enrich(&tracks[idx]); err != nil {
					log.Printf("deezer enrich error: %v", err)
				}
				if err := it.Enrich(&tracks[idx]); err != nil {
					log.Printf("itunes enrich error: %v", err)
				}
			}
		}()
	}

	for i := range tracks {
		indexCh <- i
	}
	close(indexCh)
	wg.Wait()

	enriched := 0
	for _, t := range tracks {
		if t.DeezerRank > 0 || t.Genre != "" {
			enriched++
		}
	}
	log.Printf("enriched %d/%d tracks", enriched, len(tracks))
}

// publish stamps tracks with timestamps and writes them to Kafka in one batch.
func publish(tracks []track.Track, writer *kafka.Writer) {
	now := time.Now().Unix()
	messages := make([]kafka.Message, 0, len(tracks))

	for _, t := range tracks {
		t.Timestamp = now
		data, err := json.Marshal(t)
		if err != nil {
			log.Printf("marshal error: %v", err)
			continue
		}
		messages = append(messages, kafka.Message{
			Key:   []byte(fmt.Sprintf("%s-%s-%s", t.Country, t.Artist, t.Name)),
			Value: data,
		})
	}

	if len(messages) == 0 {
		log.Println("no messages to publish")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := writer.WriteMessages(ctx, messages...); err != nil {
		log.Printf("kafka write error: %v", err)
		return
	}

	log.Printf("published %d events to kafka", len(messages))
}

// ── Direct-to-DB mode (no Kafka) ─────────────────────────────────────────

func ingestDirect(lf *lastfm.Client, dz *deezer.Client, it *itunes.Client, db *sql.DB) {
	purgeOldEvents(db)
	tracks := fetchAllCountries(lf)
	if len(tracks) == 0 {
		log.Println("no tracks fetched, skipping")
		return
	}
	enrichAll(tracks, dz, it)
	writeToDB(tracks, db)
}

// purgeOldEvents deletes events older than 7 days to keep storage bounded.
func purgeOldEvents(db *sql.DB) {
	res, err := db.Exec(`DELETE FROM track_events WHERE time < NOW() - INTERVAL '7 days'`)
	if err != nil {
		log.Printf("purge error: %v", err)
		return
	}
	n, _ := res.RowsAffected()
	if n > 0 {
		log.Printf("purged %d events older than 7 days", n)
	}
}

// computeHypeScore mirrors the Python formula in transform/consumer.py.
func computeHypeScore(listeners, geoSpread, deezerRank int64) float64 {
	reach := math.Min(math.Log10(float64(listeners+1))/7.0, 1.0)
	geo := math.Min(float64(geoSpread)/10.0, 1.0)
	rank := 0.0
	if deezerRank > 0 {
		rank = math.Min(float64(deezerRank)/1_000_000.0, 1.0)
	}
	return math.Round((reach*0.30+geo*0.25+rank*0.25)*100*100) / 100
}

func writeToDB(tracks []track.Track, db *sql.DB) {
	now := time.Now().UTC()

	// Compute geo_spread per (name, artist) across the whole batch.
	type key struct{ name, artist string }
	spread := make(map[key]map[string]struct{})
	for _, t := range tracks {
		k := key{t.Name, t.Artist}
		if spread[k] == nil {
			spread[k] = make(map[string]struct{})
		}
		spread[k][t.Country] = struct{}{}
	}

	tx, err := db.Begin()
	if err != nil {
		log.Printf("db begin: %v", err)
		return
	}
	stmt, err := tx.Prepare(`
		INSERT INTO track_events
			(time, track_name, artist, country, listeners, playcount,
			 deezer_rank, duration, genre, release_date, geo_spread, hype_score)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
	`)
	if err != nil {
		log.Printf("db prepare: %v", err)
		tx.Rollback()
		return
	}
	defer stmt.Close()

	written := 0
	for _, t := range tracks {
		k := key{t.Name, t.Artist}
		geo := int64(len(spread[k]))
		score := computeHypeScore(t.Listeners, geo, t.DeezerRank)

		var releaseDate *string
		if t.ReleaseDate != "" {
			releaseDate = &t.ReleaseDate
		}
		var deezerRank *int64
		if t.DeezerRank > 0 {
			deezerRank = &t.DeezerRank
		}
		var duration *int
		if t.Duration > 0 {
			duration = &t.Duration
		}
		var genre *string
		if t.Genre != "" {
			genre = &t.Genre
		}

		if _, err := stmt.Exec(
			now, t.Name, t.Artist, t.Country,
			t.Listeners, t.Playcount,
			deezerRank, duration, genre, releaseDate,
			geo, score,
		); err != nil {
			log.Printf("db insert error: %v", err)
			continue
		}
		written++
	}

	if err := tx.Commit(); err != nil {
		log.Printf("db commit: %v", err)
		return
	}
	log.Printf("wrote %d events directly to db", written)
}
