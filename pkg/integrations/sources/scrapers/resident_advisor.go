package scrapers

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

type ResidentAdvisorScraper struct {
	*BaseScraper
	baseURL string
}

func NewResidentAdvisorScraper(config ScrapingConfig) *ResidentAdvisorScraper {
	return &ResidentAdvisorScraper{
		BaseScraper: NewBaseScraper(config),
		baseURL:     "https://ra.co",
	}
}

func (r *ResidentAdvisorScraper) GetName() string {
	return "resident_advisor"
}

func (r *ResidentAdvisorScraper) ScrapeEvents(ctx context.Context, query string, limit int) ([]ScrapedEvent, error) {
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

	// RA search URL
	searchURL := fmt.Sprintf("%s/events/search?query=%s", r.baseURL, url.QueryEscape(query))

	resp, err := r.MakeRequest(ctx, searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch search results: %w", err)
	}
	defer resp.Body.Close()

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	events := r.parseEventList(doc, limit)

	// Enhance events with detailed information
	for i := range events {
		if eventURL := events[i].URL; eventURL != "" {
			if detailed, err := r.scrapeEventDetails(ctx, eventURL); err == nil {
				events[i] = r.mergeEventDetails(events[i], detailed)
			}
		}
	}

	return events, nil
}

func (r *ResidentAdvisorScraper) ScrapeEventsByLocation(ctx context.Context, city, country string, limit int) ([]ScrapedEvent, error) {
	city = strings.TrimSpace(city)
	if city == "" {
		return nil, fmt.Errorf("city cannot be empty")
	}

	if limit <= 0 {
		limit = 10
	}

	// RA uses location codes, try to find the right one
	locationCode := r.getLocationCode(city, country)
	if locationCode == "" {
		// Fallback to city name search
		return r.ScrapeEvents(ctx, city, limit)
	}

	eventsURL := fmt.Sprintf("%s/events/%s", r.baseURL, locationCode)

	resp, err := r.MakeRequest(ctx, eventsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch location events: %w", err)
	}
	defer resp.Body.Close()

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	events := r.parseEventList(doc, limit)

	// Enhance with details
	for i := range events {
		if eventURL := events[i].URL; eventURL != "" {
			if detailed, err := r.scrapeEventDetails(ctx, eventURL); err == nil {
				events[i] = r.mergeEventDetails(events[i], detailed)
			}
		}
	}

	return events, nil
}

func (r *ResidentAdvisorScraper) parseEventList(doc *html.Node, limit int) []ScrapedEvent {
	events := []ScrapedEvent{}

	// Find event containers (RA uses specific CSS classes)
	eventNodes := r.findEventNodes(doc)

	for i, node := range eventNodes {
		if i >= limit {
			break
		}

		event := r.parseEventNode(node)
		if event.Title != "" {
			events = append(events, event)
		}
	}

	return events
}

func (r *ResidentAdvisorScraper) findEventNodes(doc *html.Node) []*html.Node {
	var eventNodes []*html.Node

	var findEvents func(*html.Node)
	findEvents = func(n *html.Node) {
		if n.Type == html.ElementNode {
			// Look for common RA event container classes
			if r.hasClass(n, "eventListingItem") ||
				r.hasClass(n, "event-item") ||
				r.hasClass(n, "listing-item") {
				eventNodes = append(eventNodes, n)
				return
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findEvents(c)
		}
	}

	findEvents(doc)
	return eventNodes
}

func (r *ResidentAdvisorScraper) parseEventNode(node *html.Node) ScrapedEvent {
	event := ScrapedEvent{}

	// Extract title/artist
	if titleNode := r.findNodeByClass(node, "title"); titleNode != nil {
		event.Title = r.ExtractText(r.getTextContent(titleNode))
		event.ArtistName = event.Title
	}

	// Extract date
	if dateNode := r.findNodeByClass(node, "date"); dateNode != nil {
		dateStr := r.ExtractText(r.getTextContent(dateNode))
		event.Date = r.ParseDate(dateStr)
	}

	// Extract venue
	if venueNode := r.findNodeByClass(node, "venue"); venueNode != nil {
		event.VenueName = r.ExtractText(r.getTextContent(venueNode))
	}

	// Extract location
	if locationNode := r.findNodeByClass(node, "location"); locationNode != nil {
		location := r.ExtractText(r.getTextContent(locationNode))
		r.parseLocation(location, &event)
	}

	// Extract URL
	if linkNode := r.findNodeByTag(node, "a"); linkNode != nil {
		if href := r.getAttribute(linkNode, "href"); href != "" {
			if fullURL, err := r.NormalizeURL(r.baseURL, href); err == nil {
				event.URL = fullURL
			}
		}
	}

	// Set defaults
	if event.ArtistName == "" {
		event.ArtistName = "Unknown Artist"
	}
	if event.Date.IsZero() {
		event.Date = time.Now()
	}

	return event
}

func (r *ResidentAdvisorScraper) scrapeEventDetails(ctx context.Context, eventURL string) (ScrapedEvent, error) {
	resp, err := r.MakeRequest(ctx, eventURL, nil)
	if err != nil {
		return ScrapedEvent{}, err
	}
	defer resp.Body.Close()

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return ScrapedEvent{}, err
	}

	event := ScrapedEvent{URL: eventURL}

	// Extract detailed information
	if descNode := r.findNodeByClass(doc, "event-description"); descNode != nil {
		event.Description = r.ExtractText(r.getTextContent(descNode))
	}

	if priceNode := r.findNodeByClass(doc, "price"); priceNode != nil {
		event.Price = r.ExtractText(r.getTextContent(priceNode))
	}

	// Extract tags/genres
	tagNodes := r.findNodesByClass(doc, "tag")
	for _, tagNode := range tagNodes {
		tag := r.ExtractText(r.getTextContent(tagNode))
		if tag != "" {
			event.Tags = append(event.Tags, tag)
		}
	}

	return event, nil
}

