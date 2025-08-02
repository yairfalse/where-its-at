package scrapers

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/html"
)

type BandcampScraper struct {
	*BaseScraper
	baseURL string
}

func NewBandcampScraper(config ScrapingConfig) *BandcampScraper {
	return &BandcampScraper{
		BaseScraper: NewBaseScraper(config),
		baseURL:     "https://bandcamp.com",
	}
}

func (b *BandcampScraper) GetName() string {
	return "bandcamp"
}

func (b *BandcampScraper) ScrapeEvents(ctx context.Context, query string, limit int) ([]ScrapedEvent, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}

	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	// Search for artists first, then get their events
	artists, err := b.searchArtists(ctx, query, 5)
	if err != nil {
		return nil, fmt.Errorf("failed to search artists: %w", err)
	}

	allEvents := []ScrapedEvent{}
	eventsPerArtist := limit / len(artists)
	if eventsPerArtist == 0 {
		eventsPerArtist = 1
	}

	for _, artistURL := range artists {
		events, err := b.scrapeArtistEvents(ctx, artistURL, eventsPerArtist)
		if err != nil {
			continue // Skip failed artists
		}
		allEvents = append(allEvents, events...)
		if len(allEvents) >= limit {
			break
		}
	}

	if len(allEvents) > limit {
		allEvents = allEvents[:limit]
	}

	return allEvents, nil
}

func (b *BandcampScraper) ScrapeEventsByLocation(ctx context.Context, city, country string, limit int) ([]ScrapedEvent, error) {
	// Bandcamp doesn't have strong location-based discovery
	// Fall back to searching by city name
	return b.ScrapeEvents(ctx, city, limit)
}

func (b *BandcampScraper) searchArtists(ctx context.Context, query string, limit int) ([]string, error) {
	searchURL := fmt.Sprintf("%s/search?q=%s&item_type=b", b.baseURL, url.QueryEscape(query))

	resp, err := b.MakeRequest(ctx, searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch search results: %w", err)
	}
	defer resp.Body.Close()

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	artistURLs := []string{}

	// Find artist links in search results
	var findArtistLinks func(*html.Node)
	findArtistLinks = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			href := b.getAttribute(n, "href")
			if href != "" && b.isArtistURL(href) {
				if fullURL, err := b.NormalizeURL(b.baseURL, href); err == nil {
					artistURLs = append(artistURLs, fullURL)
					if len(artistURLs) >= limit {
						return
					}
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findArtistLinks(c)
			if len(artistURLs) >= limit {
				return
			}
		}
	}

	findArtistLinks(doc)
	return artistURLs, nil
}

func (b *BandcampScraper) scrapeArtistEvents(ctx context.Context, artistURL string, limit int) ([]ScrapedEvent, error) {
	// First get artist info
	artistInfo, err := b.getArtistInfo(ctx, artistURL)
	if err != nil {
		return nil, err
	}

	// Check if artist has upcoming shows
	showsURL := artistURL + "/shows"
	resp, err := b.MakeRequest(ctx, showsURL, nil)
	if err != nil {
		// No shows page, create pseudo-events from releases
		return b.createEventsFromReleases(ctx, artistURL, artistInfo, limit)
	}
	defer resp.Body.Close()

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse shows page: %w", err)
	}

	events := b.parseShowsPage(doc, artistInfo)

	if len(events) == 0 {
		// Fallback to release-based events
		return b.createEventsFromReleases(ctx, artistURL, artistInfo, limit)
	}

	if len(events) > limit {
		events = events[:limit]
	}

	return events, nil
}

func (b *BandcampScraper) getArtistInfo(ctx context.Context, artistURL string) (BandcampArtist, error) {
	resp, err := b.MakeRequest(ctx, artistURL, nil)
	if err != nil {
		return BandcampArtist{}, err
	}
	defer resp.Body.Close()

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return BandcampArtist{}, err
	}

	artist := BandcampArtist{URL: artistURL}

	// Extract artist name
	if nameNode := b.findNodeByTag(doc, "title"); nameNode != nil {
		title := b.getTextContent(nameNode)
		if strings.Contains(title, " | ") {
			parts := strings.Split(title, " | ")
			artist.Name = strings.TrimSpace(parts[0])
		} else {
			artist.Name = title
		}
	}

	// Extract location from bio or page
	if bioNode := b.findNodeByClass(doc, "bio"); bioNode != nil {
		bio := b.getTextContent(bioNode)
		location := b.extractLocationFromText(bio)
		artist.City = location.City
		artist.Country = location.Country
	}

	// Extract genres from tags
	tagNodes := b.findNodesByClass(doc, "tag")
	for _, tagNode := range tagNodes {
		tag := b.ExtractText(b.getTextContent(tagNode))
		if tag != "" {
			artist.Genres = append(artist.Genres, tag)
		}
	}

	if artist.Name == "" {
		artist.Name = "Unknown Artist"
	}

	return artist, nil
}

