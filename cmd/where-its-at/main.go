package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
	"github.com/yair/where-its-at/pkg/collectors"
	"github.com/yair/where-its-at/pkg/config"
	"github.com/yair/where-its-at/pkg/integrations"
	"github.com/yair/where-its-at/pkg/interfaces"
)

func main() {
	log.Println("Starting Where It's At...")

	// Load configuration
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config.json"
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Printf("Warning: Failed to load config: %v. Using defaults.", err)
		cfg = &config.Config{}
	}

	// Initialize database
	db, err := sql.Open("sqlite3", "./where-its-at.db")
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Initialize repositories
	artistRepo, err := collectors.NewArtistRepository(db)
	if err != nil {
		log.Fatalf("Failed to create artist repository: %v", err)
	}

	// Initialize integrations (optional - only if configured)
	var artistAggregator *integrations.ArtistAggregator
	if cfg.APIs.Spotify.ClientID != "" {
		spotifyClient, err := integrations.NewSpotifyClient(integrations.SpotifyConfig{
			ClientID:     cfg.APIs.Spotify.ClientID,
			ClientSecret: cfg.APIs.Spotify.ClientSecret,
		})
		if err != nil {
			log.Printf("Warning: Failed to create Spotify client: %v", err)
		} else {
			artistAggregator = integrations.NewArtistAggregator(spotifyClient, nil)
		}
	}

	// Initialize services
	artistService := interfaces.NewArtistService(artistRepo, artistAggregator)

	// Initialize HTTP handlers
	artistHandler := interfaces.NewArtistHandler(artistService)

	// Setup router
	router := mux.NewRouter()
	artistHandler.RegisterRoutes(router)

	// Health check endpoint
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}).Methods("GET")

	// Log available routes
	log.Println("Available routes:")
	router.Walk(func(route *mux.Route, router *mux.Router, ancestors []*mux.Route) error {
		path, _ := route.GetPathTemplate()
		methods, _ := route.GetMethods()
		log.Printf("  %v %s", methods, path)
		return nil
	})

	// Setup HTTP server
	port := cfg.Server.Port
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Server listening on port %s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	log.Println("Server stopped. That was a good drum break.")
}
