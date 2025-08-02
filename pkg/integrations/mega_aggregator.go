package integrations

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yair/where-its-at/pkg/domain"
	"github.com/yair/where-its-at/pkg/integrations/sources/scrapers"
)

type MegaAggregator struct {
	musicSources    map[string]MusicSource
	eventSources    map[string]EventSource
	scraperRegistry *scrapers.ScraperRegistry
	deduplicator    *Deduplicator
	cache           *AggregatorCache
	config          MegaAggregatorConfig
}

type MegaAggregatorConfig struct {
	MaxConcurrentRequests int
	RequestTimeout        time.Duration
	CacheEnabled          bool
	CacheTTL              time.Duration
	DeduplicationEnabled  bool
	IncludeScrapers       bool
	MaxResultsPerSource   int
}

type MusicSource interface {
	SearchArtists(ctx context.Context, query string, limit int) ([]domain.Artist, error)
	GetName() string
}

type EventSource interface {
	SearchEventsByArtist(ctx context.Context, artistName string, limit int) ([]domain.Event, error)
	SearchEventsByLocation(ctx context.Context, city, country string, limit int) ([]domain.Event, error)
	GetName() string
}

type Scraper interface {
	ScrapeEvents(ctx context.Context, query string, limit int) ([]scrapers.ScrapedEvent, error)
	ScrapeEventsByLocation(ctx context.Context, city, country string, limit int) ([]scrapers.ScrapedEvent, error)
	GetName() string
}

type AggregatedResults struct {
	Artists      []domain.Artist `json:"artists"`
	Events       []domain.Event  `json:"events"`
	SourceStats  map[string]int  `json:"source_stats"`
	TotalResults int             `json:"total_results"`
	SearchTime   time.Duration   `json:"search_time"`
	Errors       []string        `json:"errors,omitempty"`
}

func NewMegaAggregator(config MegaAggregatorConfig) *MegaAggregator {
	if config.MaxConcurrentRequests == 0 {
		config.MaxConcurrentRequests = 10
	}
	if config.RequestTimeout == 0 {
		config.RequestTimeout = 30 * time.Second
	}
	if config.CacheTTL == 0 {
		config.CacheTTL = 1 * time.Hour
	}
	if config.MaxResultsPerSource == 0 {
		config.MaxResultsPerSource = 20
	}

	aggregator := &MegaAggregator{
		musicSources:    make(map[string]MusicSource),
		eventSources:    make(map[string]EventSource),
		scraperRegistry: scrapers.NewScraperRegistry(),
		deduplicator:    NewDeduplicator(),
		config:          config,
	}

	if config.CacheEnabled {
		aggregator.cache = NewAggregatorCache(config.CacheTTL)
	}

	return aggregator
}

func (m *MegaAggregator) RegisterMusicSource(name string, source MusicSource) {
	m.musicSources[name] = source
}

func (m *MegaAggregator) RegisterEventSource(name string, source EventSource) {
	m.eventSources[name] = source
}

func (m *MegaAggregator) RegisterScraper(scraper scrapers.Scraper) {
	m.scraperRegistry.Register(scraper)
}

