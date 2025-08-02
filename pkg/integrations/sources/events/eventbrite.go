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

type EventbriteClient struct {
	baseURL     string
	token       string
	httpClient  *http.Client
	rateLimiter *eventRateLimiter
}

type EventbriteConfig struct {
	Token string // Eventbrite OAuth token
}

func NewEventbriteClient(config EventbriteConfig) (*EventbriteClient, error) {
	if config.Token == "" {
		return nil, fmt.Errorf("eventbrite token is required")
	}

	return &EventbriteClient{
		baseURL:     "https://www.eventbriteapi.com/v3",
		token:       config.Token,
		httpClient:  &http.Client{Timeout: 10 * time.Second},
		rateLimiter: newEventRateLimiter(1000), // 1000 requests per hour for personal tokens
	}, nil
}

type eventbriteEvent struct {
	Name                         eventbriteMultiPartText    `json:"name"`
	Description                  eventbriteMultiPartText    `json:"description"`
	ID                           string                     `json:"id"`
	URL                          string                     `json:"url"`
	VanityURL                    string                     `json:"vanity_url"`
	Start                        eventbriteDateTime         `json:"start"`
	End                          eventbriteDateTime         `json:"end"`
	OrganizationID               string                     `json:"organization_id"`
	Created                      string                     `json:"created"`
	Changed                      string                     `json:"changed"`
	Published                    string                     `json:"published"`
	Capacity                     int                        `json:"capacity"`
	CapacityIsCustom             bool                       `json:"capacity_is_custom"`
	Status                       string                     `json:"status"`
	Currency                     string                     `json:"currency"`
	Listed                       bool                       `json:"listed"`
	Shareable                    bool                       `json:"shareable"`
	OnlineEvent                  bool                       `json:"online_event"`
	TxTimeLimit                  int                        `json:"tx_time_limit"`
	HideStartDate                bool                       `json:"hide_start_date"`
	HideEndDate                  bool                       `json:"hide_end_date"`
	Locale                       string                     `json:"locale"`
	IsLocked                     bool                       `json:"is_locked"`
	PrivacySetting               string                     `json:"privacy_setting"`
	IsSeries                     bool                       `json:"is_series"`
	IsSeriesParent               bool                       `json:"is_series_parent"`
	InventoryType                string                     `json:"inventory_type"`
	IsReservedSeating            bool                       `json:"is_reserved_seating"`
	ShowPickASeat                bool                       `json:"show_pick_a_seat"`
	ShowSeatmapThumbnail         bool                       `json:"show_seatmap_thumbnail"`
	ShowColorsInSeatmapThumbnail bool                       `json:"show_colors_in_seatmap_thumbnail"`
	Source                       string                     `json:"source"`
	IsFree                       bool                       `json:"is_free"`
	Version                      string                     `json:"version"`
	SummaryPriceDisplay          string                     `json:"summary_price_display"`
	LogoID                       string                     `json:"logo_id"`
	OrganizationLogoID           string                     `json:"organization_logo_id"`
	VenueID                      string                     `json:"venue_id"`
	CategoryID                   string                     `json:"category_id"`
	SubcategoryID                string                     `json:"subcategory_id"`
	FormatID                     string                     `json:"format_id"`
	ResourceURI                  string                     `json:"resource_uri"`
	IsSalesOpen                  bool                       `json:"is_sales_open"`
	CheckoutSettings             eventbriteCheckoutSettings `json:"checkout_settings"`
}

type eventbriteMultiPartText struct {
	Text string `json:"text"`
	HTML string `json:"html"`
}

type eventbriteDateTime struct {
	Timezone string `json:"timezone"`
	Local    string `json:"local"`
	UTC      string `json:"utc"`
}

type eventbriteCheckoutSettings struct {
	CountryCode  string `json:"country_code"`
	CurrencyCode string `json:"currency_code"`
}

type eventbriteVenue struct {
	Address        eventbriteAddress `json:"address"`
	ID             string            `json:"id"`
	AgeRestriction string            `json:"age_restriction"`
	Capacity       int               `json:"capacity"`
	Name           string            `json:"name"`
	Latitude       string            `json:"latitude"`
	Longitude      string            `json:"longitude"`
	ResourceURI    string            `json:"resource_uri"`
}

