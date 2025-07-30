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

type mockArtistService struct {
	searchFunc func(ctx context.Context, query string, limit int) (*domain.ArtistSearchResponse, error)
	getFunc    func(ctx context.Context, id string) (*domain.Artist, error)
	saveFunc   func(ctx context.Context, artist *domain.Artist) error
}

func (m *mockArtistService) SearchArtists(ctx context.Context, query string, limit int) (*domain.ArtistSearchResponse, error) {
	if m.searchFunc != nil {
		return m.searchFunc(ctx, query, limit)
	}
	return nil, nil
}

func (m *mockArtistService) GetArtist(ctx context.Context, id string) (*domain.Artist, error) {
	if m.getFunc != nil {
		return m.getFunc(ctx, id)
	}
	return nil, nil
}

func (m *mockArtistService) SaveArtist(ctx context.Context, artist *domain.Artist) error {
	if m.saveFunc != nil {
		return m.saveFunc(ctx, artist)
	}
	return nil
}

func TestArtistHandler_SearchArtists(t *testing.T) {
	t.Run("successful search", func(t *testing.T) {
		mockService := &mockArtistService{
			searchFunc: func(ctx context.Context, query string, limit int) (*domain.ArtistSearchResponse, error) {
				return &domain.ArtistSearchResponse{
					Artists: []domain.Artist{
						{ID: "1", Name: "Test Artist"},
					},
					Total: 1,
				}, nil
			},
		}

		handler := NewArtistHandler(mockService)
		router := mux.NewRouter()
		handler.RegisterRoutes(router)

		req, _ := http.NewRequest("GET", "/api/artists/search?q=test", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
		}

		var response domain.ArtistSearchResponse
		if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
			t.Fatalf("could not unmarshal response: %v", err)
		}

		if len(response.Artists) != 1 {
			t.Errorf("expected 1 artist, got %d", len(response.Artists))
		}
	})

	t.Run("missing query parameter", func(t *testing.T) {
		mockService := &mockArtistService{}
		handler := NewArtistHandler(mockService)
		router := mux.NewRouter()
		handler.RegisterRoutes(router)

		req, _ := http.NewRequest("GET", "/api/artists/search", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusBadRequest {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusBadRequest)
		}
	})

	t.Run("invalid limit parameter", func(t *testing.T) {
		mockService := &mockArtistService{}
		handler := NewArtistHandler(mockService)
		router := mux.NewRouter()
		handler.RegisterRoutes(router)

		req, _ := http.NewRequest("GET", "/api/artists/search?q=test&limit=invalid", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusBadRequest {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusBadRequest)
		}
	})

	t.Run("service error", func(t *testing.T) {
		mockService := &mockArtistService{
			searchFunc: func(ctx context.Context, query string, limit int) (*domain.ArtistSearchResponse, error) {
				return nil, errors.New("service error")
			},
		}

		handler := NewArtistHandler(mockService)
		router := mux.NewRouter()
		handler.RegisterRoutes(router)

		req, _ := http.NewRequest("GET", "/api/artists/search?q=test", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusInternalServerError {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusInternalServerError)
		}
	})
}

func TestArtistHandler_GetArtist(t *testing.T) {
	t.Run("successful get", func(t *testing.T) {
		mockService := &mockArtistService{
			getFunc: func(ctx context.Context, id string) (*domain.Artist, error) {
				return &domain.Artist{
					ID:   id,
					Name: "Test Artist",
				}, nil
			},
		}

		handler := NewArtistHandler(mockService)
		router := mux.NewRouter()
		handler.RegisterRoutes(router)

		req, _ := http.NewRequest("GET", "/api/artists/123", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
		}

		var artist domain.Artist
		if err := json.Unmarshal(rr.Body.Bytes(), &artist); err != nil {
			t.Fatalf("could not unmarshal response: %v", err)
		}

		if artist.ID != "123" {
			t.Errorf("expected ID 123, got %s", artist.ID)
		}
	})

	t.Run("artist not found", func(t *testing.T) {
		mockService := &mockArtistService{
			getFunc: func(ctx context.Context, id string) (*domain.Artist, error) {
				return nil, domain.ErrArtistNotFound
			},
		}

		handler := NewArtistHandler(mockService)
		router := mux.NewRouter()
		handler.RegisterRoutes(router)

		req, _ := http.NewRequest("GET", "/api/artists/not-found", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusNotFound {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusNotFound)
		}
	})
}

func TestArtistHandler_SaveArtist(t *testing.T) {
	t.Run("successful save", func(t *testing.T) {
		mockService := &mockArtistService{
			saveFunc: func(ctx context.Context, artist *domain.Artist) error {
				return nil
			},
		}

		handler := NewArtistHandler(mockService)
		router := mux.NewRouter()
		handler.RegisterRoutes(router)

		artist := domain.Artist{
			Name: "New Artist",
		}
		body, _ := json.Marshal(artist)

		req, _ := http.NewRequest("POST", "/api/artists", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusCreated {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusCreated)
		}
	})

	t.Run("invalid request body", func(t *testing.T) {
		mockService := &mockArtistService{}
		handler := NewArtistHandler(mockService)
		router := mux.NewRouter()
		handler.RegisterRoutes(router)

		req, _ := http.NewRequest("POST", "/api/artists", bytes.NewBuffer([]byte("invalid json")))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusBadRequest {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusBadRequest)
		}
	})

	t.Run("missing artist name", func(t *testing.T) {
		mockService := &mockArtistService{}
		handler := NewArtistHandler(mockService)
		router := mux.NewRouter()
		handler.RegisterRoutes(router)

		artist := domain.Artist{}
		body, _ := json.Marshal(artist)

		req, _ := http.NewRequest("POST", "/api/artists", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusBadRequest {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusBadRequest)
		}
	})

	t.Run("duplicate artist", func(t *testing.T) {
		mockService := &mockArtistService{
			saveFunc: func(ctx context.Context, artist *domain.Artist) error {
				return domain.ErrDuplicateArtist
			},
		}

		handler := NewArtistHandler(mockService)
		router := mux.NewRouter()
		handler.RegisterRoutes(router)

		artist := domain.Artist{
			Name: "Existing Artist",
		}
		body, _ := json.Marshal(artist)

		req, _ := http.NewRequest("POST", "/api/artists", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusConflict {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusConflict)
		}
	})
}
