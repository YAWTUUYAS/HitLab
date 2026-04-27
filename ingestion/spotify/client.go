package spotify

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Client holds Spotify API credentials and an authenticated HTTP client
type Client struct {
	accessToken string
	httpClient  *http.Client
}

// tokenResponse is the shape of Spotify's auth response
type tokenResponse struct {
	AccessToken string `json:"access_token"`
}

// NewClient authenticates with Spotify using Client Credentials flow
func NewClient(clientID, clientSecret string) (*Client, error) {
	body := strings.NewReader("grant_type=client_credentials")

	req, err := http.NewRequest("POST", "https://accounts.spotify.com/api/token", body)
	if err != nil {
		return nil, fmt.Errorf("create token request: %w", err)
	}

	credentials := base64.StdEncoding.EncodeToString([]byte(clientID + ":" + clientSecret))
	req.Header.Set("Authorization", "Basic "+credentials)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("spotify auth failed: status %d", resp.StatusCode)
	}

	var tokenResp tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}

	return &Client{
		accessToken: tokenResp.AccessToken,
		httpClient:  &http.Client{},
	}, nil
}

// FeaturedPlaylists fetches featured playlists for a given country code
func (c *Client) FeaturedPlaylists(country string) ([]Track, error) {
	url := fmt.Sprintf("https://api.spotify.com/v1/browse/featured-playlists?country=%s&limit=10", country)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create playlists request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send playlists request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("spotify playlists failed: status %d — %s", resp.StatusCode, string(b))
	}

	var result struct {
		Playlists struct {
			Items []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"items"`
		} `json:"playlists"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode playlists response: %w", err)
	}

	var tracks []Track
	for _, item := range result.Playlists.Items {
		tracks = append(tracks, Track{
			ID:      item.ID,
			Name:    item.Name,
			Country: country,
		})
	}
	return tracks, nil
}

// Track represents a trending track event
type Track struct {
	ID        string
	Name      string
	Artist    string
	Country   string
	Streams   int64
	Timestamp int64
}
