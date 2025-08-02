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

type DeezerClient struct {
	baseURL     string
	httpClient  *http.Client
	rateLimiter *rateLimiter
}

type DeezerConfig struct {
	// Deezer API is free and doesn't require API key for basic search
}

func NewDeezerClient(config DeezerConfig) (*DeezerClient, error) {
	return &DeezerClient{
		baseURL:     "https://api.deezer.com",
		httpClient:  &http.Client{Timeout: 10 * time.Second},
		rateLimiter: newRateLimiter(50000), // Generous rate limit - Deezer is quite permissive
	}, nil
}

type deezerArtist struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	Link          string `json:"link"`
	Picture       string `json:"picture"`
	PictureSmall  string `json:"picture_small"`
	PictureMedium string `json:"picture_medium"`
	PictureBig    string `json:"picture_big"`
	PictureXL     string `json:"picture_xl"`
	NbAlbum       int    `json:"nb_album"`
	NbFan         int    `json:"nb_fan"`
	Radio         bool   `json:"radio"`
	Tracklist     string `json:"tracklist"`
}

type deezerSearchResponse struct {
	Data  []deezerArtist `json:"data"`
	Total int            `json:"total"`
	Next  string         `json:"next,omitempty"`
}

type deezerAlbum struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	UPC         string `json:"upc,omitempty"`
	Link        string `json:"link"`
	Cover       string `json:"cover"`
	CoverSmall  string `json:"cover_small"`
	CoverMedium string `json:"cover_medium"`
	CoverBig    string `json:"cover_big"`
	CoverXL     string `json:"cover_xl"`
	GenreID     int    `json:"genre_id"`
	Genres      struct {
		Data []deezerGenre `json:"data"`
	} `json:"genres"`
	Label          string       `json:"label"`
	NbTracks       int          `json:"nb_tracks"`
	Duration       int          `json:"duration"`
	Fans           int          `json:"fans"`
	ReleaseDate    string       `json:"release_date"`
	RecordType     string       `json:"record_type"`
	Available      bool         `json:"available"`
	Tracklist      string       `json:"tracklist"`
	ExplicitLyrics bool         `json:"explicit_lyrics"`
	Artist         deezerArtist `json:"artist"`
}

type deezerGenre struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

func (c *DeezerClient) SearchArtists(ctx context.Context, query string, limit int) ([]domain.Artist, error) {
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

	searchURL := fmt.Sprintf("%s/search/artist", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	q := req.URL.Query()
	q.Set("q", query)
	q.Set("limit", fmt.Sprintf("%d", limit))
	req.URL.RawQuery = q.Encode()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to search artists: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, domain.ErrRateLimitExceeded
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("deezer search failed: status %d", resp.StatusCode)
	}

	var searchResp deezerSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	artists := make([]domain.Artist, 0, len(searchResp.Data))
	for _, dzArtist := range searchResp.Data {
		artist := c.convertToArtist(dzArtist, nil)
		artists = append(artists, artist)
	}

	return artists, nil
}

func (c *DeezerClient) GetArtist(ctx context.Context, deezerID string) (*domain.Artist, error) {
	if err := c.rateLimiter.Allow(); err != nil {
		return nil, err
	}

	artistURL := fmt.Sprintf("%s/artist/%s", c.baseURL, deezerID)
	req, err := http.NewRequestWithContext(ctx, "GET", artistURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get artist: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, domain.ErrArtistNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("deezer get artist failed: status %d", resp.StatusCode)
	}

	var dzArtist deezerArtist
	if err := json.NewDecoder(resp.Body).Decode(&dzArtist); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Get artist's top albums to extract genres
	albums, err := c.GetArtistAlbums(ctx, deezerID, 5)
	if err != nil {
		// Continue without albums data
		albums = []DeezerAlbum{}
	}

	artist := c.convertToArtist(dzArtist, albums)
	return &artist, nil
}

