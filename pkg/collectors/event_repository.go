package collectors

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/yair/where-its-at/pkg/domain"
)

type EventRepository struct {
	db *sql.DB
}

func NewEventRepository(db *sql.DB) (*EventRepository, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is required")
	}

	repo := &EventRepository{db: db}
	if err := repo.createTables(); err != nil {
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	return repo, nil
}

func (r *EventRepository) createTables() error {
	query := `
	CREATE TABLE IF NOT EXISTS events (
		id TEXT PRIMARY KEY,
		artist_id TEXT NOT NULL,
		artist_name TEXT NOT NULL,
		title TEXT,
		datetime TIMESTAMP NOT NULL,
		venue_id TEXT,
		venue_name TEXT NOT NULL,
		venue_city TEXT NOT NULL,
		venue_region TEXT,
		venue_country TEXT NOT NULL,
		venue_latitude REAL,
		venue_longitude REAL,
		ticket_url TEXT,
		ticket_status TEXT,
		on_sale_date TIMESTAMP,
		bandsintown_id TEXT,
		ticketmaster_id TEXT,
		created_at TIMESTAMP NOT NULL,
		updated_at TIMESTAMP NOT NULL,
		cached_until TIMESTAMP NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_events_artist_id ON events(artist_id);
	CREATE INDEX IF NOT EXISTS idx_events_datetime ON events(datetime);
	CREATE INDEX IF NOT EXISTS idx_events_bandsintown_id ON events(bandsintown_id);
	CREATE INDEX IF NOT EXISTS idx_events_cached_until ON events(cached_until);
	CREATE INDEX IF NOT EXISTS idx_events_location ON events(venue_latitude, venue_longitude);
	`

	_, err := r.db.Exec(query)
	return err
}

