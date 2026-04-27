// Package track defines the canonical Track type that flows through the pipeline.
package track

// Track represents a trending music track event.
// Last.fm fields are filled at ingestion time; Spotify fields are added during enrichment.
type Track struct {
	// Last.fm fields
	Name      string `json:"name"`
	Artist    string `json:"artist"`
	Country   string `json:"country"`
	Listeners int64  `json:"listeners"`
	Playcount int64  `json:"playcount"`

	// Spotify enrichment fields
	ID         string `json:"id,omitempty"`
	Popularity int    `json:"popularity,omitempty"`

	// Pipeline metadata
	Timestamp int64 `json:"timestamp"`
}
