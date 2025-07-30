package collectors

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/yair/where-its-at/pkg/domain"
)

func setupTestDB(t *testing.T) (*sql.DB, func()) {
	tempFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	db, err := NewSQLiteDB(tempFile.Name())
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}

	cleanup := func() {
		db.Close()
		os.Remove(tempFile.Name())
	}

	return db, cleanup
}

func TestNewArtistRepository(t *testing.T) {
	t.Run("successful creation", func(t *testing.T) {
		db, cleanup := setupTestDB(t)
		defer cleanup()

		repo, err := NewArtistRepository(db)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if repo == nil {
			t.Fatal("expected repository, got nil")
		}
	})

	t.Run("nil database", func(t *testing.T) {
		_, err := NewArtistRepository(nil)
		if err == nil {
			t.Fatal("expected error for nil database")
		}
	})
}

func TestArtistRepository_Create(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, err := NewArtistRepository(db)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}

	ctx := context.Background()

	t.Run("successful creation", func(t *testing.T) {
		artist := &domain.Artist{
			ID:   "test-123",
			Name: "Test Artist",
			ExternalIDs: domain.ExternalIDs{
				SpotifyID: "spotify123",
				LastFMID:  "lastfm123",
			},
			Genres:     []string{"rock", "alternative"},
			Popularity: 75,
			ImageURL:   "https://example.com/image.jpg",
		}

		err := repo.Create(ctx, artist)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if artist.CreatedAt.IsZero() {
			t.Error("expected CreatedAt to be set")
		}
		if artist.UpdatedAt.IsZero() {
			t.Error("expected UpdatedAt to be set")
		}
	})

	t.Run("duplicate artist", func(t *testing.T) {
		artist := &domain.Artist{
			ID:   "test-123",
			Name: "Another Artist",
		}

		err := repo.Create(ctx, artist)
		if err != domain.ErrDuplicateArtist {
			t.Errorf("expected ErrDuplicateArtist, got %v", err)
		}
	})

	t.Run("nil artist", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		if err == nil {
			t.Error("expected error for nil artist")
		}
	})
}

func TestArtistRepository_GetByID(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, err := NewArtistRepository(db)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}

	ctx := context.Background()

	artist := &domain.Artist{
		ID:   "test-123",
		Name: "Test Artist",
		ExternalIDs: domain.ExternalIDs{
			SpotifyID: "spotify123",
			LastFMID:  "lastfm123",
		},
		Genres:     []string{"rock", "alternative"},
		Popularity: 75,
		ImageURL:   "https://example.com/image.jpg",
	}

	err = repo.Create(ctx, artist)
	if err != nil {
		t.Fatalf("failed to create artist: %v", err)
	}

	t.Run("existing artist", func(t *testing.T) {
		found, err := repo.GetByID(ctx, "test-123")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if found.ID != artist.ID {
			t.Errorf("expected ID %s, got %s", artist.ID, found.ID)
		}
		if found.Name != artist.Name {
			t.Errorf("expected Name %s, got %s", artist.Name, found.Name)
		}
		if found.ExternalIDs.SpotifyID != artist.ExternalIDs.SpotifyID {
			t.Errorf("expected SpotifyID %s, got %s", artist.ExternalIDs.SpotifyID, found.ExternalIDs.SpotifyID)
		}
		if len(found.Genres) != len(artist.Genres) {
			t.Errorf("expected %d genres, got %d", len(artist.Genres), len(found.Genres))
		}
	})

	t.Run("non-existing artist", func(t *testing.T) {
		_, err := repo.GetByID(ctx, "non-existing")
		if err != domain.ErrArtistNotFound {
			t.Errorf("expected ErrArtistNotFound, got %v", err)
		}
	})
}

func TestArtistRepository_GetByExternalID(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, err := NewArtistRepository(db)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}

	ctx := context.Background()

	artist := &domain.Artist{
		ID:   "test-123",
		Name: "Test Artist",
		ExternalIDs: domain.ExternalIDs{
			SpotifyID: "spotify123",
			LastFMID:  "lastfm123",
		},
	}

	err = repo.Create(ctx, artist)
	if err != nil {
		t.Fatalf("failed to create artist: %v", err)
	}

	t.Run("spotify ID", func(t *testing.T) {
		found, err := repo.GetByExternalID(ctx, "spotify123", "spotify")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if found.ID != artist.ID {
			t.Errorf("expected ID %s, got %s", artist.ID, found.ID)
		}
	})

	t.Run("lastfm ID", func(t *testing.T) {
		found, err := repo.GetByExternalID(ctx, "lastfm123", "lastfm")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if found.ID != artist.ID {
			t.Errorf("expected ID %s, got %s", artist.ID, found.ID)
		}
	})

	t.Run("invalid source", func(t *testing.T) {
		_, err := repo.GetByExternalID(ctx, "123", "invalid")
		if err == nil {
			t.Error("expected error for invalid source")
		}
	})
}

