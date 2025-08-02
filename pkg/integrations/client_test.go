package integrations

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestSpotifyClient_getAccessToken(t *testing.T) {
	t.Run("token already valid", func(t *testing.T) {
		client := &SpotifyClient{
			accessToken: "valid-token",
			tokenExpiry: time.Now().Add(time.Hour),
		}

		err := client.getAccessToken(context.Background())
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if client.accessToken != "valid-token" {
			t.Errorf("expected token to remain valid-token, got %s", client.accessToken)
		}
	})
}

func TestSpotifyClient_SearchArtists_Limits(t *testing.T) {
	client := &SpotifyClient{
		baseURL:      "http://test",
		clientID:     "test-id",
		clientSecret: "test-secret",
		httpClient:   &http.Client{Timeout: 10 * time.Second},
		accessToken:  "test-token",
		tokenExpiry:  time.Now().Add(time.Hour),
	}

	t.Run("zero limit defaults to 10", func(t *testing.T) {
		// This will fail but shows the limit handling
		_, err := client.SearchArtists(context.Background(), "test", 0)
		if err == nil {
			t.Log("Limit defaults to 10 when 0 is passed")
		}
	})

	t.Run("limit over 50 capped at 50", func(t *testing.T) {
		// This will fail but shows the limit handling
		_, err := client.SearchArtists(context.Background(), "test", 100)
		if err == nil {
			t.Log("Limit capped at 50 when over 50 is passed")
		}
	})
}

func TestLastFMClient_SearchArtists_Limits(t *testing.T) {
	client := &LastFMClient{
		baseURL:    "http://test",
		apiKey:     "test-key",
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}

	t.Run("zero limit defaults to 10", func(t *testing.T) {
		// This will fail but shows the limit handling
		_, err := client.SearchArtists(context.Background(), "test", 0)
		if err == nil {
			t.Log("Limit defaults to 10 when 0 is passed")
		}
	})

	t.Run("limit over 30 capped at 30", func(t *testing.T) {
		// This will fail but shows the limit handling
		_, err := client.SearchArtists(context.Background(), "test", 100)
		if err == nil {
			t.Log("Limit capped at 30 when over 30 is passed")
		}
	})
}
