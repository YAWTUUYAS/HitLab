// Package itunes enriches tracks with iTunes Search metadata (genre, release date).
// No auth required — public API.
//
// TODO: re-introduce a rate limiter (golang.org/x/time/rate, ~18 req/min) for production.
// Removed during dev to keep the ingestion cycle fast; iTunes occasionally returns 429
// which is currently swallowed by the caller.
package itunes

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/yourname/hitlab/ingestion/track"
)

type Client struct {
	httpClient *http.Client
}

func NewClient() *Client {
	return &Client{httpClient: &http.Client{}}
}

// Enrich looks up a track on iTunes and adds genre + release date.
// Mutates t in-place. No-op if the track isn't found.
func (c *Client) Enrich(t *track.Track) error {
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
