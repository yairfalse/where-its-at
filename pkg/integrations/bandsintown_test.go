package integrations

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/yair/where-its-at/pkg/domain"
)

func TestNewBandsintownClient(t *testing.T) {
	t.Run("successful creation", func(t *testing.T) {
		config := BandsintownConfig{
			AppID: "test-app-id",
		}

		client, err := NewBandsintownClient(config)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if client == nil {
			t.Fatal("expected client, got nil")
		}
		if client.appID != "test-app-id" {
			t.Errorf("expected appID to be test-app-id, got %s", client.appID)
		}
		if client.rateLimiter == nil {
			t.Error("expected rateLimiter to be initialized")
		}
	})

	t.Run("missing app ID", func(t *testing.T) {
		config := BandsintownConfig{}

		_, err := NewBandsintownClient(config)
		if err == nil {
			t.Error("expected error for missing app ID")
		}
	})
}

func TestBandsintownClient_SearchEvents(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		artistName := r.URL.Path[len("/artists/"):]
		artistName = artistName[:len(artistName)-len("/events")]

		if artistName == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if artistName == "unknown" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		events := []bandsintownEvent{
			{
				ID:          "bt-123",
				ArtistID:    "artist-123",
				URL:         "https://example.com/event",
				DateTime:    time.Now().Add(7 * 24 * time.Hour).Format(time.RFC3339),
				Description: "Test Concert",
				Venue: bandsintownVenue{
					ID:        "venue-123",
					Name:      "Test Venue",
					Latitude:  "52.5200",
					Longitude: "13.4050",
					City:      "Berlin",
					Region:    "Berlin",
					Country:   "Germany",
				},
				Offers: []bandsintownOffer{
					{
						Type:   "Tickets",
						URL:    "https://example.com/tickets",
						Status: "available",
					},
				},
				Lineup: []string{"Test Artist"},
			},
		}
		json.NewEncoder(w).Encode(events)
	}))
	defer mockServer.Close()

	client := &BandsintownClient{
		baseURL:     mockServer.URL,
		appID:       "test-app",
		httpClient:  &http.Client{Timeout: 10 * time.Second},
		rateLimiter: newRateLimiter(1000),
	}

	t.Run("successful search", func(t *testing.T) {
		ctx := context.Background()
		events, err := client.SearchEvents(ctx, "radiohead", "Berlin")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(events) != 1 {
			t.Fatalf("expected 1 event, got %d", len(events))
		}
		if events[0].ArtistName != "radiohead" {
			t.Errorf("expected artist name to be radiohead, got %s", events[0].ArtistName)
		}
		if events[0].Venue.City != "Berlin" {
			t.Errorf("expected venue city to be Berlin, got %s", events[0].Venue.City)
		}
		if events[0].TicketURL != "https://example.com/tickets" {
			t.Errorf("expected ticket URL, got %s", events[0].TicketURL)
		}
	})

	t.Run("empty artist name", func(t *testing.T) {
		ctx := context.Background()
		_, err := client.SearchEvents(ctx, "", "Berlin")
		if err != domain.ErrInvalidRequest {
			t.Errorf("expected ErrInvalidRequest, got %v", err)
		}
	})

	t.Run("artist not found", func(t *testing.T) {
		ctx := context.Background()
		events, err := client.SearchEvents(ctx, "unknown", "Berlin")
		if err != nil {
			t.Errorf("expected no error for 404, got %v", err)
		}
		if len(events) != 0 {
			t.Errorf("expected 0 events for not found, got %d", len(events))
		}
	})
}

func TestBandsintownClient_GetArtistEvents(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		events := []bandsintownEvent{
			{
				ID:       "bt-456",
				ArtistID: "artist-456",
				DateTime: time.Now().Add(14 * 24 * time.Hour).Format(time.RFC3339),
				Venue: bandsintownVenue{
					ID:      "venue-456",
					Name:    "Another Venue",
					City:    "London",
					Country: "UK",
				},
			},
		}
		json.NewEncoder(w).Encode(events)
	}))
	defer mockServer.Close()

	client := &BandsintownClient{
		baseURL:     mockServer.URL,
		appID:       "test-app",
		httpClient:  &http.Client{Timeout: 10 * time.Second},
		rateLimiter: newRateLimiter(1000),
	}

	t.Run("successful get", func(t *testing.T) {
		ctx := context.Background()
		events, err := client.GetArtistEvents(ctx, "artist-456")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(events) != 1 {
			t.Fatalf("expected 1 event, got %d", len(events))
		}
		if events[0].Venue.City != "London" {
			t.Errorf("expected venue city to be London, got %s", events[0].Venue.City)
		}
	})
}

