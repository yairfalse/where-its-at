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

type TicketmasterClient struct {
	baseURL     string
	apiKey      string
	httpClient  *http.Client
	rateLimiter *eventRateLimiter
}

type TicketmasterConfig struct {
	APIKey string // Ticketmaster Discovery API key
}

func NewTicketmasterClient(config TicketmasterConfig) (*TicketmasterClient, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("ticketmaster API key is required")
	}

	return &TicketmasterClient{
		baseURL:     "https://app.ticketmaster.com/discovery/v2",
		apiKey:      config.APIKey,
		httpClient:  &http.Client{Timeout: 10 * time.Second},
		rateLimiter: newEventRateLimiter(5000), // 5000 requests per day
	}, nil
}

type ticketmasterEvent struct {
	Name                  string                       `json:"name"`
	Type                  string                       `json:"type"`
	ID                    string                       `json:"id"`
	Test                  bool                         `json:"test"`
	URL                   string                       `json:"url"`
	Locale                string                       `json:"locale"`
	Images                []ticketmasterImage          `json:"images"`
	Sales                 ticketmasterSales            `json:"sales"`
	Dates                 ticketmasterDates            `json:"dates"`
	Classifications       []ticketmasterClassification `json:"classifications"`
	Promoter              ticketmasterPromoter         `json:"promoter,omitempty"`
	Info                  string                       `json:"info,omitempty"`
	PleaseNote            string                       `json:"pleaseNote,omitempty"`
	PriceRanges           []ticketmasterPriceRange     `json:"priceRanges,omitempty"`
	Products              []ticketmasterProduct        `json:"products,omitempty"`
	Seatmap               ticketmasterSeatmap          `json:"seatmap,omitempty"`
	Accessibility         ticketmasterAccessibility    `json:"accessibility,omitempty"`
	TicketLimit           ticketmasterTicketLimit      `json:"ticketLimit,omitempty"`
	AgeRestrictions       ticketmasterAgeRestrictions  `json:"ageRestrictions,omitempty"`
	TicketingInstructions string                       `json:"ticketingInstructions,omitempty"`
	AdditionalInfo        string                       `json:"additionalInfo,omitempty"`
	Embedded              struct {
		Venues      []ticketmasterVenue      `json:"venues"`
		Attractions []ticketmasterAttraction `json:"attractions"`
	} `json:"_embedded"`
}

