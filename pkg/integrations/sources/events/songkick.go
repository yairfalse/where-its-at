package events

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/yair/where-its-at/pkg/domain"
)

type SongkickClient struct {
	baseURL     string
	apiKey      string
	httpClient  *http.Client
	rateLimiter *eventRateLimiter
}

type SongkickConfig struct {
	APIKey string // Songkick API key
}

func NewSongkickClient(config SongkickConfig) (*SongkickClient, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("songkick API key is required")
	}

	return &SongkickClient{
		baseURL:     "https://api.songkick.com/api/3.0",
		apiKey:      config.APIKey,
		httpClient:  &http.Client{Timeout: 10 * time.Second},
		rateLimiter: newEventRateLimiter(1000), // 1000 requests per day
	}, nil
}

type songkickEvent struct {
	ID             int64                 `json:"id"`
	Type           string                `json:"type"`
	URI            string                `json:"uri"`
	DisplayName    string                `json:"displayName"`
	Start          songkickEventDate     `json:"start"`
	Status         string                `json:"status"`
	Popularity     float64               `json:"popularity"`
	Venue          songkickVenue         `json:"venue"`
	Location       songkickLocation      `json:"location,omitempty"`
	Performance    []songkickPerformance `json:"performance"`
	AgeRestriction string                `json:"ageRestriction,omitempty"`
}

type songkickEventDate struct {
	Date     string `json:"date"`
	Time     string `json:"time,omitempty"`
	DateTime string `json:"datetime,omitempty"`
}

type songkickVenue struct {
	ID          int64            `json:"id"`
	DisplayName string           `json:"displayName"`
	URI         string           `json:"uri"`
	MetroArea   songkickLocation `json:"metroArea"`
	Lat         float64          `json:"lat,omitempty"`
	Lng         float64          `json:"lng,omitempty"`
	Website     string           `json:"website,omitempty"`
	Phone       string           `json:"phone,omitempty"`
	Capacity    int              `json:"capacity,omitempty"`
}

type songkickLocation struct {
	ID          int64           `json:"id"`
	DisplayName string          `json:"displayName"`
	URI         string          `json:"uri"`
	Country     songkickCountry `json:"country,omitempty"`
	State       songkickState   `json:"state,omitempty"`
	Lat         float64         `json:"lat,omitempty"`
	Lng         float64         `json:"lng,omitempty"`
}

type songkickCountry struct {
	DisplayName string `json:"displayName"`
}

type songkickState struct {
	DisplayName string `json:"displayName"`
}

type songkickPerformance struct {
	ID           int64          `json:"id"`
	DisplayName  string         `json:"displayName"`
	Billing      string         `json:"billing"`
	BillingIndex int            `json:"billingIndex"`
	Artist       songkickArtist `json:"artist"`
}

type songkickArtist struct {
	ID          int64                `json:"id"`
	DisplayName string               `json:"displayName"`
	URI         string               `json:"uri"`
	Identifier  []songkickIdentifier `json:"identifier,omitempty"`
}

type songkickIdentifier struct {
	MbidType string `json:"mbid_type,omitempty"`
	Mbid     string `json:"mbid,omitempty"`
	Href     string `json:"href,omitempty"`
}

type songkickEventsResponse struct {
	ResultsPage songkickResultsPage `json:"resultsPage"`
}

type songkickResultsPage struct {
	Status       string          `json:"status"`
	Results      songkickResults `json:"results"`
	PerPage      int             `json:"perPage"`
	Page         int             `json:"page"`
	TotalEntries int             `json:"totalEntries"`
}

type songkickResults struct {
	Event []songkickEvent `json:"event"`
}

type songkickArtistSearchResponse struct {
	ResultsPage struct {
		Status  string `json:"status"`
		Results struct {
			Artist []songkickArtist `json:"artist"`
		} `json:"results"`
	} `json:"resultsPage"`
}

