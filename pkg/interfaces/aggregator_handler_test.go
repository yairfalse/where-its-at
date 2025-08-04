package interfaces

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/yair/where-its-at/pkg/domain"
	"github.com/yair/where-its-at/pkg/integrations"
)

type mockMegaAggregator struct {
	searchArtistsFunc          func(ctx context.Context, query string, limit int) (*integrations.AggregatedResults, error)
	searchEventsFunc           func(ctx context.Context, artistName string, limit int) (*integrations.AggregatedResults, error)
	searchEventsByLocationFunc func(ctx context.Context, city, country string, limit int) (*integrations.AggregatedResults, error)
	getSourceStatsFunc         func() map[string]integrations.SourceInfo
}

func (m *mockMegaAggregator) SearchArtists(ctx context.Context, query string, limit int) (*integrations.AggregatedResults, error) {
	if m.searchArtistsFunc != nil {
		return m.searchArtistsFunc(ctx, query, limit)
	}
	return &integrations.AggregatedResults{}, nil
}

func (m *mockMegaAggregator) SearchEvents(ctx context.Context, artistName string, limit int) (*integrations.AggregatedResults, error) {
	if m.searchEventsFunc != nil {
		return m.searchEventsFunc(ctx, artistName, limit)
	}
	return &integrations.AggregatedResults{}, nil
}

func (m *mockMegaAggregator) SearchEventsByLocation(ctx context.Context, city, country string, limit int) (*integrations.AggregatedResults, error) {
	if m.searchEventsByLocationFunc != nil {
		return m.searchEventsByLocationFunc(ctx, city, country, limit)
	}
	return &integrations.AggregatedResults{}, nil
}

func (m *mockMegaAggregator) GetSourceStats() map[string]integrations.SourceInfo {
	if m.getSourceStatsFunc != nil {
		return m.getSourceStatsFunc()
	}
	return map[string]integrations.SourceInfo{}
}

