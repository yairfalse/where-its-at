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

type AppleMusicClient struct {
	baseURL     string
	token       string
	httpClient  *http.Client
	rateLimiter *rateLimiter
}

type AppleMusicConfig struct {
	Token string // Apple Music API requires JWT token
}

func NewAppleMusicClient(config AppleMusicConfig) (*AppleMusicClient, error) {
	if config.Token == "" {
		return nil, fmt.Errorf("apple music token is required")
	}

	return &AppleMusicClient{
		baseURL:     "https://api.music.apple.com/v1",
		token:       config.Token,
		httpClient:  &http.Client{Timeout: 10 * time.Second},
		rateLimiter: newRateLimiter(20000), // 20k requests per hour
	}, nil
}

type appleMusicArtist struct {
	ID         string                     `json:"id"`
	Type       string                     `json:"type"`
	Attributes appleMusicArtistAttributes `json:"attributes"`
}

type appleMusicArtistAttributes struct {
	Name       string   `json:"name"`
	GenreNames []string `json:"genreNames"`
	URL        string   `json:"url"`
	ArtworkURL string   `json:"artworkUrl"`
}

type appleMusicSearchResponse struct {
	Results struct {
		Artists struct {
			Data []appleMusicArtist `json:"data"`
		} `json:"artists"`
	} `json:"results"`
}

func (c *AppleMusicClient) SearchArtists(ctx context.Context, query string, limit int) ([]domain.Artist, error) {
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
	if limit > 25 {
		limit = 25
	}

	searchURL := fmt.Sprintf("%s/catalog/us/search", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	q := req.URL.Query()
	q.Set("term", query)
	q.Set("types", "artists")
	q.Set("limit", fmt.Sprintf("%d", limit))
	req.URL.RawQuery = q.Encode()

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Music-User-Token", c.token) // For personalized results

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to search artists: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("apple music unauthorized: invalid token")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("apple music search failed: status %d", resp.StatusCode)
	}

	var searchResp appleMusicSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	artists := make([]domain.Artist, 0, len(searchResp.Results.Artists.Data))
	for _, amArtist := range searchResp.Results.Artists.Data {
		artist := domain.Artist{
			ID:          fmt.Sprintf("apple_%s", amArtist.ID),
			Name:        amArtist.Attributes.Name,
			Genres:      amArtist.Attributes.GenreNames,
			ImageURL:    c.processArtworkURL(amArtist.Attributes.ArtworkURL),
			ExternalIDs: domain.ExternalIDs{
				// Apple Music doesn't provide Spotify/LastFM IDs
			},
		}

		artists = append(artists, artist)
	}

	return artists, nil
}

func (c *AppleMusicClient) GetArtist(ctx context.Context, appleMusicID string) (*domain.Artist, error) {
	if err := c.rateLimiter.Allow(); err != nil {
		return nil, err
	}

	artistURL := fmt.Sprintf("%s/catalog/us/artists/%s", c.baseURL, appleMusicID)
	req, err := http.NewRequestWithContext(ctx, "GET", artistURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get artist: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, domain.ErrArtistNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("apple music get artist failed: status %d", resp.StatusCode)
	}

	var response struct {
		Data []appleMusicArtist `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(response.Data) == 0 {
		return nil, domain.ErrArtistNotFound
	}

	amArtist := response.Data[0]
	artist := &domain.Artist{
		ID:       fmt.Sprintf("apple_%s", amArtist.ID),
		Name:     amArtist.Attributes.Name,
		Genres:   amArtist.Attributes.GenreNames,
		ImageURL: c.processArtworkURL(amArtist.Attributes.ArtworkURL),
	}

	return artist, nil
}

func (c *AppleMusicClient) GetArtistAlbums(ctx context.Context, appleMusicID string, limit int) ([]AppleMusicAlbum, error) {
	if err := c.rateLimiter.Allow(); err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	albumsURL := fmt.Sprintf("%s/catalog/us/artists/%s/albums", c.baseURL, appleMusicID)
	req, err := http.NewRequestWithContext(ctx, "GET", albumsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	q := req.URL.Query()
	q.Set("limit", fmt.Sprintf("%d", limit))
	req.URL.RawQuery = q.Encode()

	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get albums: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("apple music get albums failed: status %d", resp.StatusCode)
	}

	var response struct {
		Data []appleMusicAlbum `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	albums := make([]AppleMusicAlbum, 0, len(response.Data))
	for _, amAlbum := range response.Data {
		album := AppleMusicAlbum{
			ID:          amAlbum.ID,
			Name:        amAlbum.Attributes.Name,
			ArtistName:  amAlbum.Attributes.ArtistName,
			ReleaseDate: amAlbum.Attributes.ReleaseDate,
			TrackCount:  amAlbum.Attributes.TrackCount,
			GenreNames:  amAlbum.Attributes.GenreNames,
			ArtworkURL:  c.processArtworkURL(amAlbum.Attributes.Artwork.URL),
		}
		albums = append(albums, album)
	}

	return albums, nil
}

type AppleMusicAlbum struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	ArtistName  string   `json:"artist_name"`
	ReleaseDate string   `json:"release_date"`
	TrackCount  int      `json:"track_count"`
	GenreNames  []string `json:"genre_names"`
	ArtworkURL  string   `json:"artwork_url"`
}

type appleMusicAlbum struct {
	ID         string                    `json:"id"`
	Type       string                    `json:"type"`
	Attributes appleMusicAlbumAttributes `json:"attributes"`
}

type appleMusicAlbumAttributes struct {
	Name        string   `json:"name"`
	ArtistName  string   `json:"artistName"`
	ReleaseDate string   `json:"releaseDate"`
	TrackCount  int      `json:"trackCount"`
	GenreNames  []string `json:"genreNames"`
	Artwork     struct {
		URL    string `json:"url"`
		Width  int    `json:"width"`
		Height int    `json:"height"`
	} `json:"artwork"`
}

func (c *AppleMusicClient) processArtworkURL(artworkURL string) string {
	if artworkURL == "" {
		return ""
	}
	// Apple Music artwork URLs use placeholders for size
	// Replace with high-quality 512x512 version
	processed := strings.Replace(artworkURL, "{w}", "512", -1)
	processed = strings.Replace(processed, "{h}", "512", -1)
	return processed
}

// Rate limiter implementation for Apple Music (20k requests per hour)
type rateLimiter struct {
	requests   []time.Time
	limit      int
	windowSize time.Duration
}

func newRateLimiter(hourlyLimit int) *rateLimiter {
	return &rateLimiter{
		requests:   make([]time.Time, 0),
		limit:      hourlyLimit,
		windowSize: time.Hour,
	}
}

func (r *rateLimiter) Allow() error {
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
