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

type SoundCloudClient struct {
	baseURL     string
	clientID    string
	httpClient  *http.Client
	rateLimiter *rateLimiter
}

type SoundCloudConfig struct {
	ClientID string // SoundCloud API requires client ID
}

func NewSoundCloudClient(config SoundCloudConfig) (*SoundCloudClient, error) {
	if config.ClientID == "" {
		return nil, fmt.Errorf("soundcloud client ID is required")
	}

	return &SoundCloudClient{
		baseURL:     "https://api.soundcloud.com",
		clientID:    config.ClientID,
		httpClient:  &http.Client{Timeout: 10 * time.Second},
		rateLimiter: newRateLimiter(15000), // 15k requests per hour for registered apps
	}, nil
}

type soundCloudUser struct {
	ID              int64  `json:"id"`
	Kind            string `json:"kind"`
	Permalink       string `json:"permalink"`
	Username        string `json:"username"`
	URI             string `json:"uri"`
	PermalinkURL    string `json:"permalink_url"`
	AvatarURL       string `json:"avatar_url"`
	Country         string `json:"country,omitempty"`
	FullName        string `json:"full_name,omitempty"`
	City            string `json:"city,omitempty"`
	Description     string `json:"description,omitempty"`
	DiscogsName     string `json:"discogs_name,omitempty"`
	MyspaceName     string `json:"myspace_name,omitempty"`
	Website         string `json:"website,omitempty"`
	WebsiteTitle    string `json:"website_title,omitempty"`
	Online          bool   `json:"online"`
	TrackCount      int    `json:"track_count"`
	PlaylistCount   int    `json:"playlist_count"`
	FollowersCount  int    `json:"followers_count"`
	FollowingsCount int    `json:"followings_count"`
	PublicFavorites bool   `json:"public_favorites_count"`
}

type soundCloudTrack struct {
	ID               int64          `json:"id"`
	Kind             string         `json:"kind"`
	CreatedAt        string         `json:"created_at"`
	UserID           int64          `json:"user_id"`
	User             soundCloudUser `json:"user"`
	Title            string         `json:"title"`
	Permalink        string         `json:"permalink"`
	PermalinkURL     string         `json:"permalink_url"`
	URI              string         `json:"uri"`
	Sharing          string         `json:"sharing"`
	EmbeddableBy     string         `json:"embeddable_by"`
	PurchaseURL      string         `json:"purchase_url,omitempty"`
	ArtworkURL       string         `json:"artwork_url,omitempty"`
	Description      string         `json:"description,omitempty"`
	Label            soundCloudUser `json:"label,omitempty"`
	Duration         int            `json:"duration"`
	Genre            string         `json:"genre,omitempty"`
	TagList          string         `json:"tag_list"`
	LabelName        string         `json:"label_name,omitempty"`
	Release          string         `json:"release,omitempty"`
	StreamURL        string         `json:"stream_url"`
	DownloadURL      string         `json:"download_url,omitempty"`
	PlaybackCount    int            `json:"playback_count"`
	FavoritingsCount int            `json:"favoritings_count"`
	CommentCount     int            `json:"comment_count"`
	Downloadable     bool           `json:"downloadable"`
	Waveform         soundCloudWave `json:"waveform_url"`
	AttachmentsURI   string         `json:"attachments_uri"`
}

type soundCloudWave struct {
	URL string `json:"url"`
}

type soundCloudSearchResponse struct {
	Collection []soundCloudUser `json:"collection"`
	NextHref   string           `json:"next_href,omitempty"`
}

func (c *SoundCloudClient) SearchArtists(ctx context.Context, query string, limit int) ([]domain.Artist, error) {
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
	if limit > 200 {
		limit = 200
	}

	searchURL := fmt.Sprintf("%s/users", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	q := req.URL.Query()
	q.Set("q", query)
	q.Set("limit", fmt.Sprintf("%d", limit))
	q.Set("client_id", c.clientID)
	req.URL.RawQuery = q.Encode()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to search artists: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, domain.ErrRateLimitExceeded
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("soundcloud unauthorized: invalid client ID")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("soundcloud search failed: status %d", resp.StatusCode)
	}

	var searchResp soundCloudSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	artists := make([]domain.Artist, 0, len(searchResp.Collection))
	for _, scUser := range searchResp.Collection {
		if scUser.TrackCount > 0 { // Only users with tracks are likely artists
			artist := c.convertToArtist(scUser)
			artists = append(artists, artist)
		}
	}

	return artists, nil
}

