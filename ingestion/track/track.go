// Package track defines the canonical Track type that flows through the pipeline.
package track

// Track represents a trending music track event.
// Last.fm fields are filled at ingestion. Deezer + iTunes fields are added during enrichment.
type Track struct {
	// Last.fm — primary source
	Name      string `json:"name"`
	Artist    string `json:"artist"`
	Country   string `json:"country"`
	Listeners int64  `json:"listeners"`
	Playcount int64  `json:"playcount"`

	// Deezer enrichment
	DeezerRank int64  `json:"deezer_rank,omitempty"` // 0–1,000,000 popularity proxy
	Duration   int    `json:"duration,omitempty"`    // seconds
	PreviewURL string `json:"preview_url,omitempty"`

	// iTunes enrichment
	Genre       string `json:"genre,omitempty"`
	ReleaseDate string `json:"release_date,omitempty"` // ISO 8601

	// Pipeline metadata
	Timestamp int64 `json:"timestamp"`
}
