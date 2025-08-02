package interfaces

import (
	"context"
	"testing"

	"github.com/yair/where-its-at/pkg/domain"
	"github.com/yair/where-its-at/pkg/integrations"
)

func TestArtistService_ExtraScenarios(t *testing.T) {
	t.Run("search returns error from repository", func(t *testing.T) {
		mockRepo := &mockRepository{
			searchFunc: func(ctx context.Context, query string, limit int) ([]domain.Artist, error) {
				return nil, domain.ErrArtistNotFound
			},
		}

		service := NewArtistService(mockRepo, nil)
		_, err := service.SearchArtists(context.Background(), "test", 10)
		if err == nil {
			t.Error("expected error from repository")
		}
	})

	t.Run("search with external API when local returns few results", func(t *testing.T) {
		mockRepo := &mockRepository{
			searchFunc: func(ctx context.Context, query string, limit int) ([]domain.Artist, error) {
				// Return less than requested limit
				return []domain.Artist{
					{ID: "1", Name: "Local Artist"},
				}, nil
			},
		}

		// Create aggregator that returns nil (simulating API failure)
		aggregator := integrations.NewArtistAggregator(nil, nil)
		service := NewArtistService(mockRepo, aggregator)

		response, err := service.SearchArtists(context.Background(), "test", 5)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		// Should still return the local results
		if len(response.Artists) != 1 {
			t.Errorf("expected 1 artist from local, got %d", len(response.Artists))
		}
	})
}
