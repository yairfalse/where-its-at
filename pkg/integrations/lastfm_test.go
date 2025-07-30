package integrations

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewLastFMClient(t *testing.T) {
	t.Run("successful creation", func(t *testing.T) {
		config := LastFMConfig{
			APIKey: "test-api-key",
		}

		client, err := NewLastFMClient(config)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if client == nil {
			t.Fatal("expected client, got nil")
		}
		if client.apiKey != "test-api-key" {
			t.Errorf("expected apiKey to be test-api-key, got %s", client.apiKey)
		}
	})

	t.Run("missing API key", func(t *testing.T) {
		config := LastFMConfig{}

		_, err := NewLastFMClient(config)
		if err == nil {
			t.Error("expected error for missing API key")
		}
	})
}

func TestLastFMClient_SearchArtists(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := r.URL.Query().Get("method")
		if method != "artist.search" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		response := lastFMSearchResponse{
			Results: struct {
				ArtistMatches struct {
					Artist []lastFMArtist `json:"artist"`
				} `json:"artistmatches"`
				TotalResults string `json:"opensearch:totalResults"`
			}{
				ArtistMatches: struct {
					Artist []lastFMArtist `json:"artist"`
				}{
					Artist: []lastFMArtist{
						{
							Name:      "Test Artist",
							MBID:      "test-mbid",
							URL:       "https://www.last.fm/music/Test+Artist",
							Listeners: "1000000",
							Image: []struct {
								Text string `json:"#text"`
								Size string `json:"size"`
							}{
								{Text: "https://example.com/small.jpg", Size: "small"},
								{Text: "https://example.com/large.jpg", Size: "extralarge"},
							},
						},
					},
				},
				TotalResults: "1",
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	client := &LastFMClient{
		baseURL:    mockServer.URL,
		apiKey:     "test-api-key",
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}

	t.Run("successful search", func(t *testing.T) {
		ctx := context.Background()
		artists, err := client.SearchArtists(ctx, "test", 10)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(artists) != 1 {
			t.Fatalf("expected 1 artist, got %d", len(artists))
		}
		if artists[0].Name != "Test Artist" {
			t.Errorf("expected name to be Test Artist, got %s", artists[0].Name)
		}
		if artists[0].ExternalIDs.LastFMID != "test-mbid" {
			t.Errorf("expected lastfm ID to be test-mbid, got %s", artists[0].ExternalIDs.LastFMID)
		}
		if artists[0].ImageURL != "https://example.com/large.jpg" {
			t.Errorf("expected image URL to be https://example.com/large.jpg, got %s", artists[0].ImageURL)
		}
	})
}

func TestLastFMClient_GetArtist(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := r.URL.Query().Get("method")
		if method != "artist.getinfo" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		artistName := r.URL.Query().Get("artist")
		if artistName == "not-found" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		response := lastFMArtistInfoResponse{
			Artist: struct {
				Name  string `json:"name"`
				MBID  string `json:"mbid"`
				URL   string `json:"url"`
				Image []struct {
					Text string `json:"#text"`
					Size string `json:"size"`
				} `json:"image"`
				Stats struct {
					Listeners string `json:"listeners"`
					Playcount string `json:"playcount"`
				} `json:"stats"`
				Tags struct {
					Tag []struct {
						Name string `json:"name"`
					} `json:"tag"`
				} `json:"tags"`
			}{
				Name: "Test Artist",
				MBID: "test-mbid",
				URL:  "https://www.last.fm/music/Test+Artist",
				Image: []struct {
					Text string `json:"#text"`
					Size string `json:"size"`
				}{
					{Text: "https://example.com/artist.jpg", Size: "extralarge"},
				},
				Tags: struct {
					Tag []struct {
						Name string `json:"name"`
					} `json:"tag"`
				}{
					Tag: []struct {
						Name string `json:"name"`
					}{
						{Name: "rock"},
						{Name: "alternative"},
					},
				},
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	client := &LastFMClient{
		baseURL:    mockServer.URL,
		apiKey:     "test-api-key",
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}

	t.Run("successful get", func(t *testing.T) {
		ctx := context.Background()
		artist, err := client.GetArtist(ctx, "Test Artist")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if artist.Name != "Test Artist" {
			t.Errorf("expected name to be Test Artist, got %s", artist.Name)
		}
		if artist.ExternalIDs.LastFMID != "test-mbid" {
			t.Errorf("expected lastfm ID to be test-mbid, got %s", artist.ExternalIDs.LastFMID)
		}
		if len(artist.Genres) != 2 {
			t.Errorf("expected 2 genres, got %d", len(artist.Genres))
		}
		if artist.ImageURL != "https://example.com/artist.jpg" {
			t.Errorf("expected image URL to be https://example.com/artist.jpg, got %s", artist.ImageURL)
		}
	})

	t.Run("artist not found", func(t *testing.T) {
		ctx := context.Background()
		_, err := client.GetArtist(ctx, "not-found")
		if err == nil {
			t.Error("expected error for not found artist")
		}
	})
}
