package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Config holds all configuration for the application
type Config struct {
	Server   ServerConfig   `json:"server"`
	Database DatabaseConfig `json:"database"`
	APIs     APIConfig      `json:"apis"`
	Scrapers ScraperConfig  `json:"scrapers"`
	Cache    CacheConfig    `json:"cache"`
}

// ServerConfig for HTTP server settings
type ServerConfig struct {
	Port         string `json:"port"`
	ReadTimeout  int    `json:"read_timeout_seconds"`
	WriteTimeout int    `json:"write_timeout_seconds"`
}

// DatabaseConfig for PostgreSQL connection
type DatabaseConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Database string `json:"database"`
	SSLMode  string `json:"ssl_mode"`
}

// APIConfig holds all external API configurations
type APIConfig struct {
	Spotify      SpotifyConfig      `json:"spotify"`
	AppleMusic   AppleMusicConfig   `json:"apple_music"`
	YouTube      YouTubeConfig      `json:"youtube"`
	MusicBrainz  MusicBrainzConfig  `json:"musicbrainz"`
	Deezer       DeezerConfig       `json:"deezer"`
	SoundCloud   SoundCloudConfig   `json:"soundcloud"`
	Songkick     SongkickConfig     `json:"songkick"`
	Ticketmaster TicketmasterConfig `json:"ticketmaster"`
	Eventbrite   EventbriteConfig   `json:"eventbrite"`
	SetlistFM    SetlistFMConfig    `json:"setlistfm"`
}

// SpotifyConfig for Spotify API
type SpotifyConfig struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	RedirectURI  string `json:"redirect_uri"`
}

// AppleMusicConfig for Apple Music API
type AppleMusicConfig struct {
	TeamID      string `json:"team_id"`
	KeyID       string `json:"key_id"`
	PrivateKey  string `json:"private_key"`
	CountryCode string `json:"country_code"`
}

// YouTubeConfig for YouTube Music API
type YouTubeConfig struct {
	APIKey string `json:"api_key"`
}

// MusicBrainzConfig for MusicBrainz API
type MusicBrainzConfig struct {
	UserAgent string `json:"user_agent"`
}

// DeezerConfig for Deezer API
type DeezerConfig struct {
	AppID     string `json:"app_id"`
	AppSecret string `json:"app_secret"`
}

// SoundCloudConfig for SoundCloud API
type SoundCloudConfig struct {
	ClientID string `json:"client_id"`
}

// SongkickConfig for Songkick API
type SongkickConfig struct {
	APIKey string `json:"api_key"`
}

// TicketmasterConfig for Ticketmaster Discovery API
type TicketmasterConfig struct {
	APIKey string `json:"api_key"`
}

// EventbriteConfig for Eventbrite API
type EventbriteConfig struct {
	Token string `json:"token"`
}

// SetlistFMConfig for Setlist.fm API
type SetlistFMConfig struct {
	APIKey string `json:"api_key"`
}

// ScraperConfig for web scraper settings
type ScraperConfig struct {
	UserAgent        string `json:"user_agent"`
	RateLimitSeconds int    `json:"rate_limit_seconds"`
	Timeout          int    `json:"timeout_seconds"`
}

// CacheConfig for caching settings
type CacheConfig struct {
	EventCacheDuration int `json:"event_cache_duration_hours"`
}

// Load reads configuration from file and environment variables
// Environment variables override file values using the pattern WHEREITS_SECTION_KEY
func Load(configPath string) (*Config, error) {
	config := &Config{}

	// Load from file if it exists
	if configPath != "" {
		data, err := os.ReadFile(configPath)
		if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		if err == nil {
			if err := json.Unmarshal(data, config); err != nil {
				return nil, fmt.Errorf("failed to parse config file: %w", err)
			}
		}
	}

	// Apply defaults
	applyDefaults(config)

	// Override with environment variables
	applyEnvOverrides(config)

	return config, nil
}

func applyDefaults(config *Config) {
	if config.Server.Port == "" {
		config.Server.Port = "8080"
	}
	if config.Server.ReadTimeout == 0 {
		config.Server.ReadTimeout = 30
	}
	if config.Server.WriteTimeout == 0 {
		config.Server.WriteTimeout = 30
	}
	if config.Database.Port == 0 {
		config.Database.Port = 5432
	}
	if config.Database.SSLMode == "" {
		config.Database.SSLMode = "disable"
	}
	if config.APIs.MusicBrainz.UserAgent == "" {
		config.APIs.MusicBrainz.UserAgent = "WhereItsAt/1.0"
	}
	if config.Scrapers.UserAgent == "" {
		config.Scrapers.UserAgent = "Mozilla/5.0 (compatible; WhereItsAt/1.0)"
	}
	if config.Scrapers.RateLimitSeconds == 0 {
		config.Scrapers.RateLimitSeconds = 2
	}
	if config.Scrapers.Timeout == 0 {
		config.Scrapers.Timeout = 30
	}
	if config.Cache.EventCacheDuration == 0 {
		config.Cache.EventCacheDuration = 24
	}
}

