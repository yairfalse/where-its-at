package interfaces

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"github.com/yair/where-its-at/pkg/integrations"
)

// AggregatorService defines the interface for the mega aggregator
type AggregatorService interface {
	SearchArtists(ctx context.Context, query string, limit int) (*integrations.AggregatedResults, error)
	SearchEvents(ctx context.Context, artistName string, limit int) (*integrations.AggregatedResults, error)
	SearchEventsByLocation(ctx context.Context, city, country string, limit int) (*integrations.AggregatedResults, error)
	GetSourceStats() map[string]integrations.SourceInfo
}

type AggregatorHandler struct {
	aggregator AggregatorService
}

func NewAggregatorHandler(aggregator AggregatorService) *AggregatorHandler {
	return &AggregatorHandler{
		aggregator: aggregator,
	}
}

func (h *AggregatorHandler) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/api/search/artists", h.SearchArtists).Methods("GET")
	router.HandleFunc("/api/search/events", h.SearchEvents).Methods("GET")
	router.HandleFunc("/api/search/events/location", h.SearchEventsByLocation).Methods("GET")
	router.HandleFunc("/api/sources", h.GetSources).Methods("GET")
}

func (h *AggregatorHandler) SearchArtists(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		h.writeErrorResponse(w, http.StatusBadRequest, "query parameter 'q' is required")
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
			if limit > 200 {
				limit = 200
			}
		}
	}

	ctx := r.Context()
	results, err := h.aggregator.SearchArtists(ctx, query, limit)
	if err != nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, "failed to search artists")
		return
	}

	h.writeJSONResponse(w, http.StatusOK, results)
}

func (h *AggregatorHandler) SearchEvents(w http.ResponseWriter, r *http.Request) {
	artistName := r.URL.Query().Get("artist")
	if artistName == "" {
		h.writeErrorResponse(w, http.StatusBadRequest, "query parameter 'artist' is required")
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
			if limit > 200 {
				limit = 200
			}
		}
	}

	ctx := r.Context()
	results, err := h.aggregator.SearchEvents(ctx, artistName, limit)
	if err != nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, "failed to search events")
		return
	}

	h.writeJSONResponse(w, http.StatusOK, results)
}

func (h *AggregatorHandler) SearchEventsByLocation(w http.ResponseWriter, r *http.Request) {
	city := r.URL.Query().Get("city")
	if city == "" {
		h.writeErrorResponse(w, http.StatusBadRequest, "query parameter 'city' is required")
		return
	}

	country := r.URL.Query().Get("country")

	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
			if limit > 200 {
				limit = 200
			}
		}
	}

	ctx := r.Context()
	results, err := h.aggregator.SearchEventsByLocation(ctx, city, country, limit)
	if err != nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, "failed to search events by location")
		return
	}

	h.writeJSONResponse(w, http.StatusOK, results)
}

func (h *AggregatorHandler) GetSources(w http.ResponseWriter, r *http.Request) {
	sources := h.aggregator.GetSourceStats()

	response := SourcesResponse{
		Sources: sources,
		Total:   len(sources),
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

func (h *AggregatorHandler) writeJSONResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	
	// Try to encode first to check for errors before writing status
	encoded, err := json.Marshal(data)
	if err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
	
	w.WriteHeader(status)
	w.Write(encoded)
}

func (h *AggregatorHandler) writeErrorResponse(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	response := ErrorResponse{
		Error:  message,
		Status: status,
	}

	json.NewEncoder(w).Encode(response)
}

type SourcesResponse struct {
	Sources map[string]integrations.SourceInfo `json:"sources"`
	Total   int                                `json:"total"`
}

type ErrorResponse struct {
	Error  string `json:"error"`
	Status int    `json:"status"`
}

func (h *AggregatorHandler) formatLocation(city, country string) string {
	parts := []string{}
	if city != "" {
		parts = append(parts, city)
	}
	if country != "" {
		parts = append(parts, country)
	}
	return strings.Join(parts, ", ")
}
