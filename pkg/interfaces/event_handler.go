package interfaces

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/yair/where-its-at/pkg/domain"
)

type EventHandler struct {
	service domain.EventService
}

func NewEventHandler(service domain.EventService) *EventHandler {
	return &EventHandler{
		service: service,
	}
}

func (h *EventHandler) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/api/artists/{id}/events", h.GetArtistEvents).Methods("GET")
	router.HandleFunc("/api/events/search", h.SearchEvents).Methods("GET")
	router.HandleFunc("/api/events/{id}", h.GetEvent).Methods("GET")
}

func (h *EventHandler) GetArtistEvents(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	vars := mux.Vars(r)
	artistID := vars["id"]

	response, err := h.service.GetArtistEvents(ctx, artistID)
	if err != nil {
		switch err {
		case domain.ErrArtistNotFound:
			h.respondWithError(w, http.StatusNotFound, "artist not found")
		case domain.ErrRateLimitExceeded:
			h.respondWithError(w, http.StatusTooManyRequests, "rate limit exceeded")
		case domain.ErrExternalAPIFailure:
			h.respondWithError(w, http.StatusServiceUnavailable, "external service unavailable")
		default:
			h.respondWithError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	h.respondWithJSON(w, http.StatusOK, response)
}

func (h *EventHandler) SearchEvents(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	artistName := r.URL.Query().Get("artist")
	location := r.URL.Query().Get("location")
	radiusStr := r.URL.Query().Get("radius")

	if artistName == "" {
		h.respondWithError(w, http.StatusBadRequest, "artist parameter is required")
		return
	}

	radius := 50
	if radiusStr != "" {
		parsedRadius, err := strconv.Atoi(radiusStr)
		if err != nil || parsedRadius <= 0 {
			h.respondWithError(w, http.StatusBadRequest, "radius must be a positive integer")
			return
		}
		if parsedRadius > 500 {
			parsedRadius = 500
		}
		radius = parsedRadius
	}

	response, err := h.service.SearchArtistEvents(ctx, artistName, location, radius)
	if err != nil {
		switch err {
		case domain.ErrInvalidRequest:
			h.respondWithError(w, http.StatusBadRequest, err.Error())
		case domain.ErrRateLimitExceeded:
			h.respondWithError(w, http.StatusTooManyRequests, "rate limit exceeded")
		case domain.ErrExternalAPIFailure:
			h.respondWithError(w, http.StatusServiceUnavailable, "external service unavailable")
		default:
			h.respondWithError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	h.respondWithJSON(w, http.StatusOK, response)
}

func (h *EventHandler) GetEvent(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	vars := mux.Vars(r)
	id := vars["id"]

	event, err := h.service.GetEvent(ctx, id)
	if err != nil {
		switch err {
		case domain.ErrEventNotFound:
			h.respondWithError(w, http.StatusNotFound, "event not found")
		case domain.ErrRateLimitExceeded:
			h.respondWithError(w, http.StatusTooManyRequests, "rate limit exceeded")
		default:
			h.respondWithError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	h.respondWithJSON(w, http.StatusOK, event)
}

func (h *EventHandler) respondWithError(w http.ResponseWriter, code int, message string) {
	h.respondWithJSON(w, code, map[string]string{"error": message})
}

func (h *EventHandler) respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, err := json.Marshal(payload)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal server error"}`))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}
