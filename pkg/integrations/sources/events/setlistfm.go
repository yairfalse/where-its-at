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

type SetlistFMClient struct {
	baseURL     string
	apiKey      string
	httpClient  *http.Client
	rateLimiter *eventRateLimiter
}

type SetlistFMConfig struct {
	APIKey string // Setlist.fm API key
}

func NewSetlistFMClient(config SetlistFMConfig) (*SetlistFMClient, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("setlist.fm API key is required")
	}

	return &SetlistFMClient{
		baseURL:     "https://api.setlist.fm/rest/1.0",
		apiKey:      config.APIKey,
		httpClient:  &http.Client{Timeout: 10 * time.Second},
		rateLimiter: newEventRateLimiter(2000), // 2000 requests per day
	}, nil
}

type setlistFMSetlist struct {
	ID          string          `json:"id"`
	VersionID   string          `json:"versionId"`
	EventDate   string          `json:"eventDate"`
	LastUpdated string          `json:"lastUpdated"`
	Artist      setlistFMArtist `json:"artist"`
	Venue       setlistFMVenue  `json:"venue"`
	Tour        setlistFMTour   `json:"tour,omitempty"`
	Sets        setlistFMSets   `json:"sets"`
	Info        string          `json:"info,omitempty"`
	URL         string          `json:"url"`
}

type setlistFMArtist struct {
	MBID           string `json:"mbid,omitempty"`
	TMID           int64  `json:"tmid,omitempty"`
	Name           string `json:"name"`
	SortName       string `json:"sortName"`
	Disambiguation string `json:"disambiguation,omitempty"`
	URL            string `json:"url"`
}

type setlistFMVenue struct {
	ID   string        `json:"id"`
	Name string        `json:"name"`
	City setlistFMCity `json:"city"`
	URL  string        `json:"url"`
}

type setlistFMCity struct {
	ID        string           `json:"id"`
	Name      string           `json:"name"`
	State     string           `json:"state,omitempty"`
	StateCode string           `json:"stateCode,omitempty"`
	Country   setlistFMCountry `json:"country"`
	Coords    setlistFMCoords  `json:"coords"`
}

type setlistFMCountry struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type setlistFMCoords struct {
	Lat  float64 `json:"lat"`
	Long float64 `json:"long"`
}

type setlistFMTour struct {
	Name string `json:"name"`
}

type setlistFMSets struct {
	Set []setlistFMSet `json:"set"`
}

type setlistFMSet struct {
	Name   string          `json:"name,omitempty"`
	Encore int             `json:"encore,omitempty"`
	Song   []setlistFMSong `json:"song"`
}

type setlistFMSong struct {
	Name  string          `json:"name"`
	With  setlistFMArtist `json:"with,omitempty"`
	Cover setlistFMArtist `json:"cover,omitempty"`
	Info  string          `json:"info,omitempty"`
	Tape  bool            `json:"tape,omitempty"`
}

type setlistFMSearchResponse struct {
	Type         string             `json:"type"`
	ItemsPerPage int                `json:"itemsPerPage"`
	Page         int                `json:"page"`
	Total        int                `json:"total"`
	Setlist      []setlistFMSetlist `json:"setlist"`
}

type setlistFMArtistSearchResponse struct {
	Type         string            `json:"type"`
	ItemsPerPage int               `json:"itemsPerPage"`
	Page         int               `json:"page"`
	Total        int               `json:"total"`
	Artist       []setlistFMArtist `json:"artist"`
}

type setlistFMVenueSearchResponse struct {
	Type         string           `json:"type"`
	ItemsPerPage int              `json:"itemsPerPage"`
	Page         int              `json:"page"`
	Total        int              `json:"total"`
	Venue        []setlistFMVenue `json:"venue"`
}

