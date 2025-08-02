package domain

import (
	"testing"
	"time"
)

func TestEvent(t *testing.T) {
	t.Run("Event struct creation", func(t *testing.T) {
		now := time.Now()
		onSaleDate := now.Add(24 * time.Hour)

		event := Event{
			ID:         "test-123",
			ArtistID:   "artist-456",
			ArtistName: "Test Artist",
			Title:      "Test Concert",
			DateTime:   now.Add(7 * 24 * time.Hour),
			Venue: Venue{
				ID:        "venue-789",
				Name:      "Test Venue",
				City:      "Berlin",
				Region:    "Berlin",
				Country:   "Germany",
				Latitude:  52.5200,
				Longitude: 13.4050,
			},
			TicketURL:    "https://example.com/tickets",
			TicketStatus: "available",
			OnSaleDate:   &onSaleDate,
			ExternalIDs: EventExternalIDs{
				BandsintownID:  "bt-123",
				TicketmasterID: "tm-456",
			},
			CreatedAt:   now,
			UpdatedAt:   now,
			CachedUntil: now.Add(24 * time.Hour),
		}

		if event.ID != "test-123" {
			t.Errorf("expected ID to be test-123, got %s", event.ID)
		}
		if event.ArtistName != "Test Artist" {
			t.Errorf("expected ArtistName to be Test Artist, got %s", event.ArtistName)
		}
		if event.Venue.City != "Berlin" {
			t.Errorf("expected Venue.City to be Berlin, got %s", event.Venue.City)
		}
		if event.Venue.Latitude != 52.5200 {
			t.Errorf("expected Venue.Latitude to be 52.5200, got %f", event.Venue.Latitude)
		}
		if event.ExternalIDs.BandsintownID != "bt-123" {
			t.Errorf("expected BandsintownID to be bt-123, got %s", event.ExternalIDs.BandsintownID)
		}
		if event.OnSaleDate == nil || !event.OnSaleDate.Equal(onSaleDate) {
			t.Errorf("expected OnSaleDate to be %v, got %v", onSaleDate, event.OnSaleDate)
		}
	})

	t.Run("Venue struct validation", func(t *testing.T) {
		venue := Venue{
			ID:        "v-123",
			Name:      "Madison Square Garden",
			City:      "New York",
			Region:    "NY",
			Country:   "USA",
			Latitude:  40.7505,
			Longitude: -73.9934,
		}

		if venue.Name != "Madison Square Garden" {
			t.Errorf("expected Name to be Madison Square Garden, got %s", venue.Name)
		}
		if venue.Latitude != 40.7505 {
			t.Errorf("expected Latitude to be 40.7505, got %f", venue.Latitude)
		}
		if venue.Longitude != -73.9934 {
			t.Errorf("expected Longitude to be -73.9934, got %f", venue.Longitude)
		}
	})

	t.Run("EventSearchRequest validation", func(t *testing.T) {
		startDate := time.Now()
		endDate := startDate.Add(30 * 24 * time.Hour)

		req := EventSearchRequest{
			ArtistName: "Radiohead",
			ArtistID:   "artist-123",
			Location:   "Berlin",
			Radius:     50,
			StartDate:  &startDate,
			EndDate:    &endDate,
		}

		if req.ArtistName != "Radiohead" {
			t.Errorf("expected ArtistName to be Radiohead, got %s", req.ArtistName)
		}
		if req.Radius != 50 {
			t.Errorf("expected Radius to be 50, got %d", req.Radius)
		}
		if req.StartDate == nil || !req.StartDate.Equal(startDate) {
			t.Errorf("expected StartDate to be %v, got %v", startDate, req.StartDate)
		}
	})

	t.Run("EventSearchResponse creation", func(t *testing.T) {
		events := []Event{
			{ID: "1", ArtistName: "Artist 1"},
			{ID: "2", ArtistName: "Artist 2"},
		}

		response := EventSearchResponse{
			Events: events,
			Total:  2,
		}

		if len(response.Events) != 2 {
			t.Errorf("expected 2 events, got %d", len(response.Events))
		}
		if response.Total != 2 {
			t.Errorf("expected Total to be 2, got %d", response.Total)
		}
	})

	t.Run("Event without optional fields", func(t *testing.T) {
		event := Event{
			ID:         "minimal-123",
			ArtistID:   "artist-min",
			ArtistName: "Minimal Artist",
			DateTime:   time.Now(),
			Venue: Venue{
				Name:    "Minimal Venue",
				City:    "City",
				Country: "Country",
			},
			CachedUntil: time.Now().Add(time.Hour),
		}

		if event.Title != "" {
			t.Errorf("expected Title to be empty, got %s", event.Title)
		}
		if event.TicketURL != "" {
			t.Errorf("expected TicketURL to be empty, got %s", event.TicketURL)
		}
		if event.OnSaleDate != nil {
			t.Errorf("expected OnSaleDate to be nil, got %v", event.OnSaleDate)
		}
		if event.Venue.Region != "" {
			t.Errorf("expected Venue.Region to be empty, got %s", event.Venue.Region)
		}
	})
}