func (r *ResidentAdvisorScraper) mergeEventDetails(base, detailed ScrapedEvent) ScrapedEvent {
	if detailed.Description != "" {
		base.Description = detailed.Description
	}
	if detailed.Price != "" {
		base.Price = detailed.Price
	}
	if len(detailed.Tags) > 0 {
		base.Tags = detailed.Tags
	}
	return base
}

func (r *ResidentAdvisorScraper) getLocationCode(city, country string) string {
	// Common RA location codes
	locationMap := map[string]string{
		"london":        "london",
		"berlin":        "berlin",
		"new york":      "newyork",
		"paris":         "paris",
		"amsterdam":     "amsterdam",
		"barcelona":     "barcelona",
		"melbourne":     "melbourne",
		"sydney":        "sydney",
		"tokyo":         "tokyo",
		"montreal":      "montreal",
		"toronto":       "toronto",
		"chicago":       "chicago",
		"los angeles":   "losangeles",
		"san francisco": "sanfrancisco",
		"detroit":       "detroit",
		"miami":         "miami",
		"dublin":        "dublin",
		"glasgow":       "glasgow",
		"manchester":    "manchester",
		"birmingham":    "birmingham",
		"bristol":       "bristol",
		"leeds":         "leeds",
		"liverpool":     "liverpool",
		"rome":          "rome",
		"milan":         "milan",
		"madrid":        "madrid",
		"valencia":      "valencia",
		"lisbon":        "lisbon",
		"prague":        "prague",
		"vienna":        "vienna",
		"zurich":        "zurich",
		"oslo":          "oslo",
		"stockholm":     "stockholm",
		"copenhagen":    "copenhagen",
		"helsinki":      "helsinki",
	}

	cityLower := strings.ToLower(city)
	return locationMap[cityLower]
}

func (r *ResidentAdvisorScraper) parseLocation(location string, event *ScrapedEvent) {
	parts := strings.Split(location, ",")
	if len(parts) >= 2 {
		event.City = strings.TrimSpace(parts[0])
		event.Country = strings.TrimSpace(parts[len(parts)-1])
	} else if len(parts) == 1 {
		event.City = strings.TrimSpace(parts[0])
	}
}

// Utility functions for HTML parsing
func (r *ResidentAdvisorScraper) hasClass(node *html.Node, className string) bool {
	for _, attr := range node.Attr {
		if attr.Key == "class" && strings.Contains(attr.Val, className) {
			return true
		}
	}
	return false
}

func (r *ResidentAdvisorScraper) findNodeByClass(node *html.Node, className string) *html.Node {
	if node.Type == html.ElementNode && r.hasClass(node, className) {
		return node
	}

	for c := node.FirstChild; c != nil; c = c.NextSibling {
		if result := r.findNodeByClass(c, className); result != nil {
			return result
		}
	}

	return nil
}

func (r *ResidentAdvisorScraper) findNodesByClass(node *html.Node, className string) []*html.Node {
	var nodes []*html.Node

	var find func(*html.Node)
	find = func(n *html.Node) {
		if n.Type == html.ElementNode && r.hasClass(n, className) {
			nodes = append(nodes, n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			find(c)
		}
	}

	find(node)
	return nodes
}

func (r *ResidentAdvisorScraper) findNodeByTag(node *html.Node, tagName string) *html.Node {
	if node.Type == html.ElementNode && node.Data == tagName {
		return node
	}

	for c := node.FirstChild; c != nil; c = c.NextSibling {
		if result := r.findNodeByTag(c, tagName); result != nil {
			return result
		}
	}

	return nil
}

func (r *ResidentAdvisorScraper) getAttribute(node *html.Node, attrName string) string {
	for _, attr := range node.Attr {
		if attr.Key == attrName {
			return attr.Val
		}
	}
	return ""
}

func (r *ResidentAdvisorScraper) getTextContent(node *html.Node) string {
	if node.Type == html.TextNode {
		return node.Data
	}

	var text strings.Builder
	for c := node.FirstChild; c != nil; c = c.NextSibling {
		text.WriteString(r.getTextContent(c))
	}

	return text.String()
}
