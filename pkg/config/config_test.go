package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	t.Run("loads from file", func(t *testing.T) {
		// Create temp config file
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.json")

		testConfig := Config{
			Server: ServerConfig{
				Port: "9090",
			},
			Database: DatabaseConfig{
				Host:     "localhost",
				Port:     5432,
				User:     "testuser",
				Password: "testpass",
				Database: "testdb",
			},
			APIs: APIConfig{
				Spotify: SpotifyConfig{
					ClientID:     "test-client-id",
					ClientSecret: "test-client-secret",
				},
			},
		}

		data, _ := json.Marshal(testConfig)
		if err := os.WriteFile(configPath, data, 0644); err != nil {
			t.Fatal(err)
		}

		// Load config
		config, err := Load(configPath)
		if err != nil {
			t.Fatalf("failed to load config: %v", err)
		}

		// Verify values
		if config.Server.Port != "9090" {
			t.Errorf("expected port 9090, got %s", config.Server.Port)
		}
		if config.Database.User != "testuser" {
			t.Errorf("expected user testuser, got %s", config.Database.User)
		}
		if config.APIs.Spotify.ClientID != "test-client-id" {
			t.Errorf("expected client ID test-client-id, got %s", config.APIs.Spotify.ClientID)
		}
	})

	t.Run("applies defaults", func(t *testing.T) {
		config, err := Load("")
		if err != nil {
			t.Fatalf("failed to load config: %v", err)
		}

		if config.Server.Port != "8080" {
			t.Errorf("expected default port 8080, got %s", config.Server.Port)
		}
		if config.Server.ReadTimeout != 30 {
			t.Errorf("expected default read timeout 30, got %d", config.Server.ReadTimeout)
		}
		if config.Database.Port != 5432 {
			t.Errorf("expected default database port 5432, got %d", config.Database.Port)
		}
		if config.Cache.EventCacheDuration != 24 {
			t.Errorf("expected default cache duration 24, got %d", config.Cache.EventCacheDuration)
		}
	})

	t.Run("environment overrides", func(t *testing.T) {
		// Set env vars
		os.Setenv("WHEREITS_SERVER_PORT", "7070")
		os.Setenv("WHEREITS_DATABASE_HOST", "env-host")
		os.Setenv("WHEREITS_SPOTIFY_CLIENT_ID", "env-spotify-id")
		defer func() {
			os.Unsetenv("WHEREITS_SERVER_PORT")
			os.Unsetenv("WHEREITS_DATABASE_HOST")
			os.Unsetenv("WHEREITS_SPOTIFY_CLIENT_ID")
		}()

		config, err := Load("")
		if err != nil {
			t.Fatalf("failed to load config: %v", err)
		}

		if config.Server.Port != "7070" {
			t.Errorf("expected env port 7070, got %s", config.Server.Port)
		}
		if config.Database.Host != "env-host" {
			t.Errorf("expected env host env-host, got %s", config.Database.Host)
		}
		if config.APIs.Spotify.ClientID != "env-spotify-id" {
			t.Errorf("expected env spotify ID env-spotify-id, got %s", config.APIs.Spotify.ClientID)
		}
	})

	t.Run("handles missing file", func(t *testing.T) {
		config, err := Load("/non/existent/path.json")
		if err != nil {
			t.Fatalf("should not error on missing file: %v", err)
		}

		// Should still have defaults
		if config.Server.Port != "8080" {
			t.Errorf("expected default port 8080, got %s", config.Server.Port)
		}
	})
}

func TestDatabaseConfig_GetDSN(t *testing.T) {
	config := DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "testuser",
		Password: "testpass",
		Database: "testdb",
		SSLMode:  "disable",
	}

	expected := "host=localhost port=5432 user=testuser password=testpass dbname=testdb sslmode=disable"
	dsn := config.GetDSN()

	if dsn != expected {
		t.Errorf("expected DSN %s, got %s", expected, dsn)
	}
}