type ticketmasterImage struct {
	Ratio       string `json:"ratio"`
	URL         string `json:"url"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	Fallback    bool   `json:"fallback"`
	Attribution string `json:"attribution,omitempty"`
}

type ticketmasterSales struct {
	Public   ticketmasterSaleDate   `json:"public"`
	Presales []ticketmasterSaleDate `json:"presales,omitempty"`
}

type ticketmasterSaleDate struct {
	StartDateTime string `json:"startDateTime"`
	StartTBD      bool   `json:"startTBD"`
	StartTBA      bool   `json:"startTBA"`
	EndDateTime   string `json:"endDateTime"`
	EndTBD        bool   `json:"endTBD"`
	EndTBA        bool   `json:"endTBA"`
}

type ticketmasterDates struct {
	Start            ticketmasterEventDate `json:"start"`
	End              ticketmasterEventDate `json:"end,omitempty"`
	Timezone         string                `json:"timezone"`
	Status           ticketmasterStatus    `json:"status"`
	SpanMultipleDays bool                  `json:"spanMultipleDays"`
}

type ticketmasterEventDate struct {
	LocalDate      string `json:"localDate"`
	LocalTime      string `json:"localTime"`
	DateTime       string `json:"dateTime"`
	DateTBD        bool   `json:"dateTBD"`
	DateTBA        bool   `json:"dateTBA"`
	TimeTBA        bool   `json:"timeTBA"`
	NoSpecificTime bool   `json:"noSpecificTime"`
}

type ticketmasterStatus struct {
	Code string `json:"code"`
}

type ticketmasterClassification struct {
	Primary  bool                           `json:"primary"`
	Segment  ticketmasterClassificationItem `json:"segment"`
	Genre    ticketmasterClassificationItem `json:"genre"`
	SubGenre ticketmasterClassificationItem `json:"subGenre"`
	Type     ticketmasterClassificationItem `json:"type"`
	SubType  ticketmasterClassificationItem `json:"subType"`
	Family   bool                           `json:"family"`
}

type ticketmasterClassificationItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ticketmasterPromoter struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type ticketmasterPriceRange struct {
	Type     string  `json:"type"`
	Currency string  `json:"currency"`
	Min      float64 `json:"min"`
	Max      float64 `json:"max"`
}

type ticketmasterProduct struct {
	Name            string                       `json:"name"`
	ID              string                       `json:"id"`
	URL             string                       `json:"url"`
	Type            string                       `json:"type"`
	Classifications []ticketmasterClassification `json:"classifications"`
}

type ticketmasterSeatmap struct {
	StaticURL string `json:"staticUrl"`
}

type ticketmasterAccessibility struct {
	TicketLimit int `json:"ticketLimit"`
}

type ticketmasterTicketLimit struct {
	Info string `json:"info"`
}

type ticketmasterAgeRestrictions struct {
	LegalAgeEnforced bool   `json:"legalAgeEnforced"`
	ID               string `json:"id,omitempty"`
}

type ticketmasterVenue struct {
	Name              string                        `json:"name"`
	Type              string                        `json:"type"`
	ID                string                        `json:"id"`
	Test              bool                          `json:"test"`
	URL               string                        `json:"url"`
	Locale            string                        `json:"locale"`
	Images            []ticketmasterImage           `json:"images"`
	PostalCode        string                        `json:"postalCode"`
	Timezone          string                        `json:"timezone"`
	City              ticketmasterCity              `json:"city"`
	State             ticketmasterState             `json:"state"`
	Country           ticketmasterCountry           `json:"country"`
	Address           ticketmasterAddress           `json:"address"`
	Location          ticketmasterLocation          `json:"location"`
	Markets           []ticketmasterMarket          `json:"markets"`
	Dmas              []ticketmasterDma             `json:"dmas"`
	Social            ticketmasterSocial            `json:"social,omitempty"`
	BoxOffice         ticketmasterBoxOffice         `json:"boxOffice,omitempty"`
	ParkingDetail     string                        `json:"parkingDetail,omitempty"`
	AccessibleSeating ticketmasterAccessibleSeating `json:"accessibleSeating,omitempty"`
	GeneralInfo       ticketmasterGeneralInfo       `json:"generalInfo,omitempty"`
}

type ticketmasterCity struct {
	Name string `json:"name"`
}

type ticketmasterState struct {
	Name      string `json:"name"`
	StateCode string `json:"stateCode"`
}

type ticketmasterCountry struct {
	Name        string `json:"name"`
	CountryCode string `json:"countryCode"`
}

type ticketmasterAddress struct {
	Line1 string `json:"line1"`
	Line2 string `json:"line2,omitempty"`
}

type ticketmasterLocation struct {
	Longitude string `json:"longitude"`
	Latitude  string `json:"latitude"`
}

type ticketmasterMarket struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

type ticketmasterDma struct {
	ID string `json:"id"`
}

type ticketmasterSocial struct {
	Twitter ticketmasterSocialAccount `json:"twitter,omitempty"`
}

type ticketmasterSocialAccount struct {
	Handle string `json:"handle"`
}

type ticketmasterBoxOffice struct {
	PhoneNumberDetail     string `json:"phoneNumberDetail"`
	OpenHoursDetail       string `json:"openHoursDetail"`
	AcceptedPaymentDetail string `json:"acceptedPaymentDetail"`
	WillCallDetail        string `json:"willCallDetail"`
}

type ticketmasterAccessibleSeating struct {
	Info string `json:"info"`
}

type ticketmasterGeneralInfo struct {
	GeneralRule string `json:"generalRule"`
	ChildRule   string `json:"childRule"`
}

type ticketmasterAttraction struct {
	Name            string                       `json:"name"`
	Type            string                       `json:"type"`
	ID              string                       `json:"id"`
	Test            bool                         `json:"test"`
	URL             string                       `json:"url"`
	Locale          string                       `json:"locale"`
	ExternalLinks   ticketmasterExternalLinks    `json:"externalLinks,omitempty"`
	Aliases         []string                     `json:"aliases,omitempty"`
	Images          []ticketmasterImage          `json:"images"`
	Classifications []ticketmasterClassification `json:"classifications"`
}

type ticketmasterExternalLinks struct {
	YouTube     []ticketmasterExternalLink `json:"youtube,omitempty"`
	Twitter     []ticketmasterExternalLink `json:"twitter,omitempty"`
	LastFm      []ticketmasterExternalLink `json:"lastfm,omitempty"`
	Facebook    []ticketmasterExternalLink `json:"facebook,omitempty"`
	Spotify     []ticketmasterExternalLink `json:"spotify,omitempty"`
	MusicBrainz []ticketmasterExternalLink `json:"musicbrainz,omitempty"`
	Instagram   []ticketmasterExternalLink `json:"instagram,omitempty"`
	Homepage    []ticketmasterExternalLink `json:"homepage,omitempty"`
}

type ticketmasterExternalLink struct {
	URL string `json:"url"`
}

type ticketmasterEventsResponse struct {
	Embedded struct {
		Events []ticketmasterEvent `json:"events"`
	} `json:"_embedded"`
	Links ticketmasterLinks `json:"_links"`
	Page  ticketmasterPage  `json:"page"`
}

type ticketmasterLinks struct {
	Self ticketmasterLink `json:"self"`
	Next ticketmasterLink `json:"next,omitempty"`
	Prev ticketmasterLink `json:"prev,omitempty"`
}

type ticketmasterLink struct {
	Href      string `json:"href"`
	Templated bool   `json:"templated,omitempty"`
}

type ticketmasterPage struct {
	Size          int `json:"size"`
	TotalElements int `json:"totalElements"`
	TotalPages    int `json:"totalPages"`
	Number        int `json:"number"`
}

func (c *TicketmasterClient) SearchEventsByKeyword(ctx context.Context, keyword string, limit int) ([]domain.Event, error) {
	if err := c.rateLimiter.Allow(); err != nil {
		return nil, err
	}

	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return nil, domain.ErrInvalidRequest
	}

	if limit <= 0 {
		limit = 10
	}
	if limit > 200 {
		limit = 200
	}

	eventsURL := fmt.Sprintf("%s/events.json", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", eventsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	q := req.URL.Query()
	q.Set("apikey", c.apiKey)
	q.Set("keyword", keyword)
	q.Set("size", fmt.Sprintf("%d", limit))
	q.Set("classificationName", "music") // Focus on music events
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
		return nil, fmt.Errorf("ticketmaster search failed: status %d", resp.StatusCode)
	}

	var eventsResp ticketmasterEventsResponse
	if err := json.NewDecoder(resp.Body).Decode(&eventsResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	events := make([]domain.Event, 0, len(eventsResp.Embedded.Events))
	for _, tmEvent := range eventsResp.Embedded.Events {
		event := c.convertToEvent(tmEvent)
		events = append(events, event)
	}

	return events, nil
}

func (c *TicketmasterClient) SearchEventsByLocation(ctx context.Context, city, country string, limit int) ([]domain.Event, error) {
	if err := c.rateLimiter.Allow(); err != nil {
		return nil, err
	}

	city = strings.TrimSpace(city)
	if city == "" {
		return nil, domain.ErrInvalidRequest
	}

	if limit <= 0 {
		limit = 10
	}
	if limit > 200 {
		limit = 200
	}

	eventsURL := fmt.Sprintf("%s/events.json", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", eventsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	q := req.URL.Query()
	q.Set("apikey", c.apiKey)
	q.Set("city", city)
	if country != "" {
		q.Set("countryCode", strings.ToUpper(country))
	}
	q.Set("size", fmt.Sprintf("%d", limit))
	q.Set("classificationName", "music")
	req.URL.RawQuery = q.Encode()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to search events: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ticketmaster location search failed: status %d", resp.StatusCode)
	}

	var eventsResp ticketmasterEventsResponse
	if err := json.NewDecoder(resp.Body).Decode(&eventsResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	events := make([]domain.Event, 0, len(eventsResp.Embedded.Events))
	for _, tmEvent := range eventsResp.Embedded.Events {
		event := c.convertToEvent(tmEvent)
		events = append(events, event)
	}

	return events, nil
}

func (c *TicketmasterClient) GetEvent(ctx context.Context, ticketmasterID string) (*domain.Event, error) {
	if err := c.rateLimiter.Allow(); err != nil {
		return nil, err
	}

	eventURL := fmt.Sprintf("%s/events/%s.json", c.baseURL, ticketmasterID)
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
		return nil, fmt.Errorf("ticketmaster get event failed: status %d", resp.StatusCode)
	}

	var tmEvent ticketmasterEvent
	if err := json.NewDecoder(resp.Body).Decode(&tmEvent); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	event := c.convertToEvent(tmEvent)
	return &event, nil
}

func (c *TicketmasterClient) convertToEvent(tmEvent ticketmasterEvent) domain.Event {
	// Parse event datetime
	eventTime := c.parseEventDateTime(tmEvent.Dates.Start)

	// Get primary attraction (artist) name
	artistName := tmEvent.Name
	if len(tmEvent.Embedded.Attractions) > 0 {
		artistName = tmEvent.Embedded.Attractions[0].Name
	}

	// Convert venue
	venue := domain.Venue{
		Name:    "Unknown Venue",
		City:    "Unknown City",
		Country: "Unknown Country",
	}

	if len(tmEvent.Embedded.Venues) > 0 {
		tmVenue := tmEvent.Embedded.Venues[0]
		venue.Name = tmVenue.Name
		venue.City = tmVenue.City.Name
		venue.Country = tmVenue.Country.Name

		// Parse coordinates
		if tmVenue.Location.Latitude != "" && tmVenue.Location.Longitude != "" {
			fmt.Sscanf(tmVenue.Location.Latitude, "%f", &venue.Latitude)
			fmt.Sscanf(tmVenue.Location.Longitude, "%f", &venue.Longitude)
		}
	}

	// Set 24-hour cache
	cacheUntil := time.Now().Add(24 * time.Hour)

	return domain.Event{
		ID:          fmt.Sprintf("ticketmaster_%s", tmEvent.ID),
		ArtistID:    fmt.Sprintf("ticketmaster_artist_%s", strings.ReplaceAll(strings.ToLower(artistName), " ", "_")),
		ArtistName:  artistName,
		DateTime:    eventTime,
		Venue:       venue,
		CachedUntil: cacheUntil,
	}
}

func (c *TicketmasterClient) parseEventDateTime(start ticketmasterEventDate) time.Time {
	// Try to parse full datetime first
	if start.DateTime != "" {
		if t, err := time.Parse("2006-01-02T15:04:05Z", start.DateTime); err == nil {
			return t
		}
		if t, err := time.Parse("2006-01-02T15:04:05-0700", start.DateTime); err == nil {
			return t
		}
	}

	// Try date + time combination
	if start.LocalDate != "" && start.LocalTime != "" {
		dateTimeStr := start.LocalDate + "T" + start.LocalTime
		if t, err := time.Parse("2006-01-02T15:04:05", dateTimeStr); err == nil {
			return t
		}
	}

	// Fallback to date only
	if start.LocalDate != "" {
		if t, err := time.Parse("2006-01-02", start.LocalDate); err == nil {
			return t
		}
	}

	// Ultimate fallback
	return time.Now()
}
