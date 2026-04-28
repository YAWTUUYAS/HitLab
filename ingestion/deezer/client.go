// Package deezer enriches tracks with the Deezer rank (popularity proxy).
// No auth required — public API.
package deezer

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

// Enrich looks up a track on Deezer and adds rank, duration, and preview URL.
// Mutates t in-place. No-op if the track isn't found.
func (c *Client) Enrich(t *track.Track) error {
	query := fmt.Sprintf(`track:"%s" artist:"%s"`, t.Name, t.Artist)
	endpoint := fmt.Sprintf(
		"https://api.deezer.com/search?q=%s&limit=1",
		url.QueryEscape(query),
	)

	resp, err := c.httpClient.Get(endpoint)
	if err != nil {
		return fmt.Errorf("deezer request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("deezer status %d", resp.StatusCode)
	}

	var result struct {
		Data []struct {
			Rank     int64  `json:"rank"`
			Duration int    `json:"duration"`
			Preview  string `json:"preview"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode deezer response: %w", err)
	}

	if len(result.Data) > 0 {
		t.DeezerRank = result.Data[0].Rank
		t.Duration = result.Data[0].Duration
		t.PreviewURL = result.Data[0].Preview
	}
	return nil
}
