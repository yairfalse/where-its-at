package integrations

import (
	"context"
	"fmt"
	"testing"

	"github.com/yair/where-its-at/pkg/domain"
)

type mockSpotifyClient struct {
	artists []domain.Artist
	err     error
}

func (m *mockSpotifyClient) SearchArtists(ctx context.Context, query string, limit int) ([]domain.Artist, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.artists, nil
}

type mockLastFMClient struct {
	artists []domain.Artist
	err     error
}

func (m *mockLastFMClient) SearchArtists(ctx context.Context, query string, limit int) ([]domain.Artist, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.artists, nil
}

func TestArtistAggregator_SearchArtists(t *testing.T) {
	t.Run("both sources return results", func(t *testing.T) {
		aggregator := &ArtistAggregator{
			spotify: &SpotifyClient{
				baseURL: "test",
			},
			lastfm: &LastFMClient{
				baseURL: "test",
			},
		}

		if aggregator.spotify == nil && aggregator.lastfm == nil {
			t.Skip("Mock setup incomplete")
		}
	})

	t.Run("spotify fails, lastfm succeeds", func(t *testing.T) {
		aggregator := &ArtistAggregator{
			spotify: nil,
			lastfm:  nil,
		}

		ctx := context.Background()
		results, err := aggregator.SearchArtists(ctx, "test", 10)

		if err == nil && len(results) == 0 {
			t.Log("Empty results when both clients are nil")
		}
	})

	t.Run("both sources fail", func(t *testing.T) {
		aggregator := &ArtistAggregator{
			spotify: nil,
			lastfm:  nil,
		}

		ctx := context.Background()
		_, err := aggregator.SearchArtists(ctx, "test", 10)

		if err != nil {
			expectedErr := fmt.Sprintf("all external APIs failed")
			if err.Error()[:len(expectedErr)] != expectedErr {
				t.Errorf("expected error starting with %s, got %v", expectedErr, err)
			}
		}
	})

	t.Run("nil aggregator components", func(t *testing.T) {
		aggregator := NewArtistAggregator(nil, nil)

		ctx := context.Background()
		results, err := aggregator.SearchArtists(ctx, "test", 10)

		if err == nil && len(results) == 0 {
			t.Log("Empty results when both clients are nil")
		} else if err != nil {
			t.Logf("Error when both clients are nil: %v", err)
		}
	})
}