func (m *MegaAggregator) SearchArtists(ctx context.Context, query string, limit int) (*AggregatedResults, error) {
	startTime := time.Now()

	if limit <= 0 {
		limit = 50
	}

	// Check cache first
	if m.cache != nil {
		if cached := m.cache.GetArtists(query, limit); cached != nil {
			return cached, nil
		}
	}

	// Parallel search across all music sources
	resultsChan := make(chan SourceResult, len(m.musicSources))
	ctx, cancel := context.WithTimeout(ctx, m.config.RequestTimeout)
	defer cancel()

	// Launch parallel searches
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, m.config.MaxConcurrentRequests)

	for name, source := range m.musicSources {
		wg.Add(1)
		go func(sourceName string, src MusicSource) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			artists, err := src.SearchArtists(ctx, query, m.config.MaxResultsPerSource)
			resultsChan <- SourceResult{
				SourceName: sourceName,
				Artists:    artists,
				Error:      err,
			}
		}(name, source)
	}

	// Close channel when all goroutines complete
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect results
	allArtists := []domain.Artist{}
	sourceStats := make(map[string]int)
	errors := []string{}

	for result := range resultsChan {
		if result.Error != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", result.SourceName, result.Error))
			continue
		}

		sourceStats[result.SourceName] = len(result.Artists)
		allArtists = append(allArtists, result.Artists...)
	}

	// Deduplication
	if m.config.DeduplicationEnabled {
		allArtists = m.deduplicator.DeduplicateArtists(allArtists)
	}

	// Sort by relevance/popularity
	sort.Slice(allArtists, func(i, j int) bool {
		return allArtists[i].Popularity > allArtists[j].Popularity
	})

	// Limit results
	if len(allArtists) > limit {
		allArtists = allArtists[:limit]
	}

	results := &AggregatedResults{
		Artists:      allArtists,
		Events:       []domain.Event{},
		SourceStats:  sourceStats,
		TotalResults: len(allArtists),
		SearchTime:   time.Since(startTime),
		Errors:       errors,
	}

	// Cache results
	if m.cache != nil {
		m.cache.SetArtists(query, limit, results)
	}

	return results, nil
}

func (m *MegaAggregator) SearchEvents(ctx context.Context, artistName string, limit int) (*AggregatedResults, error) {
	startTime := time.Now()

	if limit <= 0 {
		limit = 50
	}

	// Check cache first
	if m.cache != nil {
		if cached := m.cache.GetEvents(artistName, "", limit); cached != nil {
			return cached, nil
		}
	}

	// Parallel search across all event sources
	resultsChan := make(chan SourceResult, len(m.eventSources))
	ctx, cancel := context.WithTimeout(ctx, m.config.RequestTimeout)
	defer cancel()

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, m.config.MaxConcurrentRequests)

	for name, source := range m.eventSources {
		wg.Add(1)
		go func(sourceName string, src EventSource) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			events, err := src.SearchEventsByArtist(ctx, artistName, m.config.MaxResultsPerSource)
			resultsChan <- SourceResult{
				SourceName: sourceName,
				Events:     events,
				Error:      err,
			}
		}(name, source)
	}

	// Include scrapers if enabled
	if m.config.IncludeScrapers {
		scrapers := m.scraperRegistry.GetAllScrapers()
		for _, scraper := range scrapers {
			wg.Add(1)
			go func(scrpr Scraper) {
				defer wg.Done()
				semaphore <- struct{}{}
				defer func() { <-semaphore }()

				scrapedEvents, err := scrpr.ScrapeEvents(ctx, artistName, m.config.MaxResultsPerSource)
				if err != nil {
					resultsChan <- SourceResult{
						SourceName: scrpr.GetName(),
						Error:      err,
					}
					return
				}

				// Convert scraped events to domain events
				events := make([]domain.Event, 0, len(scrapedEvents))
				for _, se := range scrapedEvents {
					events = append(events, se.ToEvent())
				}

				resultsChan <- SourceResult{
					SourceName: scrpr.GetName(),
					Events:     events,
				}
			}(scraper)
		}
	}

	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect results
	allEvents := []domain.Event{}
	sourceStats := make(map[string]int)
	errors := []string{}

	for result := range resultsChan {
		if result.Error != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", result.SourceName, result.Error))
			continue
		}

		sourceStats[result.SourceName] = len(result.Events)
		allEvents = append(allEvents, result.Events...)
	}

	// Deduplication
	if m.config.DeduplicationEnabled {
		allEvents = m.deduplicator.DeduplicateEvents(allEvents)
	}

	// Sort by date (upcoming events first)
	now := time.Now()
	sort.Slice(allEvents, func(i, j int) bool {
		// Upcoming events first, then by date
		iUpcoming := allEvents[i].DateTime.After(now)
		jUpcoming := allEvents[j].DateTime.After(now)

		if iUpcoming && !jUpcoming {
			return true
		}
		if !iUpcoming && jUpcoming {
			return false
		}

		return allEvents[i].DateTime.Before(allEvents[j].DateTime)
	})

	// Limit results
	if len(allEvents) > limit {
		allEvents = allEvents[:limit]
	}

	results := &AggregatedResults{
		Artists:      []domain.Artist{},
		Events:       allEvents,
		SourceStats:  sourceStats,
		TotalResults: len(allEvents),
		SearchTime:   time.Since(startTime),
		Errors:       errors,
	}

	// Cache results
	if m.cache != nil {
		m.cache.SetEvents(artistName, "", limit, results)
	}

	return results, nil
}

