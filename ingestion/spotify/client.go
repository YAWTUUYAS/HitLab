package spotify

// Client holds Spotify API credentials
type Client struct {
	// TODO
}

// NewClient creates an authenticated Spotify client
func NewClient(clientID, clientSecret string) *Client {
	return &Client{}
}

// FeaturedPlaylists fetches featured playlists for a given country code
func (c *Client) FeaturedPlaylists(country string) ([]Track, error) {
	// TODO
	return nil, nil
}

// Track represents a Spotify track event
type Track struct {
	ID        string
	Name      string
	Artist    string
	Country   string
	Streams   int64
	Timestamp int64
}