func TestAggregatorHandler_SearchArtists(t *testing.T) {
	t.Run("successful artist search", func(t *testing.T) {
		mock := &mockMegaAggregator{
			searchArtistsFunc: func(ctx context.Context, query string, limit int) (*integrations.AggregatedResults, error) {
				return &integrations.AggregatedResults{
					Artists: []domain.Artist{
						{ID: "1", Name: "Test Artist 1"},
						{ID: "2", Name: "Test Artist 2"},
					},
					TotalResults: 2,
					SearchTime:   100 * time.Millisecond,
					SourceStats: map[string]int{
						"spotify":     1,
						"apple_music": 1,
					},
				}, nil
			},
		}

		handler := NewAggregatorHandler(mock)
		router := mux.NewRouter()
		handler.RegisterRoutes(router)

		req, _ := http.NewRequest("GET", "/api/search/artists?q=test&limit=10", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rr.Code)
		}

		var response integrations.AggregatedResults
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if len(response.Artists) != 2 {
			t.Errorf("expected 2 artists, got %d", len(response.Artists))
		}
	})

	t.Run("missing query parameter", func(t *testing.T) {
		handler := NewAggregatorHandler(&mockMegaAggregator{})
		router := mux.NewRouter()
		handler.RegisterRoutes(router)

		req, _ := http.NewRequest("GET", "/api/search/artists", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", rr.Code)
		}

		var response ErrorResponse
		json.NewDecoder(rr.Body).Decode(&response)
		if response.Error != "query parameter 'q' is required" {
			t.Errorf("expected specific error message, got %s", response.Error)
		}
	})

	t.Run("aggregator error", func(t *testing.T) {
		mock := &mockMegaAggregator{
			searchArtistsFunc: func(ctx context.Context, query string, limit int) (*integrations.AggregatedResults, error) {
				return nil, errors.New("aggregation failed")
			},
		}

		handler := NewAggregatorHandler(mock)
		router := mux.NewRouter()
		handler.RegisterRoutes(router)

		req, _ := http.NewRequest("GET", "/api/search/artists?q=test", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status 500, got %d", rr.Code)
		}
	})

	t.Run("default limit when not specified", func(t *testing.T) {
		var capturedLimit int
		mock := &mockMegaAggregator{
			searchArtistsFunc: func(ctx context.Context, query string, limit int) (*integrations.AggregatedResults, error) {
				capturedLimit = limit
				return &integrations.AggregatedResults{}, nil
			},
		}

		handler := NewAggregatorHandler(mock)
		router := mux.NewRouter()
		handler.RegisterRoutes(router)

		req, _ := http.NewRequest("GET", "/api/search/artists?q=test", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if capturedLimit != 50 {
			t.Errorf("expected default limit 50, got %d", capturedLimit)
		}
	})

	t.Run("limit capped at 200", func(t *testing.T) {
		var capturedLimit int
		mock := &mockMegaAggregator{
			searchArtistsFunc: func(ctx context.Context, query string, limit int) (*integrations.AggregatedResults, error) {
				capturedLimit = limit
				return &integrations.AggregatedResults{}, nil
			},
		}

		handler := NewAggregatorHandler(mock)
		router := mux.NewRouter()
		handler.RegisterRoutes(router)

		req, _ := http.NewRequest("GET", "/api/search/artists?q=test&limit=500", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if capturedLimit != 200 {
			t.Errorf("expected limit capped at 200, got %d", capturedLimit)
		}
	})
}

func TestAggregatorHandler_SearchEvents(t *testing.T) {
	t.Run("successful event search", func(t *testing.T) {
		mock := &mockMegaAggregator{
			searchEventsFunc: func(ctx context.Context, artistName string, limit int) (*integrations.AggregatedResults, error) {
				return &integrations.AggregatedResults{
					Events: []domain.Event{
						{
							ID:         "1",
							ArtistName: artistName,
							DateTime:   time.Now().Add(7 * 24 * time.Hour),
							Venue: domain.Venue{
								Name: "Test Venue",
								City: "Berlin",
							},
						},
					},
					TotalResults: 1,
					SourceStats: map[string]int{
						"bandsintown": 1,
					},
				}, nil
			},
		}

		handler := NewAggregatorHandler(mock)
		router := mux.NewRouter()
		handler.RegisterRoutes(router)

		req, _ := http.NewRequest("GET", "/api/search/events?artist=Test+Artist", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rr.Code)
		}

		var response integrations.AggregatedResults
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if len(response.Events) != 1 {
			t.Errorf("expected 1 event, got %d", len(response.Events))
		}
	})

	t.Run("missing artist parameter", func(t *testing.T) {
		handler := NewAggregatorHandler(&mockMegaAggregator{})
		router := mux.NewRouter()
		handler.RegisterRoutes(router)

		req, _ := http.NewRequest("GET", "/api/search/events", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", rr.Code)
		}

		var response ErrorResponse
		json.NewDecoder(rr.Body).Decode(&response)
		if response.Error != "query parameter 'artist' is required" {
			t.Errorf("expected specific error message, got %s", response.Error)
		}
	})

	t.Run("aggregator error", func(t *testing.T) {
		mock := &mockMegaAggregator{
			searchEventsFunc: func(ctx context.Context, artistName string, limit int) (*integrations.AggregatedResults, error) {
				return nil, errors.New("aggregation failed")
			},
		}

		handler := NewAggregatorHandler(mock)
		router := mux.NewRouter()
		handler.RegisterRoutes(router)

		req, _ := http.NewRequest("GET", "/api/search/events?artist=test", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status 500, got %d", rr.Code)
		}
	})

	t.Run("with limit parameter", func(t *testing.T) {
		var capturedLimit int
		mock := &mockMegaAggregator{
			searchEventsFunc: func(ctx context.Context, artistName string, limit int) (*integrations.AggregatedResults, error) {
				capturedLimit = limit
				return &integrations.AggregatedResults{}, nil
			},
		}

		handler := NewAggregatorHandler(mock)
		router := mux.NewRouter()
		handler.RegisterRoutes(router)

		req, _ := http.NewRequest("GET", "/api/search/events?artist=test&limit=25", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if capturedLimit != 25 {
			t.Errorf("expected limit 25, got %d", capturedLimit)
		}
	})
}

func TestAggregatorHandler_SearchEventsByLocation(t *testing.T) {
	t.Run("successful location search", func(t *testing.T) {
		mock := &mockMegaAggregator{
			searchEventsByLocationFunc: func(ctx context.Context, city, country string, limit int) (*integrations.AggregatedResults, error) {
				return &integrations.AggregatedResults{
					Events: []domain.Event{
						{
							ID:         "1",
							ArtistName: "Local Artist",
							DateTime:   time.Now().Add(24 * time.Hour),
							Venue: domain.Venue{
								Name:    "Local Venue",
								City:    city,
								Country: country,
							},
						},
					},
					TotalResults: 1,
				}, nil
			},
		}

		handler := NewAggregatorHandler(mock)
		router := mux.NewRouter()
		handler.RegisterRoutes(router)

		req, _ := http.NewRequest("GET", "/api/search/events/location?city=Berlin&country=DE", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rr.Code)
		}

		var response integrations.AggregatedResults
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if len(response.Events) != 1 {
			t.Errorf("expected 1 event, got %d", len(response.Events))
		}
		if response.Events[0].Venue.City != "Berlin" {
			t.Errorf("expected city Berlin, got %s", response.Events[0].Venue.City)
		}
	})

	t.Run("missing city parameter", func(t *testing.T) {
		handler := NewAggregatorHandler(&mockMegaAggregator{})
		router := mux.NewRouter()
		handler.RegisterRoutes(router)

		req, _ := http.NewRequest("GET", "/api/search/events/location?country=DE", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", rr.Code)
		}

		var response ErrorResponse
		json.NewDecoder(rr.Body).Decode(&response)
		if response.Error != "query parameter 'city' is required" {
			t.Errorf("expected specific error message, got %s", response.Error)
		}
	})

	t.Run("city only without country", func(t *testing.T) {
		var capturedCity, capturedCountry string
		mock := &mockMegaAggregator{
			searchEventsByLocationFunc: func(ctx context.Context, city, country string, limit int) (*integrations.AggregatedResults, error) {
				capturedCity = city
				capturedCountry = country
				return &integrations.AggregatedResults{}, nil
			},
		}

		handler := NewAggregatorHandler(mock)
		router := mux.NewRouter()
		handler.RegisterRoutes(router)

		req, _ := http.NewRequest("GET", "/api/search/events/location?city=Berlin", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if capturedCity != "Berlin" {
			t.Errorf("expected city Berlin, got %s", capturedCity)
		}
		if capturedCountry != "" {
			t.Errorf("expected empty country, got %s", capturedCountry)
		}
	})

	t.Run("aggregator error", func(t *testing.T) {
		mock := &mockMegaAggregator{
			searchEventsByLocationFunc: func(ctx context.Context, city, country string, limit int) (*integrations.AggregatedResults, error) {
				return nil, errors.New("location search failed")
			},
		}

		handler := NewAggregatorHandler(mock)
		router := mux.NewRouter()
		handler.RegisterRoutes(router)

		req, _ := http.NewRequest("GET", "/api/search/events/location?city=Berlin", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status 500, got %d", rr.Code)
		}
	})

	t.Run("with limit parameter", func(t *testing.T) {
		var capturedLimit int
		mock := &mockMegaAggregator{
			searchEventsByLocationFunc: func(ctx context.Context, city, country string, limit int) (*integrations.AggregatedResults, error) {
				capturedLimit = limit
				return &integrations.AggregatedResults{}, nil
			},
		}

		handler := NewAggregatorHandler(mock)
		router := mux.NewRouter()
		handler.RegisterRoutes(router)

		req, _ := http.NewRequest("GET", "/api/search/events/location?city=Berlin&limit=75", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if capturedLimit != 75 {
			t.Errorf("expected limit 75, got %d", capturedLimit)
		}
	})
}

func TestAggregatorHandler_GetSources(t *testing.T) {
	t.Run("successful get sources", func(t *testing.T) {
		mock := &mockMegaAggregator{
			getSourceStatsFunc: func() map[string]integrations.SourceInfo {
				return map[string]integrations.SourceInfo{
					"spotify":          {Type: "music", Status: "active"},
					"apple_music":      {Type: "music", Status: "active"},
					"bandsintown":      {Type: "events", Status: "active"},
					"resident_advisor": {Type: "scraper", Status: "active"},
				}
			},
		}

		handler := NewAggregatorHandler(mock)
		router := mux.NewRouter()
		handler.RegisterRoutes(router)

		req, _ := http.NewRequest("GET", "/api/sources", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rr.Code)
		}

		var response SourcesResponse
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response.Total != 4 {
			t.Errorf("expected 4 sources, got %d", response.Total)
		}

		if response.Sources["spotify"].Type != "music" {
			t.Errorf("expected spotify to be music type, got %s", response.Sources["spotify"].Type)
		}
	})
}

func TestAggregatorHandler_ErrorResponses(t *testing.T) {
	t.Run("writeErrorResponse formats correctly", func(t *testing.T) {
		handler := NewAggregatorHandler(nil)
		rr := httptest.NewRecorder()

		handler.writeErrorResponse(rr, http.StatusNotFound, "not found")

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", rr.Code)
		}

		var response ErrorResponse
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response.Error != "not found" {
			t.Errorf("expected error 'not found', got %s", response.Error)
		}
		if response.Status != http.StatusNotFound {
			t.Errorf("expected status 404 in response, got %d", response.Status)
		}
	})
}