func (b *BandcampScraper) parseShowsPage(doc *html.Node, artist BandcampArtist) []ScrapedEvent {
	events := []ScrapedEvent{}

	// Find show listings
	var findShows func(*html.Node)
	findShows = func(n *html.Node) {
		if n.Type == html.ElementNode && b.hasClass(n, "show-item") {
			event := b.parseShowItem(n, artist)
			if event.Title != "" {
				events = append(events, event)
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findShows(c)
		}
	}

	findShows(doc)
	return events
}

func (b *BandcampScraper) parseShowItem(node *html.Node, artist BandcampArtist) ScrapedEvent {
	event := ScrapedEvent{
		ArtistName: artist.Name,
		Title:      artist.Name + " Live",
	}

	// Extract date
	if dateNode := b.findNodeByClass(node, "date"); dateNode != nil {
		dateStr := b.ExtractText(b.getTextContent(dateNode))
		event.Date = b.ParseDate(dateStr)
	}

	// Extract venue
	if venueNode := b.findNodeByClass(node, "venue"); venueNode != nil {
		event.VenueName = b.ExtractText(b.getTextContent(venueNode))
	}

	// Extract location
	if locationNode := b.findNodeByClass(node, "location"); locationNode != nil {
		location := b.ExtractText(b.getTextContent(locationNode))
		b.parseLocation(location, &event)
	}

	// Set defaults from artist info
	if event.City == "" {
		event.City = artist.City
	}
	if event.Country == "" {
		event.Country = artist.Country
	}

	event.Tags = artist.Genres

	return event
}

func (b *BandcampScraper) createEventsFromReleases(ctx context.Context, artistURL string, artist BandcampArtist, limit int) ([]ScrapedEvent, error) {
	// Create pseudo-events based on recent releases
	resp, err := b.MakeRequest(ctx, artistURL, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return nil, err
	}

	events := []ScrapedEvent{}

	// Find release items
	releaseNodes := b.findNodesByClass(doc, "music-grid-item")
	for i, releaseNode := range releaseNodes {
		if i >= limit {
			break
		}

		event := b.createEventFromRelease(releaseNode, artist)
		if event.Title != "" {
			events = append(events, event)
		}
	}

	return events, nil
}

func (b *BandcampScraper) createEventFromRelease(node *html.Node, artist BandcampArtist) ScrapedEvent {
	event := ScrapedEvent{
		ArtistName: artist.Name,
		City:       artist.City,
		Country:    artist.Country,
		Tags:       artist.Genres,
	}

	// Extract release title
	if titleNode := b.findNodeByClass(node, "title"); titleNode != nil {
		releaseTitle := b.ExtractText(b.getTextContent(titleNode))
		event.Title = fmt.Sprintf("%s - %s", artist.Name, releaseTitle)
	}

	// Use release as virtual venue
	event.VenueName = "Bandcamp Release"

	// Set date to recent (since it's a release, not a live event)
	event.Date = time.Now().AddDate(0, 0, -30) // 30 days ago

	return event
}

func (b *BandcampScraper) extractLocationFromText(text string) struct{ City, Country string } {
	location := struct{ City, Country string }{}

	// Simple location extraction patterns
	locationPatterns := []string{
		`(?i)from\s+([^,\n]+),?\s*([^,\n]+)`,
		`(?i)based\s+in\s+([^,\n]+),?\s*([^,\n]+)`,
		`(?i)located\s+in\s+([^,\n]+),?\s*([^,\n]+)`,
	}

	for _, pattern := range locationPatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(text)
		if len(matches) >= 3 {
			location.City = strings.TrimSpace(matches[1])
			location.Country = strings.TrimSpace(matches[2])
			break
		}
	}

	return location
}

func (b *BandcampScraper) isArtistURL(href string) bool {
	// Bandcamp artist URLs typically don't contain certain paths
	excludePaths := []string{"/search", "/tag/", "/discover", "/signup", "/login"}
	for _, exclude := range excludePaths {
		if strings.Contains(href, exclude) {
			return false
		}
	}

	// Should contain artist subdomain pattern or be a valid band page
	return strings.Contains(href, ".bandcamp.com") ||
		(strings.HasPrefix(href, "/") && !strings.Contains(href, "?"))
}

func (b *BandcampScraper) parseLocation(location string, event *ScrapedEvent) {
	parts := strings.Split(location, ",")
	if len(parts) >= 2 {
		event.City = strings.TrimSpace(parts[0])
		event.Country = strings.TrimSpace(parts[len(parts)-1])
	} else if len(parts) == 1 {
		event.City = strings.TrimSpace(parts[0])
	}
}

// Utility functions (similar to ResidentAdvisorScraper)
func (b *BandcampScraper) hasClass(node *html.Node, className string) bool {
	for _, attr := range node.Attr {
		if attr.Key == "class" && strings.Contains(attr.Val, className) {
			return true
		}
	}
	return false
}

func (b *BandcampScraper) findNodeByClass(node *html.Node, className string) *html.Node {
	if node.Type == html.ElementNode && b.hasClass(node, className) {
		return node
	}

	for c := node.FirstChild; c != nil; c = c.NextSibling {
		if result := b.findNodeByClass(c, className); result != nil {
			return result
		}
	}

	return nil
}

func (b *BandcampScraper) findNodesByClass(node *html.Node, className string) []*html.Node {
	var nodes []*html.Node

	var find func(*html.Node)
	find = func(n *html.Node) {
		if n.Type == html.ElementNode && b.hasClass(n, className) {
			nodes = append(nodes, n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			find(c)
		}
	}

	find(node)
	return nodes
}

func (b *BandcampScraper) findNodeByTag(node *html.Node, tagName string) *html.Node {
	if node.Type == html.ElementNode && node.Data == tagName {
		return node
	}

	for c := node.FirstChild; c != nil; c = c.NextSibling {
		if result := b.findNodeByTag(c, tagName); result != nil {
			return result
		}
	}

	return nil
}

func (b *BandcampScraper) getAttribute(node *html.Node, attrName string) string {
	for _, attr := range node.Attr {
		if attr.Key == attrName {
			return attr.Val
		}
	}
	return ""
}

func (b *BandcampScraper) getTextContent(node *html.Node) string {
	if node.Type == html.TextNode {
		return node.Data
	}

	var text strings.Builder
	for c := node.FirstChild; c != nil; c = c.NextSibling {
		text.WriteString(b.getTextContent(c))
	}

	return text.String()
}

type BandcampArtist struct {
	Name    string
	City    string
	Country string
	Genres  []string
	URL     string
}
