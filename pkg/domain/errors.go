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
	ErrEventNotFound      = errors.New("event not found")
	ErrDuplicateEvent     = errors.New("event already exists")
	ErrInvalidLocation    = errors.New("invalid location")
	ErrRateLimitExceeded  = errors.New("rate limit exceeded")
)

type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation error on field %s: %s", e.Field, e.Message)
}
