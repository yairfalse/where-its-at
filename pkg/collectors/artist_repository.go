package collectors

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/yair/where-its-at/pkg/domain"
)

type ArtistRepository struct {
	db *sql.DB
}

func NewArtistRepository(db *sql.DB) (*ArtistRepository, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is required")
	}

	repo := &ArtistRepository{db: db}
	if err := repo.createTables(); err != nil {
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	return repo, nil
}

func (r *ArtistRepository) createTables() error {
	query := `
	CREATE TABLE IF NOT EXISTS artists (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		spotify_id TEXT,
		lastfm_id TEXT,
		genres TEXT,
		popularity INTEGER,
		image_url TEXT,
		created_at TIMESTAMP NOT NULL,
		updated_at TIMESTAMP NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_artists_spotify_id ON artists(spotify_id);
	CREATE INDEX IF NOT EXISTS idx_artists_lastfm_id ON artists(lastfm_id);
	CREATE INDEX IF NOT EXISTS idx_artists_name ON artists(name);
	`

	_, err := r.db.Exec(query)
	return err
}

func (r *ArtistRepository) Create(ctx context.Context, artist *domain.Artist) error {
	if artist == nil {
		return fmt.Errorf("artist cannot be nil")
	}

	query := `
	INSERT INTO artists (id, name, spotify_id, lastfm_id, genres, popularity, image_url, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	genres := strings.Join(artist.Genres, ",")
	now := time.Now()
	artist.CreatedAt = now
	artist.UpdatedAt = now

	_, err := r.db.ExecContext(ctx, query,
		artist.ID,
		artist.Name,
		artist.ExternalIDs.SpotifyID,
		artist.ExternalIDs.LastFMID,
		genres,
		artist.Popularity,
		artist.ImageURL,
		artist.CreatedAt,
		artist.UpdatedAt,
	)

	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return domain.ErrDuplicateArtist
		}
		return fmt.Errorf("failed to create artist: %w", err)
	}

	return nil
}

func (r *ArtistRepository) GetByID(ctx context.Context, id string) (*domain.Artist, error) {
	query := `
	SELECT id, name, spotify_id, lastfm_id, genres, popularity, image_url, created_at, updated_at
	FROM artists
	WHERE id = ?
	`

	var artist domain.Artist
	var genres sql.NullString

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&artist.ID,
		&artist.Name,
		&artist.ExternalIDs.SpotifyID,
		&artist.ExternalIDs.LastFMID,
		&genres,
		&artist.Popularity,
		&artist.ImageURL,
		&artist.CreatedAt,
		&artist.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, domain.ErrArtistNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get artist by id: %w", err)
	}

	if genres.Valid && genres.String != "" {
		artist.Genres = strings.Split(genres.String, ",")
	}

	return &artist, nil
}

func (r *ArtistRepository) GetByExternalID(ctx context.Context, externalID string, source string) (*domain.Artist, error) {
	var query string
	switch source {
	case "spotify":
		query = `
		SELECT id, name, spotify_id, lastfm_id, genres, popularity, image_url, created_at, updated_at
		FROM artists
		WHERE spotify_id = ?
		`
	case "lastfm":
		query = `
		SELECT id, name, spotify_id, lastfm_id, genres, popularity, image_url, created_at, updated_at
		FROM artists
		WHERE lastfm_id = ?
		`
	default:
		return nil, fmt.Errorf("invalid source: %s", source)
	}

	var artist domain.Artist
	var genres sql.NullString

	err := r.db.QueryRowContext(ctx, query, externalID).Scan(
		&artist.ID,
		&artist.Name,
		&artist.ExternalIDs.SpotifyID,
		&artist.ExternalIDs.LastFMID,
		&genres,
		&artist.Popularity,
		&artist.ImageURL,
		&artist.CreatedAt,
		&artist.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, domain.ErrArtistNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get artist by external id: %w", err)
	}

	if genres.Valid && genres.String != "" {
		artist.Genres = strings.Split(genres.String, ",")
	}

	return &artist, nil
}

func (r *ArtistRepository) Search(ctx context.Context, query string, limit int) ([]domain.Artist, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	sqlQuery := `
	SELECT id, name, spotify_id, lastfm_id, genres, popularity, image_url, created_at, updated_at
	FROM artists
	WHERE name LIKE ?
	ORDER BY popularity DESC
	LIMIT ?
	`

	rows, err := r.db.QueryContext(ctx, sqlQuery, "%"+query+"%", limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search artists: %w", err)
	}
	defer rows.Close()

	var artists []domain.Artist
	for rows.Next() {
		var artist domain.Artist
		var genres sql.NullString

		err := rows.Scan(
			&artist.ID,
			&artist.Name,
			&artist.ExternalIDs.SpotifyID,
			&artist.ExternalIDs.LastFMID,
			&genres,
			&artist.Popularity,
			&artist.ImageURL,
			&artist.CreatedAt,
			&artist.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan artist: %w", err)
		}

		if genres.Valid && genres.String != "" {
			artist.Genres = strings.Split(genres.String, ",")
		}

		artists = append(artists, artist)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate rows: %w", err)
	}

	return artists, nil
}

func (r *ArtistRepository) Update(ctx context.Context, artist *domain.Artist) error {
	if artist == nil {
		return fmt.Errorf("artist cannot be nil")
	}

	query := `
	UPDATE artists
	SET name = ?, spotify_id = ?, lastfm_id = ?, genres = ?, popularity = ?, image_url = ?, updated_at = ?
	WHERE id = ?
	`

	genres := strings.Join(artist.Genres, ",")
	artist.UpdatedAt = time.Now()

	result, err := r.db.ExecContext(ctx, query,
		artist.Name,
		artist.ExternalIDs.SpotifyID,
		artist.ExternalIDs.LastFMID,
		genres,
		artist.Popularity,
		artist.ImageURL,
		artist.UpdatedAt,
		artist.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update artist: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return domain.ErrArtistNotFound
	}

	return nil
}

func (r *ArtistRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM artists WHERE id = ?`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete artist: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return domain.ErrArtistNotFound
	}

	return nil
}