func TestAggregatorHandler_InvalidLimitValues(t *testing.T) {
	tests := []struct {
		name          string
		limitParam    string
		expectedLimit int
	}{
		{"negative limit uses default", "-10", 50},
		{"zero limit uses default", "0", 50},
		{"invalid format uses default", "abc", 50},
		{"valid limit within range", "75", 75},
		{"huge limit capped at 200", "1000", 200},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedLimit int
			mock := &mockMegaAggregator{
				searchArtistsFunc: func(ctx context.Context, query string, limit int) (*integrations.AggregatedResults, error) {
					capturedLimit = limit
					return &integrations.AggregatedResults{}, nil
				},
			}

			handler := NewAggregatorHandler(mock)
			router := mux.NewRouter()
			handler.RegisterRoutes(router)

			req, _ := http.NewRequest("GET", "/api/search/artists?q=test&limit="+tt.limitParam, nil)
			rr := httptest.NewRecorder()

			router.ServeHTTP(rr, req)

			if capturedLimit != tt.expectedLimit {
				t.Errorf("expected limit %d, got %d", tt.expectedLimit, capturedLimit)
			}
		})
	}
}

func TestAggregatorHandler_ContentType(t *testing.T) {
	mock := &mockMegaAggregator{
		searchArtistsFunc: func(ctx context.Context, query string, limit int) (*integrations.AggregatedResults, error) {
			return &integrations.AggregatedResults{}, nil
		},
	}

	handler := NewAggregatorHandler(mock)
	router := mux.NewRouter()
	handler.RegisterRoutes(router)

	req, _ := http.NewRequest("GET", "/api/search/artists?q=test", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	contentType := rr.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", contentType)
	}
}

