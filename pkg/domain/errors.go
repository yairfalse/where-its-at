package domain

import (
	"errors"
	"fmt"
)

var (
	ErrArtistNotFound     = errors.New("artist not found")
	ErrInvalidRequest     = errors.New("invalid request")
	ErrDuplicateArtist    = errors.New("artist already exists")
	ErrExternalAPIFailure = errors.New("external API failure")
)

type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation error on field %s: %s", e.Field, e.Message)
}