func TestConfig_Validate(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		config := &Config{
			Database: DatabaseConfig{
				Host:     "localhost",
				User:     "user",
				Database: "db",
			},
			APIs: APIConfig{
				Spotify: SpotifyConfig{
					ClientID:     "id",
					ClientSecret: "secret",
				},
				Songkick: SongkickConfig{
					APIKey: "key",
				},
			},
		}

		if err := config.Validate(); err != nil {
			t.Errorf("expected valid config, got error: %v", err)
		}
	})

	t.Run("missing database config", func(t *testing.T) {
		config := &Config{
			APIs: APIConfig{
				Spotify: SpotifyConfig{
					ClientID:     "id",
					ClientSecret: "secret",
				},
			},
		}

		err := config.Validate()
		if err == nil {
			t.Error("expected validation error for missing database config")
		}
	})

	t.Run("missing music API", func(t *testing.T) {
		config := &Config{
			Database: DatabaseConfig{
				Host:     "localhost",
				User:     "user",
				Database: "db",
			},
			APIs: APIConfig{
				Songkick: SongkickConfig{
					APIKey: "key",
				},
			},
		}

		err := config.Validate()
		if err == nil {
			t.Error("expected validation error for missing music API")
		}
	})

	t.Run("missing event API", func(t *testing.T) {
		config := &Config{
			Database: DatabaseConfig{
				Host:     "localhost",
				User:     "user",
				Database: "db",
			},
			APIs: APIConfig{
				Spotify: SpotifyConfig{
					ClientID:     "id",
					ClientSecret: "secret",
				},
			},
		}

		err := config.Validate()
		if err == nil {
			t.Error("expected validation error for missing event API")
		}
	})
}

func TestApplyDefaults(t *testing.T) {
	config := &Config{}
	applyDefaults(config)

	if config.Server.Port != "8080" {
		t.Errorf("expected default port 8080, got %s", config.Server.Port)
	}
	if config.Server.ReadTimeout != 30 {
		t.Errorf("expected default read timeout 30, got %d", config.Server.ReadTimeout)
	}
	if config.Server.WriteTimeout != 30 {
		t.Errorf("expected default write timeout 30, got %d", config.Server.WriteTimeout)
	}
	if config.Database.Port != 5432 {
		t.Errorf("expected default database port 5432, got %d", config.Database.Port)
	}
	if config.Database.SSLMode != "disable" {
		t.Errorf("expected default SSL mode disable, got %s", config.Database.SSLMode)
	}
	if config.APIs.MusicBrainz.UserAgent != "WhereItsAt/1.0" {
		t.Errorf("expected default MusicBrainz user agent, got %s", config.APIs.MusicBrainz.UserAgent)
	}
	if config.Scrapers.UserAgent != "Mozilla/5.0 (compatible; WhereItsAt/1.0)" {
		t.Errorf("expected default scraper user agent, got %s", config.Scrapers.UserAgent)
	}
	if config.Scrapers.RateLimitSeconds != 2 {
		t.Errorf("expected default rate limit 2, got %d", config.Scrapers.RateLimitSeconds)
	}
	if config.Scrapers.Timeout != 30 {
		t.Errorf("expected default timeout 30, got %d", config.Scrapers.Timeout)
	}
	if config.Cache.EventCacheDuration != 24 {
		t.Errorf("expected default cache duration 24, got %d", config.Cache.EventCacheDuration)
	}
}