type eventbriteAddress struct {
	Address1                         string   `json:"address_1"`
	Address2                         string   `json:"address_2"`
	City                             string   `json:"city"`
	Region                           string   `json:"region"`
	PostalCode                       string   `json:"postal_code"`
	Country                          string   `json:"country"`
	Latitude                         string   `json:"latitude"`
	Longitude                        string   `json:"longitude"`
	LocalizedAreaDisplay             string   `json:"localized_area_display"`
	LocalizedAddressDisplay          string   `json:"localized_address_display"`
	LocalizedMultiLineAddressDisplay []string `json:"localized_multi_line_address_display"`
}

type eventbriteOrganization struct {
	Name            string                  `json:"name"`
	Description     eventbriteMultiPartText `json:"description"`
	LongDescription eventbriteMultiPartText `json:"long_description"`
	LogoID          string                  `json:"logo_id"`
	ResourceURI     string                  `json:"resource_uri"`
	ID              string                  `json:"id"`
	URL             string                  `json:"url"`
	VanityURL       string                  `json:"vanity_url"`
	NumPastEvents   int                     `json:"num_past_events"`
	NumFutureEvents int                     `json:"num_future_events"`
	Twitter         string                  `json:"twitter"`
	Facebook        string                  `json:"facebook"`
}

type eventbriteCategory struct {
	ResourceURI        string                  `json:"resource_uri"`
	ID                 string                  `json:"id"`
	Name               string                  `json:"name"`
	NameLocalized      string                  `json:"name_localized"`
	ShortName          string                  `json:"short_name"`
	ShortNameLocalized string                  `json:"short_name_localized"`
	Subcategories      []eventbriteSubcategory `json:"subcategories"`
}

type eventbriteSubcategory struct {
	ResourceURI    string             `json:"resource_uri"`
	ID             string             `json:"id"`
	Name           string             `json:"name"`
	ParentCategory eventbriteCategory `json:"parent_category"`
}

type eventbriteEventsResponse struct {
	Pagination eventbritePagination `json:"pagination"`
	Events     []eventbriteEvent    `json:"events"`
}

type eventbritePagination struct {
	ObjectCount  int    `json:"object_count"`
	PageNumber   int    `json:"page_number"`
	PageSize     int    `json:"page_size"`
	PageCount    int    `json:"page_count"`
	Continuation string `json:"continuation"`
	HasMoreItems bool   `json:"has_more_items"`
}