func (c *SoundCloudClient) GetArtist(ctx context.Context, soundCloudID string) (*domain.Artist, error) {
	if err := c.rateLimiter.Allow(); err != nil {
		return nil, err
	}

	userURL := fmt.Sprintf("%s/users/%s", c.baseURL, soundCloudID)
	req, err := http.NewRequestWithContext(ctx, "GET", userURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	q := req.URL.Query()
	q.Set("client_id", c.clientID)
	req.URL.RawQuery = q.Encode()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get artist: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, domain.ErrArtistNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("soundcloud get artist failed: status %d", resp.StatusCode)
	}

	var scUser soundCloudUser
	if err := json.NewDecoder(resp.Body).Decode(&scUser); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	artist := c.convertToArtist(scUser)
	return &artist, nil
}

func (c *SoundCloudClient) GetArtistTracks(ctx context.Context, soundCloudID string, limit int) ([]SoundCloudTrack, error) {
	if err := c.rateLimiter.Allow(); err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = 10
	}
	if limit > 200 {
		limit = 200
	}

	tracksURL := fmt.Sprintf("%s/users/%s/tracks", c.baseURL, soundCloudID)
	req, err := http.NewRequestWithContext(ctx, "GET", tracksURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	q := req.URL.Query()
	q.Set("limit", fmt.Sprintf("%d", limit))
	q.Set("client_id", c.clientID)
	req.URL.RawQuery = q.Encode()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get tracks: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("soundcloud get tracks failed: status %d", resp.StatusCode)
	}

	var scTracks []soundCloudTrack
	if err := json.NewDecoder(resp.Body).Decode(&scTracks); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	tracks := make([]SoundCloudTrack, 0, len(scTracks))
	for _, scTrack := range scTracks {
		track := SoundCloudTrack{
			ID:               scTrack.ID,
			Title:            scTrack.Title,
			Genre:            scTrack.Genre,
			Duration:         scTrack.Duration,
			PlaybackCount:    scTrack.PlaybackCount,
			FavoritingsCount: scTrack.FavoritingsCount,
			CommentCount:     scTrack.CommentCount,
			CreatedAt:        scTrack.CreatedAt,
			Description:      scTrack.Description,
			PermalinkURL:     scTrack.PermalinkURL,
			ArtworkURL:       c.processArtworkURL(scTrack.ArtworkURL),
			Tags:             c.parseTags(scTrack.TagList),
		}
		tracks = append(tracks, track)
	}

	return tracks, nil
}

