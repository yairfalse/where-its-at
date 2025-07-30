package integrations

import (
	"context"
	"fmt"
	"sync"

	"github.com/yair/where-its-at/pkg/domain"
)

type ArtistAggregator struct {
	spotify *SpotifyClient
	lastfm  *LastFMClient
}

func NewArtistAggregator(spotify *SpotifyClient, lastfm *LastFMClient) *ArtistAggregator {
	return &ArtistAggregator{
		spotify: spotify,
		lastfm:  lastfm,
	}
}

func (a *ArtistAggregator) SearchArtists(ctx context.Context, query string, limit int) ([]domain.Artist, error) {
	var (
		spotifyArtists []domain.Artist
		lastfmArtists  []domain.Artist
		spotifyErr     error
		lastfmErr      error
		wg             sync.WaitGroup
	)

	wg.Add(2)

	go func() {
		defer wg.Done()
		if a.spotify != nil {
			spotifyArtists, spotifyErr = a.spotify.SearchArtists(ctx, query, limit)
		}
	}()

	go func() {
		defer wg.Done()
		if a.lastfm != nil {
			lastfmArtists, lastfmErr = a.lastfm.SearchArtists(ctx, query, limit)
		}
	}()

	wg.Wait()

	if spotifyErr != nil && lastfmErr != nil {
		return nil, fmt.Errorf("all external APIs failed: spotify=%w, lastfm=%w", spotifyErr, lastfmErr)
	}

	artistMap := make(map[string]domain.Artist)

	for _, artist := range spotifyArtists {
		artistMap[artist.Name] = artist
	}

	for _, artist := range lastfmArtists {
		if existing, ok := artistMap[artist.Name]; ok {
			if existing.ExternalIDs.LastFMID == "" && artist.ExternalIDs.LastFMID != "" {
				existing.ExternalIDs.LastFMID = artist.ExternalIDs.LastFMID
			}
			if len(existing.Genres) == 0 && len(artist.Genres) > 0 {
				existing.Genres = artist.Genres
			}
			artistMap[artist.Name] = existing
		} else {
			artistMap[artist.Name] = artist
		}
	}

	result := make([]domain.Artist, 0, len(artistMap))
	for _, artist := range artistMap {
		result = append(result, artist)
	}

	return result, nil
}