func (c *EventbriteClient) SearchEventsByQuery(ctx context.Context, query string, limit int) ([]domain.Event, error) {
	if err := c.rateLimiter.Allow(); err != nil {
		return nil, err
	}

	query = strings.TrimSpace(query)
	if query == "" {
		return nil, domain.ErrInvalidRequest
	}

	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	searchURL := fmt.Sprintf("%s/events/search/", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	q := req.URL.Query()
	q.Set("q", query)
	q.Set("categories", "103") // Music category ID
	q.Set("expand", "venue,organizer,category,subcategory")
	q.Set("sort_by", "date")
	q.Set("page_size", fmt.Sprintf("%d", limit))
	req.URL.RawQuery = q.Encode()

	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to search events: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, domain.ErrRateLimitExceeded
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("eventbrite unauthorized: invalid token")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("eventbrite search failed: status %d", resp.StatusCode)
	}

	var eventsResp eventbriteEventsResponse
	if err := json.NewDecoder(resp.Body).Decode(&eventsResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	events := make([]domain.Event, 0, len(eventsResp.Events))
	for _, ebEvent := range eventsResp.Events {
		event, err := c.convertToEvent(ctx, ebEvent)
		if err != nil {
			continue // Skip events that can't be converted
		}
		events = append(events, event)
	}

	return events, nil
}

func (c *EventbriteClient) SearchEventsByLocation(ctx context.Context, location string, limit int) ([]domain.Event, error) {
	if err := c.rateLimiter.Allow(); err != nil {
		return nil, err
	}

	location = strings.TrimSpace(location)
	if location == "" {
		return nil, domain.ErrInvalidRequest
	}

	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	searchURL := fmt.Sprintf("%s/events/search/", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	q := req.URL.Query()
	q.Set("location.address", location)
	q.Set("categories", "103") // Music category
	q.Set("expand", "venue,organizer,category,subcategory")
	q.Set("sort_by", "date")
	q.Set("page_size", fmt.Sprintf("%d", limit))
	req.URL.RawQuery = q.Encode()

	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to search events: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("eventbrite location search failed: status %d", resp.StatusCode)
	}

	var eventsResp eventbriteEventsResponse
	if err := json.NewDecoder(resp.Body).Decode(&eventsResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	events := make([]domain.Event, 0, len(eventsResp.Events))
	for _, ebEvent := range eventsResp.Events {
		event, err := c.convertToEvent(ctx, ebEvent)
		if err != nil {
			continue
		}
		events = append(events, event)
	}

	return events, nil
}

func (c *EventbriteClient) GetEvent(ctx context.Context, eventbriteID string) (*domain.Event, error) {
	if err := c.rateLimiter.Allow(); err != nil {
		return nil, err
	}

	eventURL := fmt.Sprintf("%s/events/%s/", c.baseURL, eventbriteID)
	req, err := http.NewRequestWithContext(ctx, "GET", eventURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	q := req.URL.Query()
	q.Set("expand", "venue,organizer,category,subcategory")
	req.URL.RawQuery = q.Encode()

	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get event: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, domain.ErrEventNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("eventbrite get event failed: status %d", resp.StatusCode)
	}

	var ebEvent eventbriteEvent
	if err := json.NewDecoder(resp.Body).Decode(&ebEvent); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	event, err := c.convertToEvent(ctx, ebEvent)
	if err != nil {
		return nil, err
	}

	return &event, nil
}

func (c *EventbriteClient) convertToEvent(ctx context.Context, ebEvent eventbriteEvent) (domain.Event, error) {
	// Parse event datetime
	eventTime := c.parseEventDateTime(ebEvent.Start)

	// Use event name as artist name (Eventbrite doesn't separate these well)
	artistName := ebEvent.Name.Text
	if artistName == "" {
		artistName = "Unknown Artist"
	}

	// Get venue information
	venue := domain.Venue{
		Name:    "Unknown Venue",
		City:    "Unknown City",
		Country: "Unknown Country",
	}

	if ebEvent.VenueID != "" {
		ebVenue, err := c.getVenue(ctx, ebEvent.VenueID)
		if err == nil {
			venue.Name = ebVenue.Name
			venue.City = ebVenue.Address.City
			venue.Country = ebVenue.Address.Country

			// Parse coordinates
			if ebVenue.Latitude != "" && ebVenue.Longitude != "" {
				fmt.Sscanf(ebVenue.Latitude, "%f", &venue.Latitude)
				fmt.Sscanf(ebVenue.Longitude, "%f", &venue.Longitude)
			}
		}
	}

	// Set 24-hour cache
	cacheUntil := time.Now().Add(24 * time.Hour)

	return domain.Event{
		ID:          fmt.Sprintf("eventbrite_%s", ebEvent.ID),
		ArtistID:    fmt.Sprintf("eventbrite_artist_%s", strings.ReplaceAll(strings.ToLower(artistName), " ", "_")),
		ArtistName:  artistName,
		DateTime:    eventTime,
		Venue:       venue,
		CachedUntil: cacheUntil,
	}, nil
}

func (c *EventbriteClient) getVenue(ctx context.Context, venueID string) (*eventbriteVenue, error) {
	if err := c.rateLimiter.Allow(); err != nil {
		return nil, err
	}

	venueURL := fmt.Sprintf("%s/venues/%s/", c.baseURL, venueID)
	req, err := http.NewRequestWithContext(ctx, "GET", venueURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get venue: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("eventbrite get venue failed: status %d", resp.StatusCode)
	}

	var venue eventbriteVenue
	if err := json.NewDecoder(resp.Body).Decode(&venue); err != nil {
		return nil, fmt.Errorf("failed to decode venue response: %w", err)
	}

	return &venue, nil
}

func (c *EventbriteClient) parseEventDateTime(start eventbriteDateTime) time.Time {
	// Try UTC first
	if start.UTC != "" {
		if t, err := time.Parse("2006-01-02T15:04:05Z", start.UTC); err == nil {
			return t
		}
	}

	// Try local time
	if start.Local != "" {
		if t, err := time.Parse("2006-01-02T15:04:05", start.Local); err == nil {
			return t
		}
	}

	// Ultimate fallback
	return time.Now()
}
