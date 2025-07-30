package integrations

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/yair/where-its-at/pkg/domain"
)

type LastFMClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

type LastFMConfig struct {
	APIKey string
}

func NewLastFMClient(config LastFMConfig) (*LastFMClient, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("last.fm API key is required")
	}

	return &LastFMClient{
		baseURL: "http://ws.audioscrobbler.com/2.0",
		apiKey:  config.APIKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

type lastFMArtist struct {
	Name      string `json:"name"`
	MBID      string `json:"mbid"`
	URL       string `json:"url"`
	Listeners string `json:"listeners"`
	Image     []struct {
		Text string `json:"#text"`
		Size string `json:"size"`
	} `json:"image"`
}

type lastFMSearchResponse struct {
	Results struct {
		ArtistMatches struct {
			Artist []lastFMArtist `json:"artist"`
		} `json:"artistmatches"`
		TotalResults string `json:"opensearch:totalResults"`
	} `json:"results"`
}

type lastFMArtistInfoResponse struct {
	Artist struct {
		Name  string `json:"name"`
		MBID  string `json:"mbid"`
		URL   string `json:"url"`
		Image []struct {
			Text string `json:"#text"`
			Size string `json:"size"`
		} `json:"image"`
		Stats struct {
			Listeners string `json:"listeners"`
			Playcount string `json:"playcount"`
		} `json:"stats"`
		Tags struct {
			Tag []struct {
				Name string `json:"name"`
			} `json:"tag"`
		} `json:"tags"`
	} `json:"artist"`
}

func (c *LastFMClient) SearchArtists(ctx context.Context, query string, limit int) ([]domain.Artist, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 30 {
		limit = 30
	}

	searchURL := fmt.Sprintf("%s/?method=artist.search&artist=%s&api_key=%s&format=json&limit=%d",
		c.baseURL,
		url.QueryEscape(query),
		c.apiKey,
		limit,
	)

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create search request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to search artists: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("last.fm search failed: status %d", resp.StatusCode)
	}

	var searchResp lastFMSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode search response: %w", err)
	}

	artists := make([]domain.Artist, 0, len(searchResp.Results.ArtistMatches.Artist))
	for _, lastFMArtist := range searchResp.Results.ArtistMatches.Artist {
		artist := domain.Artist{
			ID:   fmt.Sprintf("lastfm_%s", lastFMArtist.MBID),
			Name: lastFMArtist.Name,
			ExternalIDs: domain.ExternalIDs{
				LastFMID: lastFMArtist.MBID,
			},
		}

		for _, img := range lastFMArtist.Image {
			if img.Size == "extralarge" && img.Text != "" {
				artist.ImageURL = img.Text
				break
			}
		}

		artists = append(artists, artist)
	}

	return artists, nil
}

func (c *LastFMClient) GetArtist(ctx context.Context, artistName string) (*domain.Artist, error) {
	infoURL := fmt.Sprintf("%s/?method=artist.getinfo&artist=%s&api_key=%s&format=json",
		c.baseURL,
		url.QueryEscape(artistName),
		c.apiKey,
	)

	req, err := http.NewRequestWithContext(ctx, "GET", infoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create artist info request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get artist info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, domain.ErrArtistNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("last.fm get artist failed: status %d", resp.StatusCode)
	}

	var infoResp lastFMArtistInfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&infoResp); err != nil {
		return nil, fmt.Errorf("failed to decode artist info response: %w", err)
	}

	artist := &domain.Artist{
		ID:   fmt.Sprintf("lastfm_%s", infoResp.Artist.MBID),
		Name: infoResp.Artist.Name,
		ExternalIDs: domain.ExternalIDs{
			LastFMID: infoResp.Artist.MBID,
		},
	}

	for _, tag := range infoResp.Artist.Tags.Tag {
		artist.Genres = append(artist.Genres, tag.Name)
	}

	for _, img := range infoResp.Artist.Image {
		if img.Size == "extralarge" && img.Text != "" {
			artist.ImageURL = img.Text
			break
		}
	}

	return artist, nil
}