func TestRateLimiter(t *testing.T) {
	t.Run("allows requests within limit", func(t *testing.T) {
		limiter := newRateLimiter(5)

		for i := 0; i < 5; i++ {
			err := limiter.Allow()
			if err != nil {
				t.Errorf("expected request %d to be allowed, got %v", i+1, err)
			}
		}
	})

	t.Run("blocks requests over limit", func(t *testing.T) {
		limiter := newRateLimiter(2)

		limiter.Allow()
		limiter.Allow()

		err := limiter.Allow()
		if err != domain.ErrRateLimitExceeded {
			t.Errorf("expected ErrRateLimitExceeded, got %v", err)
		}
	})

	t.Run("resets after 24 hours", func(t *testing.T) {
		limiter := newRateLimiter(1)

		limiter.Allow()

		// Manually set window start to 25 hours ago
		limiter.windowStart = time.Now().Add(-25 * time.Hour)

		err := limiter.Allow()
		if err != nil {
			t.Errorf("expected request to be allowed after reset, got %v", err)
		}
	})
}

func TestConvertToDomainEvent(t *testing.T) {
	client := &BandsintownClient{}

	t.Run("successful conversion", func(t *testing.T) {
		btEvent := bandsintownEvent{
			ID:          "bt-789",
			ArtistID:    "artist-789",
			DateTime:    time.Now().Add(7 * 24 * time.Hour).Format(time.RFC3339),
			OnSaleDate:  time.Now().Add(24 * time.Hour).Format(time.RFC3339),
			Description: "Big Concert",
			Venue: bandsintownVenue{
				ID:        "venue-789",
				Name:      "Big Arena",
				Latitude:  "40.7128",
				Longitude: "-74.0060",
				City:      "New York",
				Region:    "NY",
				Country:   "USA",
			},
			Offers: []bandsintownOffer{
				{
					Type:   "Tickets",
					URL:    "https://tickets.com",
					Status: "on sale",
				},
			},
		}

		event, err := client.convertToDomainEvent(btEvent, "Test Artist")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if event.ID != "bandsintown_bt-789" {
			t.Errorf("expected ID to be bandsintown_bt-789, got %s", event.ID)
		}
		if event.ArtistName != "Test Artist" {
			t.Errorf("expected ArtistName to be Test Artist, got %s", event.ArtistName)
		}
		if event.Venue.Latitude != 40.7128 {
			t.Errorf("expected Venue.Latitude to be 40.7128, got %f", event.Venue.Latitude)
		}
		if event.Venue.Longitude != -74.0060 {
			t.Errorf("expected Venue.Longitude to be -74.0060, got %f", event.Venue.Longitude)
		}
		if event.TicketURL != "https://tickets.com" {
			t.Errorf("expected TicketURL to be https://tickets.com, got %s", event.TicketURL)
		}
		if event.TicketStatus != "on sale" {
			t.Errorf("expected TicketStatus to be on sale, got %s", event.TicketStatus)
		}
		if event.OnSaleDate == nil {
			t.Error("expected OnSaleDate to be set")
		}
	})

	t.Run("invalid datetime", func(t *testing.T) {
		btEvent := bandsintownEvent{
			ID:       "bt-bad",
			DateTime: "invalid-date",
		}

		_, err := client.convertToDomainEvent(btEvent, "Test Artist")
		if err == nil {
			t.Error("expected error for invalid datetime")
		}
	})

	t.Run("event without offers", func(t *testing.T) {
		btEvent := bandsintownEvent{
			ID:       "bt-no-offers",
			DateTime: time.Now().Format(time.RFC3339),
			Venue: bandsintownVenue{
				Name:    "Venue",
				City:    "City",
				Country: "Country",
			},
		}

		event, err := client.convertToDomainEvent(btEvent, "Artist")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if event.TicketURL != "" {
			t.Errorf("expected empty TicketURL, got %s", event.TicketURL)
		}
		if event.TicketStatus != "" {
			t.Errorf("expected empty TicketStatus, got %s", event.TicketStatus)
		}
	})
}