func (c *SetlistFMClient) SearchSetlistsByArtist(ctx context.Context, artistName string, limit int) ([]domain.Event, error) {
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
	if limit > 20 {
		limit = 20 // Setlist.fm has smaller page sizes
	}

	// First, find the artist MBID if possible
	artistMBID, err := c.findArtistMBID(ctx, artistName)
	if err != nil {
		return nil, err
	}

	var searchURL string
	if artistMBID != "" {
		searchURL = fmt.Sprintf("%s/search/setlists", c.baseURL)
	} else {
		// Fallback to artist name search
		searchURL = fmt.Sprintf("%s/search/setlists", c.baseURL)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	q := req.URL.Query()
	if artistMBID != "" {
		q.Set("artistMbid", artistMBID)
	} else {
		q.Set("artistName", artistName)
	}
	q.Set("p", "1") // Page 1
	req.URL.RawQuery = q.Encode()

	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "WhereItsAt/1.0") // Setlist.fm requires user agent

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to search setlists: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, domain.ErrRateLimitExceeded
	}
	if resp.StatusCode == http.StatusNotFound {
		return []domain.Event{}, nil // No setlists found
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("setlist.fm search failed: status %d", resp.StatusCode)
	}

	var searchResp setlistFMSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	events := make([]domain.Event, 0, len(searchResp.Setlist))
	for i, setlist := range searchResp.Setlist {
		if i >= limit {
			break
		}
		event := c.convertToEvent(setlist)
		events = append(events, event)
	}

	return events, nil
}

func (c *SetlistFMClient) SearchSetlistsByVenue(ctx context.Context, venueName, city string, limit int) ([]domain.Event, error) {
	if err := c.rateLimiter.Allow(); err != nil {
		return nil, err
	}

	venueName = strings.TrimSpace(venueName)
	city = strings.TrimSpace(city)
	if venueName == "" && city == "" {
		return nil, domain.ErrInvalidRequest
	}

	if limit <= 0 {
		limit = 10
	}
	if limit > 20 {
		limit = 20
	}

	// Search for venue first to get venue ID
	venueID, err := c.findVenueID(ctx, venueName, city)
	if err != nil {
		return nil, err
	}

	if venueID == "" {
		return []domain.Event{}, nil // Venue not found
	}

	searchURL := fmt.Sprintf("%s/search/setlists", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	q := req.URL.Query()
	q.Set("venueId", venueID)
	q.Set("p", "1")
	req.URL.RawQuery = q.Encode()

	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "WhereItsAt/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to search setlists: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return []domain.Event{}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("setlist.fm venue search failed: status %d", resp.StatusCode)
	}

	var searchResp setlistFMSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	events := make([]domain.Event, 0, len(searchResp.Setlist))
	for i, setlist := range searchResp.Setlist {
		if i >= limit {
			break
		}
		event := c.convertToEvent(setlist)
		events = append(events, event)
	}

	return events, nil
}

func (c *SetlistFMClient) GetSetlist(ctx context.Context, setlistID string) (*domain.Event, error) {
	if err := c.rateLimiter.Allow(); err != nil {
		return nil, err
	}

	setlistURL := fmt.Sprintf("%s/setlist/%s", c.baseURL, setlistID)
	req, err := http.NewRequestWithContext(ctx, "GET", setlistURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "WhereItsAt/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get setlist: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, domain.ErrEventNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("setlist.fm get setlist failed: status %d", resp.StatusCode)
	}

	var setlist setlistFMSetlist
	if err := json.NewDecoder(resp.Body).Decode(&setlist); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	event := c.convertToEvent(setlist)
	return &event, nil
}

func (c *SetlistFMClient) findArtistMBID(ctx context.Context, artistName string) (string, error) {
	if err := c.rateLimiter.Allow(); err != nil {
		return "", err
	}

	searchURL := fmt.Sprintf("%s/search/artists", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	q := req.URL.Query()
	q.Set("artistName", artistName)
	q.Set("p", "1")
	req.URL.RawQuery = q.Encode()

	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "WhereItsAt/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to search artist: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", nil // Artist not found
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("setlist.fm artist search failed: status %d", resp.StatusCode)
	}

	var searchResp setlistFMArtistSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(searchResp.Artist) == 0 {
		return "", nil
	}

	// Return MBID of the first match (best match)
	return searchResp.Artist[0].MBID, nil
}

