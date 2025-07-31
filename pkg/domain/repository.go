package domain

import (
	"context"
	"time"
)

type ArtistRepository interface {
	Create(ctx context.Context, artist *Artist) error
	GetByID(ctx context.Context, id string) (*Artist, error)
	GetByExternalID(ctx context.Context, externalID string, source string) (*Artist, error)
	Search(ctx context.Context, query string, limit int) ([]Artist, error)
	Update(ctx context.Context, artist *Artist) error
	Delete(ctx context.Context, id string) error
}

type ArtistService interface {
	SearchArtists(ctx context.Context, query string, limit int) (*ArtistSearchResponse, error)
	GetArtist(ctx context.Context, id string) (*Artist, error)
	SaveArtist(ctx context.Context, artist *Artist) error
}

type EventRepository interface {
	Create(ctx context.Context, event *Event) error
	CreateBatch(ctx context.Context, events []Event) error
	GetByID(ctx context.Context, id string) (*Event, error)
	GetByExternalID(ctx context.Context, externalID string, source string) (*Event, error)
	SearchByArtist(ctx context.Context, artistID string, startDate, endDate *time.Time) ([]Event, error)
	SearchByLocation(ctx context.Context, lat, lng float64, radius int, startDate, endDate *time.Time) ([]Event, error)
	Update(ctx context.Context, event *Event) error
	Delete(ctx context.Context, id string) error
	DeleteExpiredCache(ctx context.Context) error
}

type EventService interface {
	SearchArtistEvents(ctx context.Context, artistName string, location string, radius int) (*EventSearchResponse, error)
	GetArtistEvents(ctx context.Context, artistID string) (*EventSearchResponse, error)
	GetEvent(ctx context.Context, id string) (*Event, error)
}
