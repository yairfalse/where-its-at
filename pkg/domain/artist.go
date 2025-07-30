package domain

import (
	"time"
)

type Artist struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	ExternalIDs ExternalIDs `json:"external_ids"`
	Genres      []string    `json:"genres,omitempty"`
	Popularity  int         `json:"popularity,omitempty"`
	ImageURL    string      `json:"image_url,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

type ExternalIDs struct {
	SpotifyID string `json:"spotify_id,omitempty"`
	LastFMID  string `json:"lastfm_id,omitempty"`
}

type ArtistSearchRequest struct {
	Query string `json:"q"`
	Limit int    `json:"limit"`
}

type ArtistSearchResponse struct {
	Artists []Artist `json:"artists"`
	Total   int      `json:"total"`
}