func (c *SongkickClient) SearchEventsByArtist(ctx context.Context, artistName string, limit int) ([]domain.Event, error) {
	if err := c.rateLimiter.Allow(); err != nil {
		return nil, err
	}

	artistName = strings.TrimSpace(artistName)
	if artistName == "" {
		return nil, domain.ErrInvalidRequest
	}

	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	// First, find the artist ID
	artistID, err := c.findArtistID(ctx, artistName)
	if err != nil {
		return nil, err
	}

	if artistID == 0 {
		return []domain.Event{}, nil // Artist not found
	}

	// Get upcoming events for the artist
	eventsURL := fmt.Sprintf("%s/artists/%d/calendar.json", c.baseURL, artistID)
	req, err := http.NewRequestWithContext(ctx, "GET", eventsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	q := req.URL.Query()
	q.Set("apikey", c.apiKey)
	q.Set("per_page", fmt.Sprintf("%d", limit))
	req.URL.RawQuery = q.Encode()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to search events: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, domain.ErrRateLimitExceeded
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("songkick search failed: status %d", resp.StatusCode)
	}

	var eventsResp songkickEventsResponse
	if err := json.NewDecoder(resp.Body).Decode(&eventsResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	events := make([]domain.Event, 0, len(eventsResp.ResultsPage.Results.Event))
	for _, skEvent := range eventsResp.ResultsPage.Results.Event {
		event := c.convertToEvent(skEvent, artistName)
		events = append(events, event)
	}

	return events, nil
}

func (c *SongkickClient) SearchEventsByLocation(ctx context.Context, city, country string, limit int) ([]domain.Event, error) {
	if err := c.rateLimiter.Allow(); err != nil {
		return nil, err
	}

	city = strings.TrimSpace(city)
	country = strings.TrimSpace(country)
	if city == "" {
		return nil, domain.ErrInvalidRequest
	}

	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	// Find location ID first
	locationID, err := c.findLocationID(ctx, city, country)
	if err != nil {
		return nil, err
	}

	if locationID == 0 {
		return []domain.Event{}, nil // Location not found
	}

	// Get events in the location
	eventsURL := fmt.Sprintf("%s/metro_areas/%d/calendar.json", c.baseURL, locationID)
	req, err := http.NewRequestWithContext(ctx, "GET", eventsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	q := req.URL.Query()
	q.Set("apikey", c.apiKey)
	q.Set("per_page", fmt.Sprintf("%d", limit))
	req.URL.RawQuery = q.Encode()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to search events: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("songkick location search failed: status %d", resp.StatusCode)
	}

	var eventsResp songkickEventsResponse
	if err := json.NewDecoder(resp.Body).Decode(&eventsResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	events := make([]domain.Event, 0, len(eventsResp.ResultsPage.Results.Event))
	for _, skEvent := range eventsResp.ResultsPage.Results.Event {
		// For location-based search, use the main performer as artist
		artistName := c.getMainPerformer(skEvent)
		event := c.convertToEvent(skEvent, artistName)
		events = append(events, event)
	}

	return events, nil
}

func (c *SongkickClient) GetEvent(ctx context.Context, songkickID string) (*domain.Event, error) {
	if err := c.rateLimiter.Allow(); err != nil {
		return nil, err
	}

	eventURL := fmt.Sprintf("%s/events/%s.json", c.baseURL, songkickID)
	req, err := http.NewRequestWithContext(ctx, "GET", eventURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	q := req.URL.Query()
	q.Set("apikey", c.apiKey)
	req.URL.RawQuery = q.Encode()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get event: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, domain.ErrEventNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("songkick get event failed: status %d", resp.StatusCode)
	}

	var response struct {
		ResultsPage struct {
			Results struct {
				Event songkickEvent `json:"event"`
			} `json:"results"`
		} `json:"resultsPage"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	artistName := c.getMainPerformer(response.ResultsPage.Results.Event)
	event := c.convertToEvent(response.ResultsPage.Results.Event, artistName)
	return &event, nil
}

func (c *SongkickClient) findArtistID(ctx context.Context, artistName string) (int64, error) {
	if err := c.rateLimiter.Allow(); err != nil {
		return 0, err
	}

	searchURL := fmt.Sprintf("%s/search/artists.json", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	q := req.URL.Query()
	q.Set("apikey", c.apiKey)
	q.Set("query", artistName)
	req.URL.RawQuery = q.Encode()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to search artist: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("songkick artist search failed: status %d", resp.StatusCode)
	}

	var searchResp songkickArtistSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return 0, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(searchResp.ResultsPage.Results.Artist) == 0 {
		return 0, nil
	}

	// Return the first match (best match)
	return searchResp.ResultsPage.Results.Artist[0].ID, nil
}

func (c *SongkickClient) findLocationID(ctx context.Context, city, country string) (int64, error) {
	if err := c.rateLimiter.Allow(); err != nil {
		return 0, err
	}

	query := city
	if country != "" {
		query = fmt.Sprintf("%s, %s", city, country)
	}

	searchURL := fmt.Sprintf("%s/search/locations.json", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	q := req.URL.Query()
	q.Set("apikey", c.apiKey)
	q.Set("query", query)
	req.URL.RawQuery = q.Encode()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to search location: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("songkick location search failed: status %d", resp.StatusCode)
	}

	var searchResp struct {
		ResultsPage struct {
			Results struct {
				Location []songkickLocation `json:"location"`
			} `json:"results"`
		} `json:"resultsPage"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return 0, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(searchResp.ResultsPage.Results.Location) == 0 {
		return 0, nil
	}

	// Return the first match (best match)
	return searchResp.ResultsPage.Results.Location[0].ID, nil
}

func (c *SongkickClient) getMainPerformer(event songkickEvent) string {
	if len(event.Performance) == 0 {
		return "Unknown Artist"
	}

	// Find the headliner (billing = "headline" or billingIndex = 1)
	for _, perf := range event.Performance {
		if perf.Billing == "headline" || perf.BillingIndex == 1 {
			return perf.Artist.DisplayName
		}
	}

	// Fallback to first performer
	return event.Performance[0].Artist.DisplayName
}

func (c *SongkickClient) convertToEvent(skEvent songkickEvent, mainArtist string) domain.Event {
	// Parse event datetime
	eventTime := c.parseEventDateTime(skEvent.Start)

	// Convert venue
	venue := domain.Venue{
		Name:      skEvent.Venue.DisplayName,
		City:      skEvent.Venue.MetroArea.DisplayName,
		Country:   skEvent.Venue.MetroArea.Country.DisplayName,
		Latitude:  skEvent.Venue.Lat,
		Longitude: skEvent.Venue.Lng,
	}

	// Set 24-hour cache
	cacheUntil := time.Now().Add(24 * time.Hour)

	return domain.Event{
		ID:          fmt.Sprintf("songkick_%d", skEvent.ID),
		ArtistID:    fmt.Sprintf("songkick_artist_%s", strings.ReplaceAll(strings.ToLower(mainArtist), " ", "_")),
		ArtistName:  mainArtist,
		DateTime:    eventTime,
		Venue:       venue,
		CachedUntil: cacheUntil,
	}
}

func (c *SongkickClient) parseEventDateTime(start songkickEventDate) time.Time {
	// Try to parse full datetime first
	if start.DateTime != "" {
		if t, err := time.Parse("2006-01-02T15:04:05-0700", start.DateTime); err == nil {
			return t
		}
		if t, err := time.Parse("2006-01-02T15:04:05Z", start.DateTime); err == nil {
			return t
		}
	}

	// Try date + time
	if start.Date != "" && start.Time != "" {
		dateTimeStr := start.Date + "T" + start.Time
		if t, err := time.Parse("2006-01-02T15:04:05", dateTimeStr); err == nil {
			return t
		}
	}

	// Fallback to date only
	if start.Date != "" {
		if t, err := time.Parse("2006-01-02", start.Date); err == nil {
			return t
		}
	}

	// Ultimate fallback
	return time.Now()
}

// Event rate limiter for APIs with daily limits
type eventRateLimiter struct {
	requests   []time.Time
	limit      int
	windowSize time.Duration
}

func newEventRateLimiter(dailyLimit int) *eventRateLimiter {
	return &eventRateLimiter{
		requests:   make([]time.Time, 0),
		limit:      dailyLimit,
		windowSize: 24 * time.Hour,
	}
}

func (r *eventRateLimiter) Allow() error {
	now := time.Now()

	// Remove requests older than window
	cutoff := now.Add(-r.windowSize)
	validRequests := make([]time.Time, 0, len(r.requests))
	for _, reqTime := range r.requests {
		if reqTime.After(cutoff) {
			validRequests = append(validRequests, reqTime)
		}
	}
	r.requests = validRequests

	// Check if we're under the limit
	if len(r.requests) >= r.limit {
		return domain.ErrRateLimitExceeded
	}

	// Add current request
	r.requests = append(r.requests, now)
	return nil
}