func TestApplyEnvOverrides(t *testing.T) {
	// Set all env vars
	envVars := map[string]string{
		"WHEREITS_SERVER_PORT":             "9999",
		"WHEREITS_DATABASE_HOST":           "env-db-host",
		"WHEREITS_DATABASE_USER":           "env-db-user",
		"WHEREITS_DATABASE_PASSWORD":       "env-db-pass",
		"WHEREITS_DATABASE_NAME":           "env-db-name",
		"WHEREITS_SPOTIFY_CLIENT_ID":       "env-spotify-id",
		"WHEREITS_SPOTIFY_CLIENT_SECRET":   "env-spotify-secret",
		"WHEREITS_APPLE_MUSIC_TEAM_ID":     "env-apple-team",
		"WHEREITS_APPLE_MUSIC_KEY_ID":      "env-apple-key",
		"WHEREITS_APPLE_MUSIC_PRIVATE_KEY": "env-apple-private",
		"WHEREITS_YOUTUBE_API_KEY":         "env-youtube",
		"WHEREITS_DEEZER_APP_ID":           "env-deezer-id",
		"WHEREITS_DEEZER_APP_SECRET":       "env-deezer-secret",
		"WHEREITS_SOUNDCLOUD_CLIENT_ID":    "env-soundcloud",
		"WHEREITS_SONGKICK_API_KEY":        "env-songkick",
		"WHEREITS_TICKETMASTER_API_KEY":    "env-ticketmaster",
		"WHEREITS_EVENTBRITE_TOKEN":        "env-eventbrite",
		"WHEREITS_SETLISTFM_API_KEY":       "env-setlistfm",
	}

	for k, v := range envVars {
		os.Setenv(k, v)
	}
	defer func() {
		for k := range envVars {
			os.Unsetenv(k)
		}
	}()

	config := &Config{}
	applyEnvOverrides(config)

	// Verify all overrides
	if config.Server.Port != "9999" {
		t.Errorf("expected env port 9999, got %s", config.Server.Port)
	}
	if config.Database.Host != "env-db-host" {
		t.Errorf("expected env database host, got %s", config.Database.Host)
	}
	if config.Database.User != "env-db-user" {
		t.Errorf("expected env database user, got %s", config.Database.User)
	}
	if config.Database.Password != "env-db-pass" {
		t.Errorf("expected env database password, got %s", config.Database.Password)
	}
	if config.Database.Database != "env-db-name" {
		t.Errorf("expected env database name, got %s", config.Database.Database)
	}
	if config.APIs.Spotify.ClientID != "env-spotify-id" {
		t.Errorf("expected env spotify client ID, got %s", config.APIs.Spotify.ClientID)
	}
	if config.APIs.Spotify.ClientSecret != "env-spotify-secret" {
		t.Errorf("expected env spotify client secret, got %s", config.APIs.Spotify.ClientSecret)
	}
	if config.APIs.AppleMusic.TeamID != "env-apple-team" {
		t.Errorf("expected env apple team ID, got %s", config.APIs.AppleMusic.TeamID)
	}
	if config.APIs.AppleMusic.KeyID != "env-apple-key" {
		t.Errorf("expected env apple key ID, got %s", config.APIs.AppleMusic.KeyID)
	}
	if config.APIs.AppleMusic.PrivateKey != "env-apple-private" {
		t.Errorf("expected env apple private key, got %s", config.APIs.AppleMusic.PrivateKey)
	}
	if config.APIs.YouTube.APIKey != "env-youtube" {
		t.Errorf("expected env youtube API key, got %s", config.APIs.YouTube.APIKey)
	}
	if config.APIs.Deezer.AppID != "env-deezer-id" {
		t.Errorf("expected env deezer app ID, got %s", config.APIs.Deezer.AppID)
	}
	if config.APIs.Deezer.AppSecret != "env-deezer-secret" {
		t.Errorf("expected env deezer app secret, got %s", config.APIs.Deezer.AppSecret)
	}
	if config.APIs.SoundCloud.ClientID != "env-soundcloud" {
		t.Errorf("expected env soundcloud client ID, got %s", config.APIs.SoundCloud.ClientID)
	}
	if config.APIs.Songkick.APIKey != "env-songkick" {
		t.Errorf("expected env songkick API key, got %s", config.APIs.Songkick.APIKey)
	}
	if config.APIs.Ticketmaster.APIKey != "env-ticketmaster" {
		t.Errorf("expected env ticketmaster API key, got %s", config.APIs.Ticketmaster.APIKey)
	}
	if config.APIs.Eventbrite.Token != "env-eventbrite" {
		t.Errorf("expected env eventbrite token, got %s", config.APIs.Eventbrite.Token)
	}
	if config.APIs.SetlistFM.APIKey != "env-setlistfm" {
		t.Errorf("expected env setlistfm API key, got %s", config.APIs.SetlistFM.APIKey)
	}
}
