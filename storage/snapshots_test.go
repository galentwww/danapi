package storage

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

func openTestStore(t *testing.T) (*PostgresSnapshotStore, func()) {
	t.Helper()

	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	ctx := context.Background()
	db, err := OpenPostgres(ctx, databaseURL)
	if err != nil {
		t.Fatalf("OpenPostgres returned error: %v", err)
	}

	if err := Migrate(ctx, db); err != nil {
		db.Close()
		t.Fatalf("Migrate returned error: %v", err)
	}

	store := NewPostgresSnapshotStore(db)
	_, err = db.Exec(ctx, "truncate table danmaku_snapshots")
	if err != nil {
		db.Close()
		t.Fatalf("truncate snapshots: %v", err)
	}

	return store, db.Close
}

func TestPostgresSnapshotStoreUpsertAndGet(t *testing.T) {
	store, cleanup := openTestStore(t)
	defer cleanup()

	ctx := context.Background()
	fetchedAt := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	nextRefreshAt := fetchedAt.Add(24 * time.Hour)
	snapshot := &Snapshot{
		DandanEpisodeID:     123,
		VariantKey:          "v1|withRelated=1",
		Payload:             []byte("compressed"),
		PayloadEncoding:     "gzip",
		FetchedAt:           fetchedAt,
		NextRefreshAt:       nextRefreshAt,
		CommentCount:        2,
		ContentHash:         "hash",
		UnchangedStreak:     0,
		Version:             1,
		LastRefreshStatus:   "success",
		RefreshErrorMessage: "",
	}

	if err := store.Upsert(ctx, snapshot); err != nil {
		t.Fatalf("Upsert returned error: %v", err)
	}

	got, err := store.Get(ctx, 123, "v1|withRelated=1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}

	if got.DandanEpisodeID != snapshot.DandanEpisodeID {
		t.Fatalf("DandanEpisodeID = %d", got.DandanEpisodeID)
	}
	if got.VariantKey != snapshot.VariantKey {
		t.Fatalf("VariantKey = %q", got.VariantKey)
	}
	if string(got.Payload) != "compressed" {
		t.Fatalf("Payload = %q", got.Payload)
	}
	if got.CommentCount != 2 {
		t.Fatalf("CommentCount = %d", got.CommentCount)
	}
}

func TestPostgresSnapshotStoreGetNotFound(t *testing.T) {
	store, cleanup := openTestStore(t)
	defer cleanup()

	_, err := store.Get(context.Background(), 404, "v1|withRelated=1")
	if !errors.Is(err, ErrSnapshotNotFound) {
		t.Fatalf("Get error = %v", err)
	}
}

func TestPostgresSnapshotStoreMarkRefreshError(t *testing.T) {
	store, cleanup := openTestStore(t)
	defer cleanup()

	ctx := context.Background()
	fetchedAt := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	snapshot := &Snapshot{
		DandanEpisodeID:   123,
		VariantKey:        "v1|withRelated=1",
		Payload:           []byte("compressed"),
		PayloadEncoding:   "gzip",
		FetchedAt:         fetchedAt,
		NextRefreshAt:     fetchedAt.Add(24 * time.Hour),
		CommentCount:      2,
		ContentHash:       "hash",
		LastRefreshStatus: "success",
	}
	if err := store.Upsert(ctx, snapshot); err != nil {
		t.Fatalf("Upsert returned error: %v", err)
	}

	retryAt := fetchedAt.Add(30 * time.Minute)
	if err := store.MarkRefreshError(ctx, 123, "v1|withRelated=1", retryAt, "upstream failed"); err != nil {
		t.Fatalf("MarkRefreshError returned error: %v", err)
	}

	got, err := store.Get(ctx, 123, "v1|withRelated=1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.LastRefreshStatus != "error" {
		t.Fatalf("LastRefreshStatus = %q", got.LastRefreshStatus)
	}
	if got.RefreshErrorMessage != "upstream failed" {
		t.Fatalf("RefreshErrorMessage = %q", got.RefreshErrorMessage)
	}
	if !got.NextRefreshAt.Equal(retryAt) {
		t.Fatalf("NextRefreshAt = %s", got.NextRefreshAt)
	}
}