func (r *EventRepository) Create(ctx context.Context, event *domain.Event) error {
	if event == nil {
		return fmt.Errorf("event cannot be nil")
	}

	query := `
	INSERT INTO events (
		id, artist_id, artist_name, title, datetime,
		venue_id, venue_name, venue_city, venue_region, venue_country,
		venue_latitude, venue_longitude, ticket_url, ticket_status,
		on_sale_date, bandsintown_id, ticketmaster_id,
		created_at, updated_at, cached_until
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	now := time.Now()
	event.CreatedAt = now
	event.UpdatedAt = now

	var onSaleDate sql.NullTime
	if event.OnSaleDate != nil {
		onSaleDate = sql.NullTime{Time: *event.OnSaleDate, Valid: true}
	}

	_, err := r.db.ExecContext(ctx, query,
		event.ID,
		event.ArtistID,
		event.ArtistName,
		event.Title,
		event.DateTime,
		event.Venue.ID,
		event.Venue.Name,
		event.Venue.City,
		event.Venue.Region,
		event.Venue.Country,
		event.Venue.Latitude,
		event.Venue.Longitude,
		event.TicketURL,
		event.TicketStatus,
		onSaleDate,
		event.ExternalIDs.BandsintownID,
		event.ExternalIDs.TicketmasterID,
		event.CreatedAt,
		event.UpdatedAt,
		event.CachedUntil,
	)

	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return domain.ErrDuplicateEvent
		}
		return fmt.Errorf("failed to create event: %w", err)
	}

	return nil
}

func (r *EventRepository) CreateBatch(ctx context.Context, events []domain.Event) error {
	if len(events) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT OR REPLACE INTO events (
			id, artist_id, artist_name, title, datetime,
			venue_id, venue_name, venue_city, venue_region, venue_country,
			venue_latitude, venue_longitude, ticket_url, ticket_status,
			on_sale_date, bandsintown_id, ticketmaster_id,
			created_at, updated_at, cached_until
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	now := time.Now()
	for _, event := range events {
		event.CreatedAt = now
		event.UpdatedAt = now

		var onSaleDate sql.NullTime
		if event.OnSaleDate != nil {
			onSaleDate = sql.NullTime{Time: *event.OnSaleDate, Valid: true}
		}

		_, err := stmt.ExecContext(ctx,
			event.ID,
			event.ArtistID,
			event.ArtistName,
			event.Title,
			event.DateTime,
			event.Venue.ID,
			event.Venue.Name,
			event.Venue.City,
			event.Venue.Region,
			event.Venue.Country,
			event.Venue.Latitude,
			event.Venue.Longitude,
			event.TicketURL,
			event.TicketStatus,
			onSaleDate,
			event.ExternalIDs.BandsintownID,
			event.ExternalIDs.TicketmasterID,
			event.CreatedAt,
			event.UpdatedAt,
			event.CachedUntil,
		)
		if err != nil {
			return fmt.Errorf("failed to insert event: %w", err)
		}
	}

	return tx.Commit()
}

func (r *EventRepository) GetByID(ctx context.Context, id string) (*domain.Event, error) {
	query := `
	SELECT id, artist_id, artist_name, title, datetime,
		venue_id, venue_name, venue_city, venue_region, venue_country,
		venue_latitude, venue_longitude, ticket_url, ticket_status,
		on_sale_date, bandsintown_id, ticketmaster_id,
		created_at, updated_at, cached_until
	FROM events
	WHERE id = ?
	`

	event, err := r.scanEvent(r.db.QueryRowContext(ctx, query, id))
	if err == sql.ErrNoRows {
		return nil, domain.ErrEventNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get event by id: %w", err)
	}

	return event, nil
}

func (r *EventRepository) GetByExternalID(ctx context.Context, externalID string, source string) (*domain.Event, error) {
	var query string
	switch source {
	case "bandsintown":
		query = `
		SELECT id, artist_id, artist_name, title, datetime,
			venue_id, venue_name, venue_city, venue_region, venue_country,
			venue_latitude, venue_longitude, ticket_url, ticket_status,
			on_sale_date, bandsintown_id, ticketmaster_id,
			created_at, updated_at, cached_until
		FROM events
		WHERE bandsintown_id = ?
		`
	case "ticketmaster":
		query = `
		SELECT id, artist_id, artist_name, title, datetime,
			venue_id, venue_name, venue_city, venue_region, venue_country,
			venue_latitude, venue_longitude, ticket_url, ticket_status,
			on_sale_date, bandsintown_id, ticketmaster_id,
			created_at, updated_at, cached_until
		FROM events
		WHERE ticketmaster_id = ?
		`
	default:
		return nil, fmt.Errorf("invalid source: %s", source)
	}

	event, err := r.scanEvent(r.db.QueryRowContext(ctx, query, externalID))
	if err == sql.ErrNoRows {
		return nil, domain.ErrEventNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get event by external id: %w", err)
	}

	return event, nil
}

func (r *EventRepository) SearchByArtist(ctx context.Context, artistID string, startDate, endDate *time.Time) ([]domain.Event, error) {
	query := `
	SELECT id, artist_id, artist_name, title, datetime,
		venue_id, venue_name, venue_city, venue_region, venue_country,
		venue_latitude, venue_longitude, ticket_url, ticket_status,
		on_sale_date, bandsintown_id, ticketmaster_id,
		created_at, updated_at, cached_until
	FROM events
	WHERE artist_id = ?
	`
	args := []interface{}{artistID}

	if startDate != nil {
		query += " AND datetime >= ?"
		args = append(args, *startDate)
	}
	if endDate != nil {
		query += " AND datetime <= ?"
		args = append(args, *endDate)
	}

	query += " ORDER BY datetime ASC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to search events by artist: %w", err)
	}
	defer rows.Close()

	return r.scanEvents(rows)
}

func (r *EventRepository) SearchByLocation(ctx context.Context, lat, lng float64, radius int, startDate, endDate *time.Time) ([]domain.Event, error) {
	query := `
	SELECT id, artist_id, artist_name, title, datetime,
		venue_id, venue_name, venue_city, venue_region, venue_country,
		venue_latitude, venue_longitude, ticket_url, ticket_status,
		on_sale_date, bandsintown_id, ticketmaster_id,
		created_at, updated_at, cached_until,
		(
			6371 * acos(
				cos(radians(?)) * cos(radians(venue_latitude)) *
				cos(radians(venue_longitude) - radians(?)) +
				sin(radians(?)) * sin(radians(venue_latitude))
			)
		) AS distance
	FROM events
	WHERE venue_latitude IS NOT NULL AND venue_longitude IS NOT NULL
	`
	args := []interface{}{lat, lng, lat}

	if startDate != nil {
		query += " AND datetime >= ?"
		args = append(args, *startDate)
	}
	if endDate != nil {
		query += " AND datetime <= ?"
		args = append(args, *endDate)
	}

	query += " HAVING distance <= ? ORDER BY distance ASC, datetime ASC"
	args = append(args, radius)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to search events by location: %w", err)
	}
	defer rows.Close()

	var events []domain.Event
	for rows.Next() {
		event, err := r.scanEventWithDistance(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, *event)
	}

	return events, rows.Err()
}

func (r *EventRepository) Update(ctx context.Context, event *domain.Event) error {
	if event == nil {
		return fmt.Errorf("event cannot be nil")
	}

	query := `
	UPDATE events
	SET artist_id = ?, artist_name = ?, title = ?, datetime = ?,
		venue_id = ?, venue_name = ?, venue_city = ?, venue_region = ?, venue_country = ?,
		venue_latitude = ?, venue_longitude = ?, ticket_url = ?, ticket_status = ?,
		on_sale_date = ?, bandsintown_id = ?, ticketmaster_id = ?,
		updated_at = ?, cached_until = ?
	WHERE id = ?
	`

	event.UpdatedAt = time.Now()

	var onSaleDate sql.NullTime
	if event.OnSaleDate != nil {
		onSaleDate = sql.NullTime{Time: *event.OnSaleDate, Valid: true}
	}

	result, err := r.db.ExecContext(ctx, query,
		event.ArtistID,
		event.ArtistName,
		event.Title,
		event.DateTime,
		event.Venue.ID,
		event.Venue.Name,
		event.Venue.City,
		event.Venue.Region,
		event.Venue.Country,
		event.Venue.Latitude,
		event.Venue.Longitude,
		event.TicketURL,
		event.TicketStatus,
		onSaleDate,
		event.ExternalIDs.BandsintownID,
		event.ExternalIDs.TicketmasterID,
		event.UpdatedAt,
		event.CachedUntil,
		event.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update event: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return domain.ErrEventNotFound
	}

	return nil
}

func (r *EventRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM events WHERE id = ?`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete event: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return domain.ErrEventNotFound
	}

	return nil
}

