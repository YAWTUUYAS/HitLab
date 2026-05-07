package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/joho/godotenv"
	kafka "github.com/segmentio/kafka-go"

	"github.com/yourname/hitlab/ingestion/deezer"
	"github.com/yourname/hitlab/ingestion/itunes"
	"github.com/yourname/hitlab/ingestion/lastfm"
	"github.com/yourname/hitlab/ingestion/track"
)

var countries = []string{"US", "GB", "FR", "DE", "BR", "MX", "NG", "JP", "ES", "AU"}

const enrichWorkers = 5

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found, reading from environment")
	}

	lf := lastfm.NewClient(os.Getenv("LASTFM_API_KEY"))
	dz := deezer.NewClient()
	it := itunes.NewClient()
	log.Println("clients ready (lastfm + deezer + itunes)")

	writer := &kafka.Writer{
		Addr:     kafka.TCP(os.Getenv("KAFKA_BROKER")),
		Topic:    os.Getenv("KAFKA_TOPIC"),
		Balancer: &kafka.LeastBytes{},
	}
	defer writer.Close()

	for {
		ingest(lf, dz, it, writer)
		log.Println("sleeping 1 minute...")
		time.Sleep(1 * time.Minute)
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
