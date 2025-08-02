package music

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/yair/where-its-at/pkg/domain"
)

type MusicBrainzClient struct {
	baseURL     string
	userAgent   string
	httpClient  *http.Client
	rateLimiter *musicBrainzRateLimiter
}

type MusicBrainzConfig struct {
	UserAgent string // MusicBrainz requires identifying user agent
}

func NewMusicBrainzClient(config MusicBrainzConfig) (*MusicBrainzClient, error) {
	if config.UserAgent == "" {
		return nil, fmt.Errorf("musicbrainz user agent is required")
	}

	return &MusicBrainzClient{
		baseURL:     "https://musicbrainz.org/ws/2",
		userAgent:   config.UserAgent,
		httpClient:  &http.Client{Timeout: 10 * time.Second},
		rateLimiter: newMusicBrainzRateLimiter(),
	}, nil
}

type musicBrainzArtist struct {
	ID        string                `json:"id"`
	Name      string                `json:"name"`
	SortName  string                `json:"sort-name"`
	Type      string                `json:"type,omitempty"`
	Gender    string                `json:"gender,omitempty"`
	Country   string                `json:"country,omitempty"`
	Area      musicBrainzArea       `json:"area,omitempty"`
	BeginArea musicBrainzArea       `json:"begin-area,omitempty"`
	LifeSpan  musicBrainzLifeSpan   `json:"life-span,omitempty"`
	Tags      []musicBrainzTag      `json:"tags,omitempty"`
	Aliases   []musicBrainzAlias    `json:"aliases,omitempty"`
	Relations []musicBrainzRelation `json:"relations,omitempty"`
	Score     int                   `json:"score,omitempty"`
}

type musicBrainzArea struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	SortName string   `json:"sort-name"`
	ISO31661 []string `json:"iso-3166-1-codes,omitempty"`
}

type musicBrainzLifeSpan struct {
	Begin string `json:"begin,omitempty"`
	End   string `json:"end,omitempty"`
	Ended bool   `json:"ended,omitempty"`
}

type musicBrainzTag struct {
	Count int    `json:"count"`
	Name  string `json:"name"`
}

type musicBrainzAlias struct {
	Name     string `json:"name"`
	SortName string `json:"sort-name"`
	Type     string `json:"type,omitempty"`
	Primary  bool   `json:"primary,omitempty"`
	Locale   string `json:"locale,omitempty"`
}

type musicBrainzRelation struct {
	Type      string                    `json:"type"`
	Direction string                    `json:"direction"`
	URL       musicBrainzRelationURL    `json:"url,omitempty"`
	Artist    musicBrainzRelationArtist `json:"artist,omitempty"`
}

type musicBrainzRelationURL struct {
	ID       string `json:"id"`
	Resource string `json:"resource"`
}

type musicBrainzRelationArtist struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type musicBrainzSearchResponse struct {
	Created string              `json:"created"`
	Count   int                 `json:"count"`
	Offset  int                 `json:"offset"`
	Artists []musicBrainzArtist `json:"artists"`
}

