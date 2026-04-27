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

	"github.com/yourname/hitlab/ingestion/spotify"
)

var countries = []string{"US", "GB", "FR", "DE", "BR", "MX", "NG", "JP", "KR", "AU"}

func main() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found, reading from environment")
	}

	clientID := os.Getenv("SPOTIFY_CLIENT_ID")
	clientSecret := os.Getenv("SPOTIFY_CLIENT_SECRET")
	kafkaBroker := os.Getenv("KAFKA_BROKER")
	kafkaTopic := os.Getenv("KAFKA_TOPIC")

	// Authenticate with Spotify
	client, err := spotify.NewClient(clientID, clientSecret)
	if err != nil {
		log.Fatalf("spotify auth failed: %v", err)
	}
	log.Println("spotify authenticated")

	// Set up Kafka writer
	writer := &kafka.Writer{
		Addr:     kafka.TCP(kafkaBroker),
		Topic:    kafkaTopic,
		Balancer: &kafka.LeastBytes{},
	}
	defer writer.Close()

	// Fetch tracks every 5 minutes
	for {
		ingest(client, writer)
		log.Println("sleeping 5 minutes...")
		time.Sleep(5 * time.Minute)
	}
}

// ingest fetches trending tracks from all countries in parallel and publishes to Kafka
func ingest(client *spotify.Client, writer *kafka.Writer) {
	var wg sync.WaitGroup
	tracksCh := make(chan spotify.Track, 500)

	// Launch one goroutine per country
	for _, country := range countries {
		wg.Add(1)
		go func(c string) {
			defer wg.Done()
			tracks, err := client.FeaturedPlaylists(c)
			if err != nil {
				log.Printf("error fetching %s: %v", c, err)
				return
			}
			for _, t := range tracks {
				tracksCh <- t
			}
			log.Printf("fetched %d tracks for %s", len(tracks), c)
		}(country)
	}

	// Close channel once all goroutines finish
	go func() {
		wg.Wait()
		close(tracksCh)
	}()

	// Publish each track to Kafka
	var messages []kafka.Message
	for track := range tracksCh {
		track.Timestamp = time.Now().Unix()

		data, err := json.Marshal(track)
		if err != nil {
			log.Printf("marshal error: %v", err)
			continue
		}
		messages = append(messages, kafka.Message{
			Key:   []byte(fmt.Sprintf("%s-%s", track.Country, track.ID)),
			Value: data,
		})
	}

	if len(messages) == 0 {
		log.Println("no messages to publish")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := writer.WriteMessages(ctx, messages...); err != nil {
		log.Printf("kafka write error: %v", err)
		return
	}

	log.Printf("published %d track events to kafka", len(messages))
}
