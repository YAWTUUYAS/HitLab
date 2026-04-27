// Package lastfm wraps the Last.fm geo.getTopTracks endpoint.
package lastfm

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/yourname/hitlab/ingestion/track"
)

// countryNames maps ISO codes to the full names Last.fm expects.
var countryNames = map[string]string{
	"US": "United States",
	"GB": "United Kingdom",
	"FR": "France",
	"DE": "Germany",
	"BR": "Brazil",
	"MX": "Mexico",
	"NG": "Nigeria",
	"JP": "Japan",
	"ES": "Spain",
	"AU": "Australia",
}

// Client is a Last.fm API wrapper.
type Client struct {
	apiKey     string
	httpClient *http.Client
}

// NewClient builds a Last.fm client. No auth flow needed — API key in query string.
func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:     apiKey,
		httpClient: &http.Client{},
	}
}

// TopTracks returns the top tracks for a given country (ISO code).
func (c *Client) TopTracks(countryCode string) ([]track.Track, error) {
	countryName, ok := countryNames[countryCode]
	if !ok {
		return nil, fmt.Errorf("unknown country code: %s", countryCode)
	}

	endpoint := fmt.Sprintf(
		"https://ws.audioscrobbler.com/2.0/?method=geo.gettoptracks&country=%s&api_key=%s&format=json&limit=10",
		url.QueryEscape(countryName), c.apiKey,
	)

	resp, err := c.httpClient.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("lastfm request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("lastfm status %d", resp.StatusCode)
	}

	var raw struct {
		Tracks struct {
			Track []struct {
				Name      string `json:"name"`
				Playcount string `json:"playcount"`
				Listeners string `json:"listeners"`
				Artist    struct {
					Name string `json:"name"`
				} `json:"artist"`
			} `json:"track"`
		} `json:"tracks"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode lastfm response: %w", err)
	}

	tracks := make([]track.Track, 0, len(raw.Tracks.Track))
	for _, t := range raw.Tracks.Track {
		// Last.fm returns playcount/listeners as strings — parse them
		playcount, _ := strconv.ParseInt(t.Playcount, 10, 64)
		listeners, _ := strconv.ParseInt(t.Listeners, 10, 64)

		tracks = append(tracks, track.Track{
			Name:      t.Name,
			Artist:    t.Artist.Name,
			Country:   countryCode,
			Playcount: playcount,
			Listeners: listeners,
		})
	}

	return tracks, nil
}
