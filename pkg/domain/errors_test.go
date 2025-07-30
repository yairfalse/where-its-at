package domain

import (
	"errors"
	"testing"
)

func TestErrors(t *testing.T) {
	t.Run("Predefined errors", func(t *testing.T) {
		tests := []struct {
			name string
			err  error
			want string
		}{
			{"ErrArtistNotFound", ErrArtistNotFound, "artist not found"},
			{"ErrInvalidRequest", ErrInvalidRequest, "invalid request"},
			{"ErrDuplicateArtist", ErrDuplicateArtist, "artist already exists"},
			{"ErrExternalAPIFailure", ErrExternalAPIFailure, "external API failure"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if got := tt.err.Error(); got != tt.want {
					t.Errorf("%s.Error() = %v, want %v", tt.name, got, tt.want)
				}
			})
		}
	})

	t.Run("ValidationError", func(t *testing.T) {
		err := ValidationError{
			Field:   "name",
			Message: "name is required",
		}

		expected := "validation error on field name: name is required"
		if got := err.Error(); got != expected {
			t.Errorf("ValidationError.Error() = %v, want %v", got, expected)
		}
	})

	t.Run("Error identity", func(t *testing.T) {
		if !errors.Is(ErrArtistNotFound, ErrArtistNotFound) {
			t.Error("ErrArtistNotFound should be equal to itself")
		}

		if errors.Is(ErrArtistNotFound, ErrInvalidRequest) {
			t.Error("ErrArtistNotFound should not be equal to ErrInvalidRequest")
		}
	})
}
