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

	"github.com/yourname/hitlab/ingestion/lastfm"
	"github.com/yourname/hitlab/ingestion/track"
)

var countries = []string{"US", "GB", "FR", "DE", "BR", "MX", "NG", "JP", "ES", "AU"}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found, reading from environment")
	}

	lastfmClient := lastfm.NewClient(os.Getenv("LASTFM_API_KEY"))
	log.Println("lastfm client ready")

	writer := &kafka.Writer{
		Addr:     kafka.TCP(os.Getenv("KAFKA_BROKER")),
		Topic:    os.Getenv("KAFKA_TOPIC"),
		Balancer: &kafka.LeastBytes{},
	}
	defer writer.Close()

	for {
		ingest(lastfmClient, writer)
		log.Println("sleeping 5 minutes...")
		time.Sleep(5 * time.Minute)
	}
}

// ingest fetches trending tracks from Last.fm in parallel and publishes to Kafka.
func ingest(lf *lastfm.Client, writer *kafka.Writer) {
	var wg sync.WaitGroup
	tracksCh := make(chan track.Track, 500)

	for _, country := range countries {
		wg.Add(1)
		go func(c string) {
			defer wg.Done()
			tracks, err := lf.TopTracks(c)
			if err != nil {
				log.Printf("lastfm error %s: %v", c, err)
				return
			}
			for _, t := range tracks {
				tracksCh <- t
			}
			log.Printf("lastfm: %d tracks for %s", len(tracks), c)
		}(country)
	}

	go func() {
		wg.Wait()
		close(tracksCh)
	}()

	var messages []kafka.Message
	for t := range tracksCh {
		t.Timestamp = time.Now().Unix()

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
