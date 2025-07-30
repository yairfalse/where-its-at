package domain

import (
	"context"
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