func applyEnvOverrides(config *Config) {
	// Server overrides
	if v := os.Getenv("WHEREITS_SERVER_PORT"); v != "" {
		config.Server.Port = v
	}

	// Database overrides
	if v := os.Getenv("WHEREITS_DATABASE_HOST"); v != "" {
		config.Database.Host = v
	}
	if v := os.Getenv("WHEREITS_DATABASE_USER"); v != "" {
		config.Database.User = v
	}
	if v := os.Getenv("WHEREITS_DATABASE_PASSWORD"); v != "" {
		config.Database.Password = v
	}
	if v := os.Getenv("WHEREITS_DATABASE_NAME"); v != "" {
		config.Database.Database = v
	}

	// API key overrides
	if v := os.Getenv("WHEREITS_SPOTIFY_CLIENT_ID"); v != "" {
		config.APIs.Spotify.ClientID = v
	}
	if v := os.Getenv("WHEREITS_SPOTIFY_CLIENT_SECRET"); v != "" {
		config.APIs.Spotify.ClientSecret = v
	}
	if v := os.Getenv("WHEREITS_APPLE_MUSIC_TEAM_ID"); v != "" {
		config.APIs.AppleMusic.TeamID = v
	}
	if v := os.Getenv("WHEREITS_APPLE_MUSIC_KEY_ID"); v != "" {
		config.APIs.AppleMusic.KeyID = v
	}
	if v := os.Getenv("WHEREITS_APPLE_MUSIC_PRIVATE_KEY"); v != "" {
		config.APIs.AppleMusic.PrivateKey = v
	}
	if v := os.Getenv("WHEREITS_YOUTUBE_API_KEY"); v != "" {
		config.APIs.YouTube.APIKey = v
	}
	if v := os.Getenv("WHEREITS_DEEZER_APP_ID"); v != "" {
		config.APIs.Deezer.AppID = v
	}
	if v := os.Getenv("WHEREITS_DEEZER_APP_SECRET"); v != "" {
		config.APIs.Deezer.AppSecret = v
	}
	if v := os.Getenv("WHEREITS_SOUNDCLOUD_CLIENT_ID"); v != "" {
		config.APIs.SoundCloud.ClientID = v
	}
	if v := os.Getenv("WHEREITS_SONGKICK_API_KEY"); v != "" {
		config.APIs.Songkick.APIKey = v
	}
	if v := os.Getenv("WHEREITS_TICKETMASTER_API_KEY"); v != "" {
		config.APIs.Ticketmaster.APIKey = v
	}
	if v := os.Getenv("WHEREITS_EVENTBRITE_TOKEN"); v != "" {
		config.APIs.Eventbrite.Token = v
	}
	if v := os.Getenv("WHEREITS_SETLISTFM_API_KEY"); v != "" {
		config.APIs.SetlistFM.APIKey = v
	}
}

// GetDSN returns the PostgreSQL connection string
func (c *DatabaseConfig) GetDSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Database, c.SSLMode)
}

// Validate checks if required configurations are present
func (c *Config) Validate() error {
	var missing []string

	// Database validation
	if c.Database.Host == "" {
		missing = append(missing, "database.host")
	}
	if c.Database.User == "" {
		missing = append(missing, "database.user")
	}
	if c.Database.Database == "" {
		missing = append(missing, "database.database")
	}

	// At least one music API should be configured
	hasMusic := false
	if c.APIs.Spotify.ClientID != "" && c.APIs.Spotify.ClientSecret != "" {
		hasMusic = true
	}
	if c.APIs.AppleMusic.TeamID != "" && c.APIs.AppleMusic.KeyID != "" {
		hasMusic = true
	}
	if c.APIs.YouTube.APIKey != "" {
		hasMusic = true
	}
	if c.APIs.Deezer.AppID != "" {
		hasMusic = true
	}
	if c.APIs.SoundCloud.ClientID != "" {
		hasMusic = true
	}

	if !hasMusic {
		missing = append(missing, "at least one music API")
	}

	// At least one event API should be configured
	hasEvents := false
	if c.APIs.Songkick.APIKey != "" {
		hasEvents = true
	}
	if c.APIs.Ticketmaster.APIKey != "" {
		hasEvents = true
	}
	if c.APIs.Eventbrite.Token != "" {
		hasEvents = true
	}
	if c.APIs.SetlistFM.APIKey != "" {
		hasEvents = true
	}

	if !hasEvents {
		missing = append(missing, "at least one event API")
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required configuration: %s", strings.Join(missing, ", "))
	}

	return nil
}
