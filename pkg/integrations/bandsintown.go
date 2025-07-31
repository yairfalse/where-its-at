package integrations

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/yair/where-its-at/pkg/domain"
)

type BandsintownClient struct {
	baseURL      string
	appID        string
	httpClient   *http.Client
	rateLimiter  *rateLimiter
}

type BandsintownConfig struct {
	AppID string
}

type rateLimiter struct {
	mu          sync.Mutex
	requests    int
	windowStart time.Time
	dailyLimit  int
}

func newRateLimiter(dailyLimit int) *rateLimiter {
	return &rateLimiter{
		dailyLimit:  dailyLimit,
		windowStart: time.Now(),
	}
}

func (r *rateLimiter) Allow() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	if now.Sub(r.windowStart) > 24*time.Hour {
		r.requests = 0
		r.windowStart = now
	}

	if r.requests >= r.dailyLimit {
		return domain.ErrRateLimitExceeded
	}

	r.requests++
	return nil
}

func NewBandsintownClient(config BandsintownConfig) (*BandsintownClient, error) {
	if config.AppID == "" {
		return nil, fmt.Errorf("bandsintown app ID is required")
	}

	return &BandsintownClient{
		baseURL: "https://rest.bandsintown.com",
		appID:   config.AppID,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		rateLimiter: newRateLimiter(1000),
	}, nil
}

type bandsintownEvent struct {
	ID           string `json:"id"`
	ArtistID     string `json:"artist_id"`
	URL          string `json:"url"`
	OnSaleDate   string `json:"on_sale_datetime"`
	DateTime     string `json:"datetime"`
	Description  string `json:"description"`
	Venue        bandsintownVenue `json:"venue"`
	Offers       []bandsintownOffer `json:"offers"`
	Lineup       []string `json:"lineup"`
}

type bandsintownVenue struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Latitude  string  `json:"latitude"`
	Longitude string  `json:"longitude"`
	City      string  `json:"city"`
	Region    string  `json:"region"`
	Country   string  `json:"country"`
}

type bandsintownOffer struct {
	Type   string `json:"type"`
	URL    string `json:"url"`
	Status string `json:"status"`
}

func (c *BandsintownClient) SearchEvents(ctx context.Context, artistName string, location string) ([]domain.Event, error) {
	if err := c.rateLimiter.Allow(); err != nil {
		return nil, err
	}

	artistName = strings.TrimSpace(artistName)
	if artistName == "" {
		return nil, domain.ErrInvalidRequest
	}

	eventsURL := fmt.Sprintf("%s/artists/%s/events", c.baseURL, url.QueryEscape(artistName))
	req, err := http.NewRequestWithContext(ctx, "GET", eventsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	q := req.URL.Query()
	q.Set("app_id", c.appID)
	if location != "" {
		q.Set("location", location)
	}
	req.URL.RawQuery = q.Encode()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to search events: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return []domain.Event{}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bandsintown search failed: status %d", resp.StatusCode)
	}

	var bandsintownEvents []bandsintownEvent
	if err := json.NewDecoder(resp.Body).Decode(&bandsintownEvents); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	events := make([]domain.Event, 0, len(bandsintownEvents))
	for _, btEvent := range bandsintownEvents {
		event, err := c.convertToDomainEvent(btEvent, artistName)
		if err != nil {
			continue
		}
		events = append(events, event)
	}

	return events, nil
}

func (c *BandsintownClient) GetArtistEvents(ctx context.Context, artistID string) ([]domain.Event, error) {
	if err := c.rateLimiter.Allow(); err != nil {
		return nil, err
	}

	eventsURL := fmt.Sprintf("%s/artists/%s/events", c.baseURL, url.QueryEscape(artistID))
	req, err := http.NewRequestWithContext(ctx, "GET", eventsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	q := req.URL.Query()
	q.Set("app_id", c.appID)
	req.URL.RawQuery = q.Encode()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get artist events: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return []domain.Event{}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bandsintown get events failed: status %d", resp.StatusCode)
	}

	var bandsintownEvents []bandsintownEvent
	if err := json.NewDecoder(resp.Body).Decode(&bandsintownEvents); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	events := make([]domain.Event, 0, len(bandsintownEvents))
	for _, btEvent := range bandsintownEvents {
		event, err := c.convertToDomainEvent(btEvent, artistID)
		if err != nil {
			continue
		}
		events = append(events, event)
	}

	return events, nil
}

func (c *BandsintownClient) convertToDomainEvent(btEvent bandsintownEvent, artistName string) (domain.Event, error) {
	eventTime, err := time.Parse(time.RFC3339, btEvent.DateTime)
	if err != nil {
		return domain.Event{}, fmt.Errorf("failed to parse event time: %w", err)
	}

	lat := 0.0
	lng := 0.0
	if btEvent.Venue.Latitude != "" {
		fmt.Sscanf(btEvent.Venue.Latitude, "%f", &lat)
	}
	if btEvent.Venue.Longitude != "" {
		fmt.Sscanf(btEvent.Venue.Longitude, "%f", &lng)
	}

	event := domain.Event{
		ID:         fmt.Sprintf("bandsintown_%s", btEvent.ID),
		ArtistID:   btEvent.ArtistID,
		ArtistName: artistName,
		Title:      btEvent.Description,
		DateTime:   eventTime,
		Venue: domain.Venue{
			ID:        btEvent.Venue.ID,
			Name:      btEvent.Venue.Name,
			City:      btEvent.Venue.City,
			Region:    btEvent.Venue.Region,
			Country:   btEvent.Venue.Country,
			Latitude:  lat,
			Longitude: lng,
		},
		ExternalIDs: domain.EventExternalIDs{
			BandsintownID: btEvent.ID,
		},
		CachedUntil: time.Now().Add(24 * time.Hour),
	}

	if btEvent.OnSaleDate != "" {
		onSaleTime, err := time.Parse(time.RFC3339, btEvent.OnSaleDate)
		if err == nil {
			event.OnSaleDate = &onSaleTime
		}
	}

	for _, offer := range btEvent.Offers {
		if offer.Type == "Tickets" {
			event.TicketURL = offer.URL
			event.TicketStatus = offer.Status
			break
		}
	}

	return event, nil
}