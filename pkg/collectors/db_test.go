package collectors

import (
	"os"
	"testing"
)

func TestNewSQLiteDB(t *testing.T) {
	t.Run("successful creation", func(t *testing.T) {
		tempFile, err := os.CreateTemp("", "test-*.db")
		if err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
		defer os.Remove(tempFile.Name())

		db, err := NewSQLiteDB(tempFile.Name())
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		defer db.Close()

		if db == nil {
			t.Fatal("expected database connection, got nil")
		}

		err = db.Ping()
		if err != nil {
			t.Errorf("expected successful ping, got %v", err)
		}
	})

	t.Run("invalid path", func(t *testing.T) {
		_, err := NewSQLiteDB("/invalid/path/to/database.db")
		if err == nil {
			t.Error("expected error for invalid path")
		}
	})
}