func (r *EventRepository) DeleteExpiredCache(ctx context.Context) error {
	query := `DELETE FROM events WHERE cached_until < ?`

	_, err := r.db.ExecContext(ctx, query, time.Now())
	if err != nil {
		return fmt.Errorf("failed to delete expired cache: %w", err)
	}

	return nil
}

func (r *EventRepository) scanEvent(row *sql.Row) (*domain.Event, error) {
	var event domain.Event
	var onSaleDate sql.NullTime

	err := row.Scan(
		&event.ID,
		&event.ArtistID,
		&event.ArtistName,
		&event.Title,
		&event.DateTime,
		&event.Venue.ID,
		&event.Venue.Name,
		&event.Venue.City,
		&event.Venue.Region,
		&event.Venue.Country,
		&event.Venue.Latitude,
		&event.Venue.Longitude,
		&event.TicketURL,
		&event.TicketStatus,
		&onSaleDate,
		&event.ExternalIDs.BandsintownID,
		&event.ExternalIDs.TicketmasterID,
		&event.CreatedAt,
		&event.UpdatedAt,
		&event.CachedUntil,
	)

	if err != nil {
		return nil, err
	}

	if onSaleDate.Valid {
		event.OnSaleDate = &onSaleDate.Time
	}

	return &event, nil
}

func (r *EventRepository) scanEvents(rows *sql.Rows) ([]domain.Event, error) {
	var events []domain.Event

	for rows.Next() {
		var event domain.Event
		var onSaleDate sql.NullTime

		err := rows.Scan(
			&event.ID,
			&event.ArtistID,
			&event.ArtistName,
			&event.Title,
			&event.DateTime,
			&event.Venue.ID,
			&event.Venue.Name,
			&event.Venue.City,
			&event.Venue.Region,
			&event.Venue.Country,
			&event.Venue.Latitude,
			&event.Venue.Longitude,
			&event.TicketURL,
			&event.TicketStatus,
			&onSaleDate,
			&event.ExternalIDs.BandsintownID,
			&event.ExternalIDs.TicketmasterID,
			&event.CreatedAt,
			&event.UpdatedAt,
			&event.CachedUntil,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}

		if onSaleDate.Valid {
			event.OnSaleDate = &onSaleDate.Time
		}

		events = append(events, event)
	}

	return events, rows.Err()
}

func (r *EventRepository) scanEventWithDistance(rows *sql.Rows) (*domain.Event, error) {
	var event domain.Event
	var onSaleDate sql.NullTime
	var distance float64

	err := rows.Scan(
		&event.ID,
		&event.ArtistID,
		&event.ArtistName,
		&event.Title,
		&event.DateTime,
		&event.Venue.ID,
		&event.Venue.Name,
		&event.Venue.City,
		&event.Venue.Region,
		&event.Venue.Country,
		&event.Venue.Latitude,
		&event.Venue.Longitude,
		&event.TicketURL,
		&event.TicketStatus,
		&onSaleDate,
		&event.ExternalIDs.BandsintownID,
		&event.ExternalIDs.TicketmasterID,
		&event.CreatedAt,
		&event.UpdatedAt,
		&event.CachedUntil,
		&distance,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to scan event with distance: %w", err)
	}

	if onSaleDate.Valid {
		event.OnSaleDate = &onSaleDate.Time
	}

	return &event, nil
}