func (m *MegaAggregator) SearchEventsByLocation(ctx context.Context, city, country string, limit int) (*AggregatedResults, error) {
	startTime := time.Now()

	if limit <= 0 {
		limit = 50
	}

	// Check cache first
	if m.cache != nil {
		if cached := m.cache.GetEvents("", city, limit); cached != nil {
			return cached, nil
		}
	}

	// Parallel search across all event sources and scrapers
	totalSources := len(m.eventSources)
	if m.config.IncludeScrapers {
		totalSources += len(m.scraperRegistry.GetAllScrapers())
	}

	resultsChan := make(chan SourceResult, totalSources)
	ctx, cancel := context.WithTimeout(ctx, m.config.RequestTimeout)
	defer cancel()

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, m.config.MaxConcurrentRequests)

	// Search event sources
	for name, source := range m.eventSources {
		wg.Add(1)
		go func(sourceName string, src EventSource) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			events, err := src.SearchEventsByLocation(ctx, city, country, m.config.MaxResultsPerSource)
			resultsChan <- SourceResult{
				SourceName: sourceName,
				Events:     events,
				Error:      err,
			}
		}(name, source)
	}

	// Search scrapers
	if m.config.IncludeScrapers {
		scrapers := m.scraperRegistry.GetAllScrapers()
		for _, scraper := range scrapers {
			wg.Add(1)
			go func(scrpr Scraper) {
				defer wg.Done()
				semaphore <- struct{}{}
				defer func() { <-semaphore }()

				scrapedEvents, err := scrpr.ScrapeEventsByLocation(ctx, city, country, m.config.MaxResultsPerSource)
				if err != nil {
					resultsChan <- SourceResult{
						SourceName: scrpr.GetName(),
						Error:      err,
					}
					return
				}

				events := make([]domain.Event, 0, len(scrapedEvents))
				for _, se := range scrapedEvents {
					events = append(events, se.ToEvent())
				}

				resultsChan <- SourceResult{
					SourceName: scrpr.GetName(),
					Events:     events,
				}
			}(scraper)
		}
	}

	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect and process results (same as SearchEvents)
	allEvents := []domain.Event{}
	sourceStats := make(map[string]int)
	errors := []string{}

	for result := range resultsChan {
		if result.Error != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", result.SourceName, result.Error))
			continue
		}

		sourceStats[result.SourceName] = len(result.Events)
		allEvents = append(allEvents, result.Events...)
	}

	if m.config.DeduplicationEnabled {
		allEvents = m.deduplicator.DeduplicateEvents(allEvents)
	}

	now := time.Now()
	sort.Slice(allEvents, func(i, j int) bool {
		iUpcoming := allEvents[i].DateTime.After(now)
		jUpcoming := allEvents[j].DateTime.After(now)

		if iUpcoming && !jUpcoming {
			return true
		}
		if !iUpcoming && jUpcoming {
			return false
		}

		return allEvents[i].DateTime.Before(allEvents[j].DateTime)
	})

	if len(allEvents) > limit {
		allEvents = allEvents[:limit]
	}

	results := &AggregatedResults{
		Artists:      []domain.Artist{},
		Events:       allEvents,
		SourceStats:  sourceStats,
		TotalResults: len(allEvents),
		SearchTime:   time.Since(startTime),
		Errors:       errors,
	}

	if m.cache != nil {
		m.cache.SetEvents("", city, limit, results)
	}

	return results, nil
}