func (c *DeezerClient) GetArtistAlbums(ctx context.Context, deezerID string, limit int) ([]DeezerAlbum, error) {
	if err := c.rateLimiter.Allow(); err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	albumsURL := fmt.Sprintf("%s/artist/%s/albums", c.baseURL, deezerID)
	req, err := http.NewRequestWithContext(ctx, "GET", albumsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	q := req.URL.Query()
	q.Set("limit", fmt.Sprintf("%d", limit))
	req.URL.RawQuery = q.Encode()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get albums: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("deezer get albums failed: status %d", resp.StatusCode)
	}

	var response struct {
		Data []deezerAlbum `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	albums := make([]DeezerAlbum, 0, len(response.Data))
	for _, dzAlbum := range response.Data {
		genres := make([]string, 0, len(dzAlbum.Genres.Data))
		for _, genre := range dzAlbum.Genres.Data {
			genres = append(genres, genre.Name)
		}

		album := DeezerAlbum{
			ID:          dzAlbum.ID,
			Title:       dzAlbum.Title,
			ArtistName:  dzAlbum.Artist.Name,
			ReleaseDate: dzAlbum.ReleaseDate,
			TrackCount:  dzAlbum.NbTracks,
			Genres:      genres,
			CoverURL:    dzAlbum.CoverXL,
			Label:       dzAlbum.Label,
			Fans:        dzAlbum.Fans,
		}
		albums = append(albums, album)
	}

	return albums, nil
}

func (c *DeezerClient) GetArtistTopTracks(ctx context.Context, deezerID string, limit int) ([]DeezerTrack, error) {
	if err := c.rateLimiter.Allow(); err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	topURL := fmt.Sprintf("%s/artist/%s/top", c.baseURL, deezerID)
	req, err := http.NewRequestWithContext(ctx, "GET", topURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	q := req.URL.Query()
	q.Set("limit", fmt.Sprintf("%d", limit))
	req.URL.RawQuery = q.Encode()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get top tracks: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("deezer get top tracks failed: status %d", resp.StatusCode)
	}

	var response struct {
		Data []deezerTrack `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	tracks := make([]DeezerTrack, 0, len(response.Data))
	for _, dzTrack := range response.Data {
		track := DeezerTrack{
			ID:             dzTrack.ID,
			Title:          dzTrack.Title,
			Duration:       dzTrack.Duration,
			Rank:           dzTrack.Rank,
			ExplicitLyrics: dzTrack.ExplicitLyrics,
			Preview:        dzTrack.Preview,
			ArtistName:     dzTrack.Artist.Name,
			AlbumTitle:     dzTrack.Album.Title,
		}
		tracks = append(tracks, track)
	}

	return tracks, nil
}

type DeezerAlbum struct {
	ID          int64    `json:"id"`
	Title       string   `json:"title"`
	ArtistName  string   `json:"artist_name"`
	ReleaseDate string   `json:"release_date"`
	TrackCount  int      `json:"track_count"`
	Genres      []string `json:"genres"`
	CoverURL    string   `json:"cover_url"`
	Label       string   `json:"label"`
	Fans        int      `json:"fans"`
}

type DeezerTrack struct {
	ID             int64  `json:"id"`
	Title          string `json:"title"`
	Duration       int    `json:"duration"`
	Rank           int    `json:"rank"`
	ExplicitLyrics bool   `json:"explicit_lyrics"`
	Preview        string `json:"preview"`
	ArtistName     string `json:"artist_name"`
	AlbumTitle     string `json:"album_title"`
}

type deezerTrack struct {
	ID             int64        `json:"id"`
	Title          string       `json:"title"`
	Duration       int          `json:"duration"`
	Rank           int          `json:"rank"`
	ExplicitLyrics bool         `json:"explicit_lyrics"`
	Preview        string       `json:"preview"`
	Artist         deezerArtist `json:"artist"`
	Album          struct {
		ID    int64  `json:"id"`
		Title string `json:"title"`
		Cover string `json:"cover"`
	} `json:"album"`
}

func (c *DeezerClient) convertToArtist(dzArtist deezerArtist, albums []DeezerAlbum) domain.Artist {
	// Calculate popularity based on fan count
	popularity := 0
	if dzArtist.NbFan > 10000000 { // 10M+ fans
		popularity = 100
	} else if dzArtist.NbFan > 1000000 { // 1M+ fans
		popularity = 95
	} else if dzArtist.NbFan > 500000 { // 500k+ fans
		popularity = 90
	} else if dzArtist.NbFan > 100000 { // 100k+ fans
		popularity = 85
	} else if dzArtist.NbFan > 50000 { // 50k+ fans
		popularity = 80
	} else if dzArtist.NbFan > 10000 { // 10k+ fans
		popularity = 75
	} else if dzArtist.NbFan > 5000 { // 5k+ fans
		popularity = 70
	} else if dzArtist.NbFan > 1000 { // 1k+ fans
		popularity = 65
	} else {
		popularity = 60
	}

	// Extract genres from albums
	genreSet := make(map[string]bool)
	for _, album := range albums {
		for _, genre := range album.Genres {
			genreSet[strings.ToLower(genre)] = true
		}
	}

	genres := make([]string, 0, len(genreSet))
	for genre := range genreSet {
		genres = append(genres, genre)
	}

	// Use the highest quality image available
	imageURL := dzArtist.PictureXL
	if imageURL == "" {
		imageURL = dzArtist.PictureBig
	}
	if imageURL == "" {
		imageURL = dzArtist.PictureMedium
	}

	return domain.Artist{
		ID:          fmt.Sprintf("deezer_%d", dzArtist.ID),
		Name:        dzArtist.Name,
		Genres:      genres,
		Popularity:  popularity,
		ImageURL:    imageURL,
		ExternalIDs: domain.ExternalIDs{
			// Deezer doesn't provide cross-platform IDs
		},
	}
}
