package integrations

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/yair/where-its-at/pkg/domain"
)

type SpotifyClient struct {
	baseURL      string
	clientID     string
	clientSecret string
	httpClient   *http.Client
	accessToken  string
	tokenExpiry  time.Time
}

type SpotifyConfig struct {
	ClientID     string
	ClientSecret string
}

func NewSpotifyClient(config SpotifyConfig) (*SpotifyClient, error) {
	if config.ClientID == "" || config.ClientSecret == "" {
		return nil, fmt.Errorf("spotify client ID and secret are required")
	}

	return &SpotifyClient{
		baseURL:      "https://api.spotify.com/v1",
		clientID:     config.ClientID,
		clientSecret: config.ClientSecret,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

type spotifyTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

func (c *SpotifyClient) getAccessToken(ctx context.Context) error {
	if c.accessToken != "" && time.Now().Before(c.tokenExpiry) {
		return nil
	}

	tokenURL := "https://accounts.spotify.com/api/token"
	data := url.Values{}
	data.Set("grant_type", "client_credentials")

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create token request: %w", err)
	}

	req.SetBasicAuth(c.clientID, c.clientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to get access token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to get access token: status %d", resp.StatusCode)
	}

	var tokenResp spotifyTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return fmt.Errorf("failed to decode token response: %w", err)
	}

	c.accessToken = tokenResp.AccessToken
	c.tokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second).Add(-5 * time.Minute)

	return nil
}

type spotifyArtist struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Genres     []string `json:"genres"`
	Popularity int      `json:"popularity"`
	Images     []struct {
		URL string `json:"url"`
	} `json:"images"`
}

type spotifySearchResponse struct {
	Artists struct {
		Items []spotifyArtist `json:"items"`
		Total int             `json:"total"`
	} `json:"artists"`
}

func (c *SpotifyClient) SearchArtists(ctx context.Context, query string, limit int) ([]domain.Artist, error) {
	if err := c.getAccessToken(ctx); err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	searchURL := fmt.Sprintf("%s/search?q=%s&type=artist&limit=%d",
		c.baseURL,
		url.QueryEscape(query),
		limit,
	)

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create search request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to search artists: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("spotify search failed: status %d", resp.StatusCode)
	}

	var searchResp spotifySearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode search response: %w", err)
	}

	artists := make([]domain.Artist, 0, len(searchResp.Artists.Items))
	for _, spotifyArtist := range searchResp.Artists.Items {
		artist := domain.Artist{
			ID:   fmt.Sprintf("spotify_%s", spotifyArtist.ID),
			Name: spotifyArtist.Name,
			ExternalIDs: domain.ExternalIDs{
				SpotifyID: spotifyArtist.ID,
			},
			Genres:     spotifyArtist.Genres,
			Popularity: spotifyArtist.Popularity,
		}

		if len(spotifyArtist.Images) > 0 {
			artist.ImageURL = spotifyArtist.Images[0].URL
		}

		artists = append(artists, artist)
	}

	return artists, nil
}

func (c *SpotifyClient) GetArtist(ctx context.Context, spotifyID string) (*domain.Artist, error) {
	if err := c.getAccessToken(ctx); err != nil {
		return nil, err
	}

	artistURL := fmt.Sprintf("%s/artists/%s", c.baseURL, spotifyID)

	req, err := http.NewRequestWithContext(ctx, "GET", artistURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create artist request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get artist: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, domain.ErrArtistNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("spotify get artist failed: status %d", resp.StatusCode)
	}

	var spotifyArtist spotifyArtist
	if err := json.NewDecoder(resp.Body).Decode(&spotifyArtist); err != nil {
		return nil, fmt.Errorf("failed to decode artist response: %w", err)
	}

	artist := &domain.Artist{
		ID:   fmt.Sprintf("spotify_%s", spotifyArtist.ID),
		Name: spotifyArtist.Name,
		ExternalIDs: domain.ExternalIDs{
			SpotifyID: spotifyArtist.ID,
		},
		Genres:     spotifyArtist.Genres,
		Popularity: spotifyArtist.Popularity,
	}

	if len(spotifyArtist.Images) > 0 {
		artist.ImageURL = spotifyArtist.Images[0].URL
	}

	return artist, nil
}