func (m *MegaAggregator) GetSourceStats() map[string]SourceInfo {
	stats := make(map[string]SourceInfo)

	for name := range m.musicSources {
		stats[name] = SourceInfo{
			Type:   "music",
			Status: "active",
		}
	}

	for name := range m.eventSources {
		stats[name] = SourceInfo{
			Type:   "events",
			Status: "active",
		}
	}

	if m.config.IncludeScrapers {
		for _, scraper := range m.scraperRegistry.GetAllScrapers() {
			stats[scraper.GetName()] = SourceInfo{
				Type:   "scraper",
				Status: "active",
			}
		}
	}

	return stats
}

type SourceResult struct {
	SourceName string
	Artists    []domain.Artist
	Events     []domain.Event
	Error      error
}

type SourceInfo struct {
	Type   string `json:"type"`
	Status string `json:"status"`
}

// Deduplicator handles removing duplicate results
type Deduplicator struct{}

func NewDeduplicator() *Deduplicator {
	return &Deduplicator{}
}

func (d *Deduplicator) DeduplicateArtists(artists []domain.Artist) []domain.Artist {
	seen := make(map[string]bool)
	unique := []domain.Artist{}

	for _, artist := range artists {
		key := d.normalizeArtistName(artist.Name)
		if !seen[key] {
			seen[key] = true
			unique = append(unique, artist)
		}
	}

	return unique
}

func (d *Deduplicator) DeduplicateEvents(events []domain.Event) []domain.Event {
	seen := make(map[string]bool)
	unique := []domain.Event{}

	for _, event := range events {
		key := d.normalizeEventKey(event)
		if !seen[key] {
			seen[key] = true
			unique = append(unique, event)
		}
	}

	return unique
}

func (d *Deduplicator) normalizeArtistName(name string) string {
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "")
	name = strings.ReplaceAll(name, ".", "")
	name = strings.ReplaceAll(name, "-", "")
	return name
}

func (d *Deduplicator) normalizeEventKey(event domain.Event) string {
	artistKey := d.normalizeArtistName(event.ArtistName)
	venueKey := strings.ToLower(strings.ReplaceAll(event.Venue.Name, " ", ""))
	dateKey := event.DateTime.Format("20060102")
	return fmt.Sprintf("%s_%s_%s", artistKey, venueKey, dateKey)
}

// AggregatorCache provides caching for aggregated results
type AggregatorCache struct {
	artistCache map[string]CacheEntry
	eventCache  map[string]CacheEntry
	mutex       sync.RWMutex
	ttl         time.Duration
}

type CacheEntry struct {
	Results   *AggregatedResults
	ExpiresAt time.Time
}

func NewAggregatorCache(ttl time.Duration) *AggregatorCache {
	return &AggregatorCache{
		artistCache: make(map[string]CacheEntry),
		eventCache:  make(map[string]CacheEntry),
		ttl:         ttl,
	}
}

func (c *AggregatorCache) GetArtists(query string, limit int) *AggregatedResults {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	key := fmt.Sprintf("%s_%d", query, limit)
	entry, exists := c.artistCache[key]
	if !exists || time.Now().After(entry.ExpiresAt) {
		return nil
	}

	return entry.Results
}

func (c *AggregatorCache) SetArtists(query string, limit int, results *AggregatedResults) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	key := fmt.Sprintf("%s_%d", query, limit)
	c.artistCache[key] = CacheEntry{
		Results:   results,
		ExpiresAt: time.Now().Add(c.ttl),
	}
}

func (c *AggregatorCache) GetEvents(artistName, city string, limit int) *AggregatedResults {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	key := fmt.Sprintf("%s_%s_%d", artistName, city, limit)
	entry, exists := c.eventCache[key]
	if !exists || time.Now().After(entry.ExpiresAt) {
		return nil
	}

	return entry.Results
}

func (c *AggregatorCache) SetEvents(artistName, city string, limit int, results *AggregatedResults) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	key := fmt.Sprintf("%s_%s_%d", artistName, city, limit)
	c.eventCache[key] = CacheEntry{
		Results:   results,
		ExpiresAt: time.Now().Add(c.ttl),
	}
}
