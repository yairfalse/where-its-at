package interfaces

import (
	"context"
	"fmt"
	"time"

	"github.com/yair/where-its-at/pkg/domain"
	"github.com/yair/where-its-at/pkg/integrations"
)

type ArtistService struct {
	repository domain.ArtistRepository
	aggregator *integrations.ArtistAggregator
}

func NewArtistService(repository domain.ArtistRepository, aggregator *integrations.ArtistAggregator) *ArtistService {
	return &ArtistService{
		repository: repository,
		aggregator: aggregator,
	}
}

func (s *ArtistService) SearchArtists(ctx context.Context, query string, limit int) (*domain.ArtistSearchResponse, error) {
	if query == "" {
		return nil, domain.ErrInvalidRequest
	}

	localArtists, err := s.repository.Search(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search local artists: %w", err)
	}

	if len(localArtists) >= limit {
		return &domain.ArtistSearchResponse{
			Artists: localArtists,
			Total:   len(localArtists),
		}, nil
	}

	if s.aggregator != nil {
		externalArtists, err := s.aggregator.SearchArtists(ctx, query, limit)
		if err != nil {
			if len(localArtists) > 0 {
				return &domain.ArtistSearchResponse{
					Artists: localArtists,
					Total:   len(localArtists),
				}, nil
			}
			return nil, domain.ErrExternalAPIFailure
		}

		artistMap := make(map[string]domain.Artist)
		for _, artist := range localArtists {
			artistMap[artist.Name] = artist
		}

		for _, artist := range externalArtists {
			if _, exists := artistMap[artist.Name]; !exists {
				artistMap[artist.Name] = artist
			}
		}

		combinedArtists := make([]domain.Artist, 0, len(artistMap))
		for _, artist := range artistMap {
			combinedArtists = append(combinedArtists, artist)
		}

		if len(combinedArtists) > limit {
			combinedArtists = combinedArtists[:limit]
		}

		return &domain.ArtistSearchResponse{
			Artists: combinedArtists,
			Total:   len(combinedArtists),
		}, nil
	}

	return &domain.ArtistSearchResponse{
		Artists: localArtists,
		Total:   len(localArtists),
	}, nil
}

func (s *ArtistService) GetArtist(ctx context.Context, id string) (*domain.Artist, error) {
	if id == "" {
		return nil, domain.ErrInvalidRequest
	}

	artist, err := s.repository.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return artist, nil
}

func (s *ArtistService) SaveArtist(ctx context.Context, artist *domain.Artist) error {
	if artist == nil || artist.Name == "" {
		return domain.ErrInvalidRequest
	}

	if artist.ID == "" {
		artist.ID = fmt.Sprintf("local_%d", time.Now().UnixNano())
	}

	return s.repository.Create(ctx, artist)
}
