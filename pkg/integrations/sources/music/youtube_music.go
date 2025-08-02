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

type YouTubeMusicClient struct {
	baseURL     string
	apiKey      string
	httpClient  *http.Client
	rateLimiter *rateLimiter
}

type YouTubeMusicConfig struct {
	APIKey string // YouTube Data API v3 key
}

func NewYouTubeMusicClient(config YouTubeMusicConfig) (*YouTubeMusicClient, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("youtube music API key is required")
	}

	return &YouTubeMusicClient{
		baseURL:     "https://www.googleapis.com/youtube/v3",
		apiKey:      config.APIKey,
		httpClient:  &http.Client{Timeout: 10 * time.Second},
		rateLimiter: newRateLimiter(10000), // 10k requests per day free tier
	}, nil
}

type youTubeChannel struct {
	ID         string                   `json:"id"`
	Snippet    youTubeChannelSnippet    `json:"snippet"`
	Statistics youTubeChannelStatistics `json:"statistics,omitempty"`
}

type youTubeChannelSnippet struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Thumbnails  struct {
		High struct {
			URL string `json:"url"`
		} `json:"high"`
		Medium struct {
			URL string `json:"url"`
		} `json:"medium"`
	} `json:"thumbnails"`
	Country string `json:"country,omitempty"`
}

type youTubeChannelStatistics struct {
	ViewCount             string `json:"viewCount"`
	SubscriberCount       string `json:"subscriberCount"`
	HiddenSubscriberCount bool   `json:"hiddenSubscriberCount"`
	VideoCount            string `json:"videoCount"`
}

type youTubeSearchResponse struct {
	Items         []youTubeSearchItem `json:"items"`
	NextPageToken string              `json:"nextPageToken,omitempty"`
}

type youTubeSearchItem struct {
	ID struct {
		Kind      string `json:"kind"`
		VideoID   string `json:"videoId,omitempty"`
		ChannelID string `json:"channelId,omitempty"`
	} `json:"id"`
	Snippet youTubeChannelSnippet `json:"snippet"`
}

