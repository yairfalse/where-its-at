package scrapers

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/yair/where-its-at/pkg/domain"
)

type ScrapingConfig struct {
	UserAgent    string
	RequestDelay time.Duration
	MaxRetries   int
	Timeout      time.Duration
}

type BaseScraper struct {
	httpClient  *http.Client
	config      ScrapingConfig
	rateLimiter *scraperRateLimiter
}

func NewBaseScraper(config ScrapingConfig) *BaseScraper {
	if config.UserAgent == "" {
		config.UserAgent = "WhereItsAt-Bot/1.0 (+https://whereiatsat.com/bot)"
	}
	if config.RequestDelay == 0 {
		config.RequestDelay = 2 * time.Second // Conservative 2-second delay
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	return &BaseScraper{
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		config:      config,
		rateLimiter: newScraperRateLimiter(config.RequestDelay),
	}
}

func (b *BaseScraper) MakeRequest(ctx context.Context, url string, headers map[string]string) (*http.Response, error) {
	if err := b.rateLimiter.Wait(ctx); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", b.config.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	req.Header.Set("DNT", "1")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	var resp *http.Response
	var lastErr error

	for attempt := 0; attempt <= b.config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff
			backoff := time.Duration(attempt*attempt) * time.Second
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		resp, lastErr = b.httpClient.Do(req)
		if lastErr == nil {
			if resp.StatusCode == http.StatusTooManyRequests {
				resp.Body.Close()
				// Respect rate limits
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(5 * time.Second):
				}
				continue
			}
			if resp.StatusCode >= 500 {
				resp.Body.Close()
				// Server error, retry
				continue
			}
			// Success or non-retryable error
			break
		}
	}

	if lastErr != nil {
		return nil, fmt.Errorf("request failed after %d attempts: %w", b.config.MaxRetries+1, lastErr)
	}

	return resp, nil
}

func (b *BaseScraper) NormalizeURL(baseURL, relativeURL string) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid base URL: %w", err)
	}

	rel, err := url.Parse(relativeURL)
	if err != nil {
		return "", fmt.Errorf("invalid relative URL: %w", err)
	}

	return base.ResolveReference(rel).String(), nil
}

func (b *BaseScraper) ExtractText(text string) string {
	text = strings.TrimSpace(text)
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\t", " ")

	// Remove multiple spaces
	for strings.Contains(text, "  ") {
		text = strings.ReplaceAll(text, "  ", " ")
	}

	return text
}

func (b *BaseScraper) ParseDate(dateStr string) time.Time {
	formats := []string{
		"2006-01-02",
		"02/01/2006",
		"01/02/2006",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05-07:00",
		"January 2, 2006",
		"Jan 2, 2006",
		"2 January 2006",
		"2 Jan 2006",
		"Monday, January 2, 2006",
		"Mon, Jan 2, 2006",
	}

	dateStr = strings.TrimSpace(dateStr)

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t
		}
	}

	// Fallback to current time
	return time.Now()
}

type scraperRateLimiter struct {
	lastRequest time.Time
	delay       time.Duration
}

func newScraperRateLimiter(delay time.Duration) *scraperRateLimiter {
	return &scraperRateLimiter{
		delay: delay,
	}
}

func (r *scraperRateLimiter) Wait(ctx context.Context) error {
	now := time.Now()
	elapsed := now.Sub(r.lastRequest)

	if elapsed < r.delay {
		waitTime := r.delay - elapsed
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitTime):
		}
	}

	r.lastRequest = time.Now()
	return nil
}

type ScrapedEvent struct {
	Title       string
	ArtistName  string
	Date        time.Time
	VenueName   string
	City        string
	Country     string
	URL         string
	Description string
	Price       string
	Tags        []string
}

func (s *ScrapedEvent) ToEvent() domain.Event {
	venue := domain.Venue{
		Name:    s.VenueName,
		City:    s.City,
		Country: s.Country,
	}

	// Generate IDs based on scraped data
	artistID := fmt.Sprintf("scraped_artist_%s", strings.ReplaceAll(strings.ToLower(s.ArtistName), " ", "_"))
	eventID := fmt.Sprintf("scraped_%s_%s", strings.ReplaceAll(strings.ToLower(s.ArtistName), " ", "_"), s.Date.Format("20060102"))

	// Set 24-hour cache
	cacheUntil := time.Now().Add(24 * time.Hour)

	return domain.Event{
		ID:          eventID,
		ArtistID:    artistID,
		ArtistName:  s.ArtistName,
		DateTime:    s.Date,
		Venue:       venue,
		CachedUntil: cacheUntil,
	}
}

type Scraper interface {
	ScrapeEvents(ctx context.Context, query string, limit int) ([]ScrapedEvent, error)
	ScrapeEventsByLocation(ctx context.Context, city, country string, limit int) ([]ScrapedEvent, error)
	GetName() string
}

type ScraperRegistry struct {
	scrapers map[string]Scraper
}

func NewScraperRegistry() *ScraperRegistry {
	return &ScraperRegistry{
		scrapers: make(map[string]Scraper),
	}
}

func (r *ScraperRegistry) Register(scraper Scraper) {
	r.scrapers[scraper.GetName()] = scraper
}

func (r *ScraperRegistry) GetScraper(name string) (Scraper, bool) {
	scraper, exists := r.scrapers[name]
	return scraper, exists
}

func (r *ScraperRegistry) GetAllScrapers() []Scraper {
	scrapers := make([]Scraper, 0, len(r.scrapers))
	for _, scraper := range r.scrapers {
		scrapers = append(scrapers, scraper)
	}
	return scrapers
}

func (r *ScraperRegistry) ScrapeAll(ctx context.Context, query string, limit int) ([]ScrapedEvent, error) {
	allEvents := []ScrapedEvent{}

	for _, scraper := range r.scrapers {
		events, err := scraper.ScrapeEvents(ctx, query, limit)
		if err != nil {
			// Log error but continue with other scrapers
			continue
		}
		allEvents = append(allEvents, events...)
	}

	return allEvents, nil
}
