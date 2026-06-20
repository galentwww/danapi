package danmaku

import (
	"testing"
	"time"

	"dandanplay-middleware/storage"
)

func TestSnapshotCacheKeyUsesVariantHash(t *testing.T) {
	key := SnapshotCacheKey(123, "v1|withRelated=1")

	if key != "ddp:comments:v1:123:f3da3fc977253d94" {
		t.Fatalf("SnapshotCacheKey = %q", key)
	}
}

func TestCacheEnvelopeRoundTrip(t *testing.T) {
	fetchedAt := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	snapshot := &storage.Snapshot{
		DandanEpisodeID:   123,
		VariantKey:        "v1|withRelated=1",
		Payload:           []byte("compressed"),
		PayloadEncoding:   "gzip",
		FetchedAt:         fetchedAt,
		NextRefreshAt:     fetchedAt.Add(24 * time.Hour),
		CommentCount:      2,
		ContentHash:       "hash",
		UnchangedStreak:   1,
		Version:           7,
		LastRefreshStatus: "success",
	}

	encoded, err := encodeCacheEnvelope(snapshot)
	if err != nil {
		t.Fatalf("encodeCacheEnvelope returned error: %v", err)
	}

	got, status, err := decodeCacheEnvelope(encoded)
	if err != nil {
		t.Fatalf("decodeCacheEnvelope returned error: %v", err)
	}
	if status != CacheHit {
		t.Fatalf("status = %v", status)
	}
	if got.DandanEpisodeID != snapshot.DandanEpisodeID {
		t.Fatalf("DandanEpisodeID = %d", got.DandanEpisodeID)
	}
	if string(got.Payload) != "compressed" {
		t.Fatalf("Payload = %q", got.Payload)
	}
	if got.Version != 7 {
		t.Fatalf("Version = %d", got.Version)
	}
}

func TestDecodeCacheEnvelopeReportsCorruption(t *testing.T) {
	_, status, err := decodeCacheEnvelope([]byte(`{"payload":`))
	if err == nil {
		t.Fatal("decodeCacheEnvelope returned nil error")
	}
	if status != CacheCorrupted {
		t.Fatalf("status = %v", status)
	}
}