func (c *YouTubeMusicClient) SearchArtists(ctx context.Context, query string, limit int) ([]domain.Artist, error) {
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

	// First search for channels (artists)
	searchURL := fmt.Sprintf("%s/search", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	q := req.URL.Query()
	q.Set("part", "snippet")
	q.Set("q", query+" music artist")
	q.Set("type", "channel")
	q.Set("maxResults", fmt.Sprintf("%d", limit))
	q.Set("key", c.apiKey)
	req.URL.RawQuery = q.Encode()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to search artists: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden {
		return nil, domain.ErrRateLimitExceeded
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("youtube music search failed: status %d", resp.StatusCode)
	}

	var searchResp youTubeSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(searchResp.Items) == 0 {
		return []domain.Artist{}, nil
	}

	// Get detailed channel information
	channelIDs := make([]string, 0, len(searchResp.Items))
	for _, item := range searchResp.Items {
		if item.ID.ChannelID != "" {
			channelIDs = append(channelIDs, item.ID.ChannelID)
		}
	}

	channels, err := c.getChannelDetails(ctx, channelIDs)
	if err != nil {
		// Fallback to basic info from search
		artists := make([]domain.Artist, 0, len(searchResp.Items))
		for _, item := range searchResp.Items {
			if item.ID.ChannelID != "" {
				artist := c.convertSearchItemToArtist(item)
				artists = append(artists, artist)
			}
		}
		return artists, nil
	}

	artists := make([]domain.Artist, 0, len(channels))
	for _, channel := range channels {
		artist := c.convertChannelToArtist(channel)
		artists = append(artists, artist)
	}

	return artists, nil
}

func (c *YouTubeMusicClient) getChannelDetails(ctx context.Context, channelIDs []string) ([]youTubeChannel, error) {
	if len(channelIDs) == 0 {
		return []youTubeChannel{}, nil
	}

	if err := c.rateLimiter.Allow(); err != nil {
		return nil, err
	}

	channelURL := fmt.Sprintf("%s/channels", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", channelURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	q := req.URL.Query()
	q.Set("part", "snippet,statistics")
	q.Set("id", strings.Join(channelIDs, ","))
	q.Set("key", c.apiKey)
	req.URL.RawQuery = q.Encode()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get channel details: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("youtube get channels failed: status %d", resp.StatusCode)
	}

	var response struct {
		Items []youTubeChannel `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return response.Items, nil
}

func (c *YouTubeMusicClient) GetArtist(ctx context.Context, channelID string) (*domain.Artist, error) {
	channels, err := c.getChannelDetails(ctx, []string{channelID})
	if err != nil {
		return nil, err
	}

	if len(channels) == 0 {
		return nil, domain.ErrArtistNotFound
	}

	artist := c.convertChannelToArtist(channels[0])
	return &artist, nil
}

func (c *YouTubeMusicClient) SearchVideos(ctx context.Context, artistName string, limit int) ([]YouTubeMusicVideo, error) {
	if err := c.rateLimiter.Allow(); err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	searchURL := fmt.Sprintf("%s/search", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	q := req.URL.Query()
	q.Set("part", "snippet")
	q.Set("q", artistName)
	q.Set("type", "video")
	q.Set("videoCategoryId", "10") // Music category
	q.Set("maxResults", fmt.Sprintf("%d", limit))
	q.Set("order", "relevance")
	q.Set("key", c.apiKey)
	req.URL.RawQuery = q.Encode()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to search videos: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("youtube video search failed: status %d", resp.StatusCode)
	}

	var searchResp youTubeSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	videos := make([]YouTubeMusicVideo, 0, len(searchResp.Items))
	for _, item := range searchResp.Items {
		if item.ID.VideoID != "" {
			video := YouTubeMusicVideo{
				ID:          item.ID.VideoID,
				Title:       item.Snippet.Title,
				Description: item.Snippet.Description,
				ThumbnailURL: func() string {
					if item.Snippet.Thumbnails.High.URL != "" {
						return item.Snippet.Thumbnails.High.URL
					}
					return item.Snippet.Thumbnails.Medium.URL
				}(),
				URL: fmt.Sprintf("https://www.youtube.com/watch?v=%s", item.ID.VideoID),
			}
			videos = append(videos, video)
		}
	}

	return videos, nil
}

type YouTubeMusicVideo struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	ThumbnailURL string `json:"thumbnail_url"`
	URL          string `json:"url"`
}

func (c *YouTubeMusicClient) convertSearchItemToArtist(item youTubeSearchItem) domain.Artist {
	imageURL := item.Snippet.Thumbnails.High.URL
	if imageURL == "" {
		imageURL = item.Snippet.Thumbnails.Medium.URL
	}

	return domain.Artist{
		ID:       fmt.Sprintf("youtube_%s", item.ID.ChannelID),
		Name:     item.Snippet.Title,
		ImageURL: imageURL,
		Genres:   c.extractGenresFromDescription(item.Snippet.Description),
	}
}

func (c *YouTubeMusicClient) convertChannelToArtist(channel youTubeChannel) domain.Artist {
	imageURL := channel.Snippet.Thumbnails.High.URL
	if imageURL == "" {
		imageURL = channel.Snippet.Thumbnails.Medium.URL
	}

	// Extract popularity from subscriber count
	popularity := 0
	if channel.Statistics.SubscriberCount != "" {
		// Simple popularity calculation based on subscriber count
		// This is a rough estimate since YouTube doesn't provide a direct popularity score
		var subscribers int64
		fmt.Sscanf(channel.Statistics.SubscriberCount, "%d", &subscribers)

		// Scale to 0-100 range (very rough approximation)
		if subscribers > 10000000 { // 10M+ subscribers
			popularity = 100
		} else if subscribers > 1000000 { // 1M+ subscribers
			popularity = 90
		} else if subscribers > 100000 { // 100k+ subscribers
			popularity = 80
		} else if subscribers > 10000 { // 10k+ subscribers
			popularity = 70
		} else if subscribers > 1000 { // 1k+ subscribers
			popularity = 60
		} else {
			popularity = 50
		}
	}

	return domain.Artist{
		ID:         fmt.Sprintf("youtube_%s", channel.ID),
		Name:       channel.Snippet.Title,
		ImageURL:   imageURL,
		Popularity: popularity,
		Genres:     c.extractGenresFromDescription(channel.Snippet.Description),
	}
}

func (c *YouTubeMusicClient) extractGenresFromDescription(description string) []string {
	// Simple genre extraction from description
	// This is a basic implementation - in production you'd want more sophisticated NLP
	genres := []string{}
	commonGenres := []string{
		"rock", "pop", "hip hop", "rap", "electronic", "dance", "classical",
		"jazz", "blues", "country", "folk", "metal", "punk", "reggae",
		"r&b", "soul", "funk", "indie", "alternative", "acoustic",
	}

	descLower := strings.ToLower(description)
	for _, genre := range commonGenres {
		if strings.Contains(descLower, genre) {
			genres = append(genres, genre)
		}
	}

	return genres
}
