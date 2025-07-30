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

type ArtistHandler struct {
	service domain.ArtistService
}

func NewArtistHandler(service domain.ArtistService) *ArtistHandler {
	return &ArtistHandler{
		service: service,
	}
}

func (h *ArtistHandler) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/api/artists/search", h.SearchArtists).Methods("GET")
	router.HandleFunc("/api/artists/{id}", h.GetArtist).Methods("GET")
	router.HandleFunc("/api/artists", h.SaveArtist).Methods("POST")
}

func (h *ArtistHandler) SearchArtists(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	query := r.URL.Query().Get("q")
	if query == "" {
		h.respondWithError(w, http.StatusBadRequest, "query parameter 'q' is required")
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 10
	if limitStr != "" {
		parsedLimit, err := strconv.Atoi(limitStr)
		if err != nil || parsedLimit <= 0 {
			h.respondWithError(w, http.StatusBadRequest, "limit must be a positive integer")
			return
		}
		limit = parsedLimit
	}

	response, err := h.service.SearchArtists(ctx, query, limit)
	if err != nil {
		switch err {
		case domain.ErrInvalidRequest:
			h.respondWithError(w, http.StatusBadRequest, err.Error())
		case domain.ErrExternalAPIFailure:
			h.respondWithError(w, http.StatusServiceUnavailable, "external service unavailable")
		default:
			h.respondWithError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	h.respondWithJSON(w, http.StatusOK, response)
}

func (h *ArtistHandler) GetArtist(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	vars := mux.Vars(r)
	id := vars["id"]

	artist, err := h.service.GetArtist(ctx, id)
	if err != nil {
		switch err {
		case domain.ErrArtistNotFound:
			h.respondWithError(w, http.StatusNotFound, "artist not found")
		default:
			h.respondWithError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	h.respondWithJSON(w, http.StatusOK, artist)
}

func (h *ArtistHandler) SaveArtist(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	var artist domain.Artist
	if err := json.NewDecoder(r.Body).Decode(&artist); err != nil {
		h.respondWithError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if artist.Name == "" {
		h.respondWithError(w, http.StatusBadRequest, "artist name is required")
		return
	}

	if err := h.service.SaveArtist(ctx, &artist); err != nil {
		switch err {
		case domain.ErrDuplicateArtist:
			h.respondWithError(w, http.StatusConflict, "artist already exists")
		case domain.ErrInvalidRequest:
			h.respondWithError(w, http.StatusBadRequest, err.Error())
		default:
			h.respondWithError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	h.respondWithJSON(w, http.StatusCreated, artist)
}

func (h *ArtistHandler) respondWithError(w http.ResponseWriter, code int, message string) {
	h.respondWithJSON(w, code, map[string]string{"error": message})
}

func (h *ArtistHandler) respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
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
