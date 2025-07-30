package domain

import (
	"testing"
	"time"
)

func TestArtist(t *testing.T) {
	t.Run("Artist struct creation", func(t *testing.T) {
		artist := Artist{
			ID:   "test-123",
			Name: "Test Artist",
			ExternalIDs: ExternalIDs{
				SpotifyID: "spotify123",
				LastFMID:  "lastfm123",
			},
			Genres:     []string{"rock", "alternative"},
			Popularity: 75,
			ImageURL:   "https://example.com/image.jpg",
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}

		if artist.ID != "test-123" {
			t.Errorf("expected ID to be test-123, got %s", artist.ID)
		}
		if artist.Name != "Test Artist" {
			t.Errorf("expected Name to be Test Artist, got %s", artist.Name)
		}
		if artist.ExternalIDs.SpotifyID != "spotify123" {
			t.Errorf("expected SpotifyID to be spotify123, got %s", artist.ExternalIDs.SpotifyID)
		}
		if artist.ExternalIDs.LastFMID != "lastfm123" {
			t.Errorf("expected LastFMID to be lastfm123, got %s", artist.ExternalIDs.LastFMID)
		}
		if len(artist.Genres) != 2 {
			t.Errorf("expected 2 genres, got %d", len(artist.Genres))
		}
		if artist.Popularity != 75 {
			t.Errorf("expected Popularity to be 75, got %d", artist.Popularity)
		}
		if artist.ImageURL != "https://example.com/image.jpg" {
			t.Errorf("expected ImageURL to be https://example.com/image.jpg, got %s", artist.ImageURL)
		}
	})

	t.Run("ArtistSearchRequest validation", func(t *testing.T) {
		req := ArtistSearchRequest{
			Query: "radiohead",
			Limit: 10,
		}

		if req.Query != "radiohead" {
			t.Errorf("expected Query to be radiohead, got %s", req.Query)
		}
		if req.Limit != 10 {
			t.Errorf("expected Limit to be 10, got %d", req.Limit)
		}
	})

	t.Run("ArtistSearchResponse creation", func(t *testing.T) {
		artists := []Artist{
			{ID: "1", Name: "Artist 1"},
			{ID: "2", Name: "Artist 2"},
		}

		response := ArtistSearchResponse{
			Artists: artists,
			Total:   2,
		}

		if len(response.Artists) != 2 {
			t.Errorf("expected 2 artists, got %d", len(response.Artists))
		}
		if response.Total != 2 {
			t.Errorf("expected Total to be 2, got %d", response.Total)
		}
	})
}
