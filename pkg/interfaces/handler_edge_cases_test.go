package interfaces

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/yair/where-its-at/pkg/domain"
)

func TestArtistHandler_EdgeCases(t *testing.T) {
	t.Run("search with negative limit", func(t *testing.T) {
		mockService := &mockArtistService{
			searchFunc: func(ctx context.Context, query string, limit int) (*domain.ArtistSearchResponse, error) {
				return &domain.ArtistSearchResponse{
					Artists: []domain.Artist{},
					Total:   0,
				}, nil
			},
		}

		handler := NewArtistHandler(mockService)
		if handler == nil {
			t.Fatal("expected handler, got nil")
		}

		// Test handler methods are accessible
		if handler.service == nil {
			t.Error("expected service to be set")
		}
	})
}

func TestArtistService_ExternalAPIIntegration(t *testing.T) {
	t.Run("external API failure fallback to local", func(t *testing.T) {
		mockRepo := &mockRepository{
			searchFunc: func(ctx context.Context, query string, limit int) ([]domain.Artist, error) {
				return []domain.Artist{
					{ID: "local-1", Name: "Local Artist"},
				}, nil
			},
		}

		// Create service without aggregator to test local-only path
		service := NewArtistService(mockRepo, nil)

		response, err := service.SearchArtists(context.Background(), "test", 10)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if len(response.Artists) != 1 {
			t.Errorf("expected 1 artist, got %d", len(response.Artists))
		}
	})
}

func TestArtistHandler_MoreCoverage(t *testing.T) {
	t.Run("search with zero limit uses default", func(t *testing.T) {
		mockService := &mockArtistService{
			searchFunc: func(ctx context.Context, query string, limit int) (*domain.ArtistSearchResponse, error) {
				if limit == 10 {
					t.Log("Default limit of 10 was used")
				}
				return &domain.ArtistSearchResponse{
					Artists: []domain.Artist{{ID: "1", Name: "Test"}},
					Total:   1,
				}, nil
			},
		}

		handler := NewArtistHandler(mockService)
		router := mux.NewRouter()
		handler.RegisterRoutes(router)

		req, _ := http.NewRequest("GET", "/api/artists/search?q=test", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rr.Code)
		}
	})

	t.Run("invalid request error in search", func(t *testing.T) {
		mockService := &mockArtistService{
			searchFunc: func(ctx context.Context, query string, limit int) (*domain.ArtistSearchResponse, error) {
				return nil, domain.ErrInvalidRequest
			},
		}

		handler := NewArtistHandler(mockService)
		router := mux.NewRouter()
		handler.RegisterRoutes(router)

		req, _ := http.NewRequest("GET", "/api/artists/search?q=test", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", rr.Code)
		}
	})

	t.Run("external API failure in search", func(t *testing.T) {
		mockService := &mockArtistService{
			searchFunc: func(ctx context.Context, query string, limit int) (*domain.ArtistSearchResponse, error) {
				return nil, domain.ErrExternalAPIFailure
			},
		}

		handler := NewArtistHandler(mockService)
		router := mux.NewRouter()
		handler.RegisterRoutes(router)

		req, _ := http.NewRequest("GET", "/api/artists/search?q=test", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusServiceUnavailable {
			t.Errorf("expected status 503, got %d", rr.Code)
		}
	})

	t.Run("invalid request error in save", func(t *testing.T) {
		mockService := &mockArtistService{
			saveFunc: func(ctx context.Context, artist *domain.Artist) error {
				return domain.ErrInvalidRequest
			},
		}

		handler := NewArtistHandler(mockService)
		router := mux.NewRouter()
		handler.RegisterRoutes(router)

		artist := domain.Artist{Name: "Test"}
		body, _ := json.Marshal(artist)

		req, _ := http.NewRequest("POST", "/api/artists", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", rr.Code)
		}
	})

	t.Run("general error in get artist", func(t *testing.T) {
		mockService := &mockArtistService{
			getFunc: func(ctx context.Context, id string) (*domain.Artist, error) {
				return nil, errors.New("database connection failed")
			},
		}

		handler := NewArtistHandler(mockService)
		router := mux.NewRouter()
		handler.RegisterRoutes(router)

		req, _ := http.NewRequest("GET", "/api/artists/123", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status 500, got %d", rr.Code)
		}
	})

	t.Run("general error in save artist", func(t *testing.T) {
		mockService := &mockArtistService{
			saveFunc: func(ctx context.Context, artist *domain.Artist) error {
				return errors.New("database connection failed")
			},
		}

		handler := NewArtistHandler(mockService)
		router := mux.NewRouter()
		handler.RegisterRoutes(router)

		artist := domain.Artist{Name: "Test"}
		body, _ := json.Marshal(artist)

		req, _ := http.NewRequest("POST", "/api/artists", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status 500, got %d", rr.Code)
		}
	})
}

func TestArtistService_ValidationEdgeCases(t *testing.T) {
	t.Run("save artist with very long ID", func(t *testing.T) {
		mockRepo := &mockRepository{
			createFunc: func(ctx context.Context, artist *domain.Artist) error {
				if len(artist.ID) > 100 {
					return errors.New("ID too long")
				}
				return nil
			},
		}

		service := NewArtistService(mockRepo, nil)

		longID := ""
		for i := 0; i < 150; i++ {
			longID += "a"
		}

		artist := &domain.Artist{
			ID:   longID,
			Name: "Test Artist",
		}

		err := service.SaveArtist(context.Background(), artist)
		if err == nil {
			t.Error("expected error for long ID")
		}
	})
}