func TestAggregatorHandler_FormatLocation(t *testing.T) {
	handler := &AggregatorHandler{}

	tests := []struct {
		name     string
		city     string
		country  string
		expected string
	}{
		{"city and country", "Berlin", "DE", "Berlin, DE"},
		{"city only", "London", "", "London"},
		{"country only", "", "UK", "UK"},
		{"neither", "", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.formatLocation(tt.city, tt.country)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestAggregatorHandler_JSONEncodingError(t *testing.T) {
	t.Run("writeJSONResponse with invalid data", func(t *testing.T) {
		handler := NewAggregatorHandler(nil)
		rr := httptest.NewRecorder()

		// Create a value that cannot be marshalled to JSON
		invalidData := make(chan int)

		handler.writeJSONResponse(rr, http.StatusOK, invalidData)

		// Should get internal server error due to encoding failure
		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status 500, got %d", rr.Code)
		}
		if !strings.Contains(rr.Body.String(), "failed to encode response") {
			t.Errorf("expected encoding error message, got %s", rr.Body.String())
		}
	})
}

func TestAggregatorHandler_CompleteFlowTests(t *testing.T) {
	t.Run("search artists with very small limit", func(t *testing.T) {
		mock := &mockMegaAggregator{
			searchArtistsFunc: func(ctx context.Context, query string, limit int) (*integrations.AggregatedResults, error) {
				if limit != 1 {
					t.Errorf("expected limit 1, got %d", limit)
				}
				return &integrations.AggregatedResults{
					Artists:      []domain.Artist{{ID: "1", Name: "Solo Artist"}},
					TotalResults: 1,
				}, nil
			},
		}

		handler := NewAggregatorHandler(mock)
		router := mux.NewRouter()
		handler.RegisterRoutes(router)

		req, _ := http.NewRequest("GET", "/api/search/artists?q=test&limit=1", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rr.Code)
		}
	})

	t.Run("search events with special characters in artist name", func(t *testing.T) {
		expectedArtist := "Test & Artist (feat. Someone)"
		mock := &mockMegaAggregator{
			searchEventsFunc: func(ctx context.Context, artistName string, limit int) (*integrations.AggregatedResults, error) {
				if artistName != expectedArtist {
					t.Errorf("expected artist '%s', got '%s'", expectedArtist, artistName)
				}
				return &integrations.AggregatedResults{}, nil
			},
		}

		handler := NewAggregatorHandler(mock)
		router := mux.NewRouter()
		handler.RegisterRoutes(router)

		req, _ := http.NewRequest("GET", "/api/search/events?artist=Test+%26+Artist+(feat.+Someone)", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rr.Code)
		}
	})

	t.Run("search events by location with special characters", func(t *testing.T) {
		expectedCity := "São Paulo"
		expectedCountry := "BR"
		mock := &mockMegaAggregator{
			searchEventsByLocationFunc: func(ctx context.Context, city, country string, limit int) (*integrations.AggregatedResults, error) {
				if city != expectedCity {
					t.Errorf("expected city '%s', got '%s'", expectedCity, city)
				}
				if country != expectedCountry {
					t.Errorf("expected country '%s', got '%s'", expectedCountry, country)
				}
				return &integrations.AggregatedResults{}, nil
			},
		}

		handler := NewAggregatorHandler(mock)
		router := mux.NewRouter()
		handler.RegisterRoutes(router)

		req, _ := http.NewRequest("GET", "/api/search/events/location?city=S%C3%A3o+Paulo&country=BR", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rr.Code)
		}
	})

	t.Run("get sources returns empty map", func(t *testing.T) {
		mock := &mockMegaAggregator{
			getSourceStatsFunc: func() map[string]integrations.SourceInfo {
				return map[string]integrations.SourceInfo{}
			},
		}

		handler := NewAggregatorHandler(mock)
		router := mux.NewRouter()
		handler.RegisterRoutes(router)

		req, _ := http.NewRequest("GET", "/api/sources", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rr.Code)
		}

		var response SourcesResponse
		json.NewDecoder(rr.Body).Decode(&response)
		if response.Total != 0 {
			t.Errorf("expected 0 sources, got %d", response.Total)
		}
	})
}

func TestAggregatorHandler_RequestContext(t *testing.T) {
	t.Run("context is passed to aggregator methods", func(t *testing.T) {
		var contextReceived bool
		mock := &mockMegaAggregator{
			searchArtistsFunc: func(ctx context.Context, query string, limit int) (*integrations.AggregatedResults, error) {
				if ctx != nil {
					contextReceived = true
				}
				return &integrations.AggregatedResults{}, nil
			},
		}

		handler := NewAggregatorHandler(mock)
		router := mux.NewRouter()
		handler.RegisterRoutes(router)

		req, _ := http.NewRequest("GET", "/api/search/artists?q=test", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if !contextReceived {
			t.Error("expected context to be passed to aggregator method")
		}
	})
}