func (c *SetlistFMClient) findVenueID(ctx context.Context, venueName, city string) (string, error) {
	if err := c.rateLimiter.Allow(); err != nil {
		return "", err
	}

	searchURL := fmt.Sprintf("%s/search/venues", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	q := req.URL.Query()
	if venueName != "" {
		q.Set("name", venueName)
	}
	if city != "" {
		q.Set("cityName", city)
	}
	q.Set("p", "1")
	req.URL.RawQuery = q.Encode()

	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "WhereItsAt/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to search venue: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", nil
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("setlist.fm venue search failed: status %d", resp.StatusCode)
	}

	var searchResp setlistFMVenueSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(searchResp.Venue) == 0 {
		return "", nil
	}

	// Return ID of the first match
	return searchResp.Venue[0].ID, nil
}

func (c *SetlistFMClient) convertToEvent(setlist setlistFMSetlist) domain.Event {
	// Parse event date
	eventTime := c.parseEventDate(setlist.EventDate)

	// Convert venue
	venue := domain.Venue{
		Name:      setlist.Venue.Name,
		City:      setlist.Venue.City.Name,
		Country:   setlist.Venue.City.Country.Name,
		Latitude:  setlist.Venue.City.Coords.Lat,
		Longitude: setlist.Venue.City.Coords.Long,
	}

	// Set 24-hour cache
	cacheUntil := time.Now().Add(24 * time.Hour)

	return domain.Event{
		ID:          fmt.Sprintf("setlistfm_%s", setlist.ID),
		ArtistID:    fmt.Sprintf("setlistfm_artist_%s", strings.ReplaceAll(strings.ToLower(setlist.Artist.Name), " ", "_")),
		ArtistName:  setlist.Artist.Name,
		DateTime:    eventTime,
		Venue:       venue,
		CachedUntil: cacheUntil,
	}
}

func (c *SetlistFMClient) parseEventDate(eventDate string) time.Time {
	// Setlist.fm uses DD-MM-YYYY format
	if t, err := time.Parse("02-01-2006", eventDate); err == nil {
		return t
	}

	// Fallback to other common formats
	if t, err := time.Parse("2006-01-02", eventDate); err == nil {
		return t
	}

	// Ultimate fallback
	return time.Now()
}

func (c *SetlistFMClient) GetSetlistSongs(ctx context.Context, setlistID string) ([]SetlistFMSong, error) {
	if err := c.rateLimiter.Allow(); err != nil {
		return nil, err
	}

	setlistURL := fmt.Sprintf("%s/setlist/%s", c.baseURL, setlistID)
	req, err := http.NewRequestWithContext(ctx, "GET", setlistURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "WhereItsAt/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get setlist: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("setlist.fm get setlist failed: status %d", resp.StatusCode)
	}

	var setlist setlistFMSetlist
	if err := json.NewDecoder(resp.Body).Decode(&setlist); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	songs := []SetlistFMSong{}
	for _, set := range setlist.Sets.Set {
		for _, song := range set.Song {
			songData := SetlistFMSong{
				Name:     song.Name,
				SetName:  set.Name,
				IsEncore: set.Encore > 0,
				Info:     song.Info,
				IsTape:   song.Tape,
			}

			if song.With.Name != "" {
				songData.WithArtist = song.With.Name
			}
			if song.Cover.Name != "" {
				songData.CoverOf = song.Cover.Name
			}

			songs = append(songs, songData)
		}
	}

	return songs, nil
}

type SetlistFMSong struct {
	Name       string `json:"name"`
	SetName    string `json:"set_name"`
	IsEncore   bool   `json:"is_encore"`
	Info       string `json:"info,omitempty"`
	IsTape     bool   `json:"is_tape"`
	WithArtist string `json:"with_artist,omitempty"`
	CoverOf    string `json:"cover_of,omitempty"`
}
