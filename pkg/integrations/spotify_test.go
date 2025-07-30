package integrations

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewSpotifyClient(t *testing.T) {
	t.Run("successful creation", func(t *testing.T) {
		config := SpotifyConfig{
			ClientID:     "test-id",
			ClientSecret: "test-secret",
		}

		client, err := NewSpotifyClient(config)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if client == nil {
			t.Fatal("expected client, got nil")
		}
		if client.clientID != "test-id" {
			t.Errorf("expected clientID to be test-id, got %s", client.clientID)
		}
		if client.clientSecret != "test-secret" {
			t.Errorf("expected clientSecret to be test-secret, got %s", client.clientSecret)
		}
	})

	t.Run("missing client ID", func(t *testing.T) {
		config := SpotifyConfig{
			ClientSecret: "test-secret",
		}

		_, err := NewSpotifyClient(config)
		if err == nil {
			t.Error("expected error for missing client ID")
		}
	})

	t.Run("missing client secret", func(t *testing.T) {
		config := SpotifyConfig{
			ClientID: "test-id",
		}

		_, err := NewSpotifyClient(config)
		if err == nil {
			t.Error("expected error for missing client secret")
		}
	})
}

func TestSpotifyClient_SearchArtists(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/token":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(spotifyTokenResponse{
				AccessToken: "test-token",
				TokenType:   "Bearer",
				ExpiresIn:   3600,
			})
		case "/v1/search":
			query := r.URL.Query().Get("q")
			if query == "" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			response := spotifySearchResponse{
				Artists: struct {
					Items []spotifyArtist `json:"items"`
					Total int             `json:"total"`
				}{
					Items: []spotifyArtist{
						{
							ID:         "test-id",
							Name:       "Test Artist",
							Genres:     []string{"rock"},
							Popularity: 75,
							Images: []struct {
								URL string `json:"url"`
							}{
								{URL: "https://example.com/image.jpg"},
							},
						},
					},
					Total: 1,
				},
			}
			json.NewEncoder(w).Encode(response)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer mockServer.Close()

	client := &SpotifyClient{
		baseURL:      mockServer.URL + "/v1",
		clientID:     "test-id",
		clientSecret: "test-secret",
		httpClient:   &http.Client{Timeout: 10 * time.Second},
		accessToken:  "test-token",
		tokenExpiry:  time.Now().Add(time.Hour),
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
		if artists[0].ExternalIDs.SpotifyID != "test-id" {
			t.Errorf("expected spotify ID to be test-id, got %s", artists[0].ExternalIDs.SpotifyID)
		}
	})
}

func TestSpotifyClient_GetArtist(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/token":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(spotifyTokenResponse{
				AccessToken: "test-token",
				TokenType:   "Bearer",
				ExpiresIn:   3600,
			})
		case "/v1/artists/test-id":
			w.Header().Set("Content-Type", "application/json")
			artist := spotifyArtist{
				ID:         "test-id",
				Name:       "Test Artist",
				Genres:     []string{"rock", "alternative"},
				Popularity: 80,
				Images: []struct {
					URL string `json:"url"`
				}{
					{URL: "https://example.com/artist.jpg"},
				},
			}
			json.NewEncoder(w).Encode(artist)
		case "/v1/artists/not-found":
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer mockServer.Close()

	client := &SpotifyClient{
		baseURL:      mockServer.URL + "/v1",
		clientID:     "test-id",
		clientSecret: "test-secret",
		httpClient:   &http.Client{Timeout: 10 * time.Second},
		accessToken:  "test-token",
		tokenExpiry:  time.Now().Add(time.Hour),
	}

	t.Run("successful get", func(t *testing.T) {
		ctx := context.Background()
		artist, err := client.GetArtist(ctx, "test-id")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if artist.Name != "Test Artist" {
			t.Errorf("expected name to be Test Artist, got %s", artist.Name)
		}
		if artist.ExternalIDs.SpotifyID != "test-id" {
			t.Errorf("expected spotify ID to be test-id, got %s", artist.ExternalIDs.SpotifyID)
		}
		if len(artist.Genres) != 2 {
			t.Errorf("expected 2 genres, got %d", len(artist.Genres))
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