func TestArtistRepository_Search(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, err := NewArtistRepository(db)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}

	ctx := context.Background()

	artists := []*domain.Artist{
		{ID: "1", Name: "Radiohead", Popularity: 90},
		{ID: "2", Name: "The Radio Dept.", Popularity: 60},
		{ID: "3", Name: "Radio Moscow", Popularity: 50},
		{ID: "4", Name: "Portishead", Popularity: 70},
	}

	for _, a := range artists {
		if err := repo.Create(ctx, a); err != nil {
			t.Fatalf("failed to create artist: %v", err)
		}
	}

	t.Run("search with results", func(t *testing.T) {
		results, err := repo.Search(ctx, "radio", 10)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(results) != 3 {
			t.Errorf("expected 3 results, got %d", len(results))
		}
		if results[0].Name != "Radiohead" {
			t.Errorf("expected first result to be Radiohead, got %s", results[0].Name)
		}
	})

	t.Run("search with limit", func(t *testing.T) {
		results, err := repo.Search(ctx, "radio", 2)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(results) != 2 {
			t.Errorf("expected 2 results, got %d", len(results))
		}
	})

	t.Run("search with no results", func(t *testing.T) {
		results, err := repo.Search(ctx, "xyz", 10)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(results) != 0 {
			t.Errorf("expected 0 results, got %d", len(results))
		}
	})
}

func TestArtistRepository_Update(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, err := NewArtistRepository(db)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}

	ctx := context.Background()

	artist := &domain.Artist{
		ID:         "test-123",
		Name:       "Test Artist",
		Popularity: 50,
	}

	err = repo.Create(ctx, artist)
	if err != nil {
		t.Fatalf("failed to create artist: %v", err)
	}

	t.Run("successful update", func(t *testing.T) {
		artist.Name = "Updated Artist"
		artist.Popularity = 75
		artist.Genres = []string{"rock"}

		err := repo.Update(ctx, artist)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		updated, err := repo.GetByID(ctx, artist.ID)
		if err != nil {
			t.Fatalf("failed to get updated artist: %v", err)
		}

		if updated.Name != "Updated Artist" {
			t.Errorf("expected name to be Updated Artist, got %s", updated.Name)
		}
		if updated.Popularity != 75 {
			t.Errorf("expected popularity to be 75, got %d", updated.Popularity)
		}
		if len(updated.Genres) != 1 || updated.Genres[0] != "rock" {
			t.Errorf("expected genres to be [rock], got %v", updated.Genres)
		}
	})

	t.Run("update non-existing artist", func(t *testing.T) {
		nonExisting := &domain.Artist{
			ID:   "non-existing",
			Name: "Ghost Artist",
		}
		err := repo.Update(ctx, nonExisting)
		if err != domain.ErrArtistNotFound {
			t.Errorf("expected ErrArtistNotFound, got %v", err)
		}
	})

	t.Run("nil artist", func(t *testing.T) {
		err := repo.Update(ctx, nil)
		if err == nil {
			t.Error("expected error for nil artist")
		}
	})
}

func TestArtistRepository_Delete(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	repo, err := NewArtistRepository(db)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}

	ctx := context.Background()

	artist := &domain.Artist{
		ID:   "test-123",
		Name: "Test Artist",
	}

	err = repo.Create(ctx, artist)
	if err != nil {
		t.Fatalf("failed to create artist: %v", err)
	}

	t.Run("successful delete", func(t *testing.T) {
		err := repo.Delete(ctx, "test-123")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		_, err = repo.GetByID(ctx, "test-123")
		if err != domain.ErrArtistNotFound {
			t.Errorf("expected ErrArtistNotFound after delete, got %v", err)
		}
	})

	t.Run("delete non-existing artist", func(t *testing.T) {
		err := repo.Delete(ctx, "non-existing")
		if err != domain.ErrArtistNotFound {
			t.Errorf("expected ErrArtistNotFound, got %v", err)
		}
	})
}