func (c *SoundCloudClient) SearchTracks(ctx context.Context, query string, limit int) ([]SoundCloudTrack, error) {
	if err := c.rateLimiter.Allow(); err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = 10
	}
	if limit > 200 {
		limit = 200
	}

	searchURL := fmt.Sprintf("%s/tracks", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	q := req.URL.Query()
	q.Set("q", query)
	q.Set("limit", fmt.Sprintf("%d", limit))
	q.Set("client_id", c.clientID)
	req.URL.RawQuery = q.Encode()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to search tracks: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("soundcloud track search failed: status %d", resp.StatusCode)
	}

	var searchResp struct {
		Collection []soundCloudTrack `json:"collection"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	tracks := make([]SoundCloudTrack, 0, len(searchResp.Collection))
	for _, scTrack := range searchResp.Collection {
		track := SoundCloudTrack{
			ID:               scTrack.ID,
			Title:            scTrack.Title,
			Genre:            scTrack.Genre,
			Duration:         scTrack.Duration,
			PlaybackCount:    scTrack.PlaybackCount,
			FavoritingsCount: scTrack.FavoritingsCount,
			CommentCount:     scTrack.CommentCount,
			CreatedAt:        scTrack.CreatedAt,
			Description:      scTrack.Description,
			PermalinkURL:     scTrack.PermalinkURL,
			ArtworkURL:       c.processArtworkURL(scTrack.ArtworkURL),
			Tags:             c.parseTags(scTrack.TagList),
			UserName:         scTrack.User.Username,
		}
		tracks = append(tracks, track)
	}

	return tracks, nil
}

type SoundCloudTrack struct {
	ID               int64    `json:"id"`
	Title            string   `json:"title"`
	Genre            string   `json:"genre"`
	Duration         int      `json:"duration"`
	PlaybackCount    int      `json:"playback_count"`
	FavoritingsCount int      `json:"favoritings_count"`
	CommentCount     int      `json:"comment_count"`
	CreatedAt        string   `json:"created_at"`
	Description      string   `json:"description"`
	PermalinkURL     string   `json:"permalink_url"`
	ArtworkURL       string   `json:"artwork_url"`
	Tags             []string `json:"tags"`
	UserName         string   `json:"user_name"`
}

func (c *SoundCloudClient) convertToArtist(scUser soundCloudUser) domain.Artist {
	// Calculate popularity based on followers and track engagement
	popularity := 0

	// Base popularity on follower count
	if scUser.FollowersCount > 1000000 { // 1M+ followers
		popularity = 100
	} else if scUser.FollowersCount > 500000 { // 500k+ followers
		popularity = 95
	} else if scUser.FollowersCount > 100000 { // 100k+ followers
		popularity = 90
	} else if scUser.FollowersCount > 50000 { // 50k+ followers
		popularity = 85
	} else if scUser.FollowersCount > 10000 { // 10k+ followers
		popularity = 80
	} else if scUser.FollowersCount > 5000 { // 5k+ followers
		popularity = 75
	} else if scUser.FollowersCount > 1000 { // 1k+ followers
		popularity = 70
	} else if scUser.FollowersCount > 500 { // 500+ followers
		popularity = 65
	} else if scUser.FollowersCount > 100 { // 100+ followers
		popularity = 60
	} else {
		popularity = 50
	}

	// Boost popularity based on track count (active artists)
	if scUser.TrackCount > 100 {
		popularity = min(100, popularity+5)
	} else if scUser.TrackCount > 50 {
		popularity = min(100, popularity+3)
	} else if scUser.TrackCount > 20 {
		popularity = min(100, popularity+2)
	}

	// Extract potential genres from description
	genres := c.extractGenresFromDescription(scUser.Description)

	// Use highest quality avatar available
	avatarURL := c.processArtworkURL(scUser.AvatarURL)

	return domain.Artist{
		ID:          fmt.Sprintf("soundcloud_%d", scUser.ID),
		Name:        c.getDisplayName(scUser),
		Genres:      genres,
		Popularity:  popularity,
		ImageURL:    avatarURL,
		ExternalIDs: domain.ExternalIDs{
			// SoundCloud doesn't provide cross-platform IDs directly
		},
	}
}

func (c *SoundCloudClient) getDisplayName(user soundCloudUser) string {
	if user.FullName != "" {
		return user.FullName
	}
	return user.Username
}

func (c *SoundCloudClient) extractGenresFromDescription(description string) []string {
	if description == "" {
		return []string{}
	}

	genres := []string{}
	commonGenres := []string{
		"electronic", "house", "techno", "trance", "dubstep", "drum and bass", "dnb",
		"hip hop", "rap", "trap", "lo-fi", "chill", "ambient", "downtempo",
		"rock", "indie", "alternative", "pop", "folk", "acoustic",
		"jazz", "blues", "soul", "funk", "r&b", "reggae",
		"experimental", "synthwave", "vaporwave", "future bass", "deep house",
	}

	descLower := strings.ToLower(description)
	for _, genre := range commonGenres {
		if strings.Contains(descLower, genre) {
			genres = append(genres, genre)
		}
	}

	return genres
}

func (c *SoundCloudClient) processArtworkURL(artworkURL string) string {
	if artworkURL == "" {
		return ""
	}

	// SoundCloud artwork URLs can be upgraded to higher resolution
	// Replace "large" with "t500x500" for better quality
	processed := strings.Replace(artworkURL, "large.jpg", "t500x500.jpg", 1)
	processed = strings.Replace(processed, "large.png", "t500x500.png", 1)

	return processed
}

func (c *SoundCloudClient) parseTags(tagList string) []string {
	if tagList == "" {
		return []string{}
	}

	// SoundCloud tag list is typically space or quote separated
	tags := []string{}

	// Handle quoted tags
	parts := strings.Split(tagList, "\"")
	for i, part := range parts {
		if i%2 == 1 { // Odd indices are inside quotes
			tag := strings.TrimSpace(part)
			if tag != "" {
				tags = append(tags, tag)
			}
		} else {
			// Even indices, split by spaces
			spaceParts := strings.Fields(part)
			for _, spacePart := range spaceParts {
				tag := strings.TrimSpace(spacePart)
				if tag != "" {
					tags = append(tags, tag)
				}
			}
		}
	}

	return tags
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