func (c *MusicBrainzClient) SearchArtists(ctx context.Context, query string, limit int) ([]domain.Artist, error) {
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
	if limit > 100 {
		limit = 100
	}

	searchURL := fmt.Sprintf("%s/artist", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	q := req.URL.Query()
	q.Set("query", fmt.Sprintf("artist:%s", query))
	q.Set("limit", fmt.Sprintf("%d", limit))
	q.Set("fmt", "json")
	q.Set("inc", "tags+aliases+area-rels+url-rels")
	req.URL.RawQuery = q.Encode()

	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to search artists: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusServiceUnavailable {
		return nil, domain.ErrRateLimitExceeded
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("musicbrainz search failed: status %d", resp.StatusCode)
	}

	var searchResp musicBrainzSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	artists := make([]domain.Artist, 0, len(searchResp.Artists))
	for _, mbArtist := range searchResp.Artists {
		artist := c.convertToArtist(mbArtist)
		artists = append(artists, artist)
	}

	return artists, nil
}

func (c *MusicBrainzClient) GetArtist(ctx context.Context, musicBrainzID string) (*domain.Artist, error) {
	if err := c.rateLimiter.Allow(); err != nil {
		return nil, err
	}

	artistURL := fmt.Sprintf("%s/artist/%s", c.baseURL, musicBrainzID)
	req, err := http.NewRequestWithContext(ctx, "GET", artistURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	q := req.URL.Query()
	q.Set("fmt", "json")
	q.Set("inc", "tags+aliases+area-rels+url-rels+release-groups")
	req.URL.RawQuery = q.Encode()

	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get artist: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, domain.ErrArtistNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("musicbrainz get artist failed: status %d", resp.StatusCode)
	}

	var mbArtist musicBrainzArtist
	if err := json.NewDecoder(resp.Body).Decode(&mbArtist); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	artist := c.convertToArtist(mbArtist)
	return &artist, nil
}

func (c *MusicBrainzClient) GetArtistReleases(ctx context.Context, musicBrainzID string, limit int) ([]MusicBrainzRelease, error) {
	if err := c.rateLimiter.Allow(); err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	releaseURL := fmt.Sprintf("%s/release", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", releaseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	q := req.URL.Query()
	q.Set("artist", musicBrainzID)
	q.Set("limit", fmt.Sprintf("%d", limit))
	q.Set("fmt", "json")
	q.Set("inc", "release-groups+media+labels")
	req.URL.RawQuery = q.Encode()

	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("musicbrainz get releases failed: status %d", resp.StatusCode)
	}

	var response struct {
		Releases []musicBrainzRelease `json:"releases"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	releases := make([]MusicBrainzRelease, 0, len(response.Releases))
	for _, mbRelease := range response.Releases {
		release := MusicBrainzRelease{
			ID:        mbRelease.ID,
			Title:     mbRelease.Title,
			Date:      mbRelease.Date,
			Country:   mbRelease.Country,
			Status:    mbRelease.Status,
			Packaging: mbRelease.Packaging,
		}
		releases = append(releases, release)
	}

	return releases, nil
}

type MusicBrainzRelease struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Date      string `json:"date"`
	Country   string `json:"country"`
	Status    string `json:"status"`
	Packaging string `json:"packaging"`
}

type musicBrainzRelease struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Date      string `json:"date"`
	Country   string `json:"country"`
	Status    string `json:"status"`
	Packaging string `json:"packaging"`
}

func (c *MusicBrainzClient) convertToArtist(mbArtist musicBrainzArtist) domain.Artist {
	// Extract genres from tags
	genres := make([]string, 0, len(mbArtist.Tags))
	for _, tag := range mbArtist.Tags {
		// Only include tags with decent count/confidence
		if tag.Count >= 3 {
			genres = append(genres, tag.Name)
		}
	}

	// Calculate popularity based on score and tag counts
	popularity := 0
	if mbArtist.Score > 0 {
		popularity = mbArtist.Score
		if popularity > 100 {
			popularity = 100
		}
	} else {
		// Fallback: estimate popularity from tag activity
		totalTagCount := 0
		for _, tag := range mbArtist.Tags {
			totalTagCount += tag.Count
		}

		if totalTagCount > 1000 {
			popularity = 100
		} else if totalTagCount > 500 {
			popularity = 90
		} else if totalTagCount > 100 {
			popularity = 80
		} else if totalTagCount > 50 {
			popularity = 70
		} else if totalTagCount > 10 {
			popularity = 60
		} else {
			popularity = 50
		}
	}

	// Extract external URLs from relations
	externalIDs := domain.ExternalIDs{}
	for _, relation := range mbArtist.Relations {
		if relation.Type == "url" && relation.URL.Resource != "" {
			// Try to extract Spotify/LastFM IDs from URLs
			if strings.Contains(relation.URL.Resource, "spotify.com") {
				// Extract Spotify artist ID from URL
				parts := strings.Split(relation.URL.Resource, "/")
				if len(parts) > 0 {
					spotifyID := parts[len(parts)-1]
					if spotifyID != "" {
						externalIDs.SpotifyID = spotifyID
					}
				}
			}
			if strings.Contains(relation.URL.Resource, "last.fm") {
				// Extract LastFM artist name from URL
				parts := strings.Split(relation.URL.Resource, "/")
				for i, part := range parts {
					if part == "music" && i+1 < len(parts) {
						lastfmID := parts[i+1]
						if lastfmID != "" {
							externalIDs.LastFMID = lastfmID
						}
						break
					}
				}
			}
		}
	}

	return domain.Artist{
		ID:          fmt.Sprintf("musicbrainz_%s", mbArtist.ID),
		Name:        mbArtist.Name,
		Genres:      genres,
		Popularity:  popularity,
		ExternalIDs: externalIDs,
		// MusicBrainz doesn't provide direct image URLs
		ImageURL: "",
	}
}

// MusicBrainz-specific rate limiter (1 request per second)
type musicBrainzRateLimiter struct {
	lastRequest time.Time
	interval    time.Duration
}

func newMusicBrainzRateLimiter() *musicBrainzRateLimiter {
	return &musicBrainzRateLimiter{
		interval: time.Second, // MusicBrainz requires 1 second between requests
	}
}

func (r *musicBrainzRateLimiter) Allow() error {
	now := time.Now()
	if now.Sub(r.lastRequest) < r.interval {
		sleepTime := r.interval - now.Sub(r.lastRequest)
		time.Sleep(sleepTime)
	}
	r.lastRequest = time.Now()
	return nil
}
