// Package itunes enriches tracks with iTunes Search metadata (genre, release date).
// No auth required — public API. Rate-limited to ~20 req/min per Apple's guidelines.
package itunes

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"golang.org/x/time/rate"

	"github.com/yourname/hitlab/ingestion/track"
)

type Client struct {
	httpClient *http.Client
	limiter    *rate.Limiter
}

// NewClient builds an iTunes client with a token-bucket limiter.
// Apple recommends max 20 req/min for the Search API; we set 18 to leave headroom.
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{},
		limiter:    rate.NewLimiter(rate.Limit(18.0/60.0), 5), // 18/min, burst of 5
	}
}

// Enrich looks up a track on iTunes and adds genre + release date.
// Mutates t in-place. Blocks if the rate limit is full.
func (c *Client) Enrich(t *track.Track) error {
	if err := c.limiter.Wait(context.Background()); err != nil {
		return fmt.Errorf("itunes rate limiter: %w", err)
	}

	term := fmt.Sprintf("%s %s", t.Artist, t.Name)
	endpoint := fmt.Sprintf(
		"https://itunes.apple.com/search?term=%s&media=music&entity=song&limit=1",
		url.QueryEscape(term),
	)

	resp, err := c.httpClient.Get(endpoint)
	if err != nil {
		return fmt.Errorf("itunes request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("itunes status %d", resp.StatusCode)
	}

	var result struct {
		Results []struct {
			PrimaryGenreName string `json:"primaryGenreName"`
			ReleaseDate      string `json:"releaseDate"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode itunes response: %w", err)
	}

	if len(result.Results) > 0 {
		t.Genre = result.Results[0].PrimaryGenreName
		t.ReleaseDate = result.Results[0].ReleaseDate
	}
	return nil
}
