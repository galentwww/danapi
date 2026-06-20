package danmaku

import (
	"context"
	"errors"
	"net/url"
	"reflect"
	"sync"
	"testing"
	"time"

	"dandanplay-middleware/storage"
)

func testPolicy() RefreshPolicy {
	return RefreshPolicy{
		DefaultRefreshInterval:       24 * time.Hour,
		EmptyCommentsRefreshInterval: time.Hour,
		RefreshFailureRetryInterval:  30 * time.Minute,
	}
}

func compressedSnapshot(t *testing.T, id int64, variantKey string, body []byte, nextRefreshAt time.Time) *storage.Snapshot {
	t.Helper()
	info, err := ValidatePayload(body)
	if err != nil {
		t.Fatalf("ValidatePayload returned error: %v", err)
	}
	compressed, err := GzipPayload(body)
	if err != nil {
		t.Fatalf("GzipPayload returned error: %v", err)
	}
	return &storage.Snapshot{
		DandanEpisodeID:   id,
		VariantKey:        variantKey,
		Payload:           compressed,
		PayloadEncoding:   "gzip",
		FetchedAt:         time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC),
		NextRefreshAt:     nextRefreshAt,
		CommentCount:      info.CommentCount,
		ContentHash:       info.ContentHash,
		LastRefreshStatus: "success",
		Version:           1,
	}
}

func TestCommentServiceRedisHitReturnsCachedPayload(t *testing.T) {
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	body := []byte(`{"count":1,"comments":[{"cid":1,"p":"0.00,1,16777215,abc","m":"one"}]}`)
	cache := &fakeSnapshotCache{
		getSnapshot: compressedSnapshot(t, 123, "v1|withRelated=1", body, now.Add(time.Hour)),
		getStatus:   CacheHit,
	}
	store := &fakeSnapshotStore{}
	upstream := &fakeUpstreamClient{}
	service := NewCommentService(CommentServiceOptions{
		Cache:              cache,
		Store:              store,
		Upstream:           upstream,
		Policy:             testPolicy(),
		RedisSnapshotTTL:   48 * time.Hour,
		RefreshQueueSize:   10,
		RefreshWorkerCount: 0,
		Now:                func() time.Time { return now },
	})

	result, err := service.GetComments(context.Background(), "123", mustQuery(t, "withRelated=true"))
	if err != nil {
		t.Fatalf("GetComments returned error: %v", err)
	}

	if string(result.Payload) != string(body) {
		t.Fatalf("payload = %s", result.Payload)
	}
	if result.CacheStatus != "redis" {
		t.Fatalf("CacheStatus = %q", result.CacheStatus)
	}
	if result.VariantKey != "v1|withRelated=1" {
		t.Fatalf("VariantKey = %q", result.VariantKey)
	}
	if store.getCalls != 0 {
		t.Fatalf("store get calls = %d", store.getCalls)
	}
	if upstream.calls != 0 {
		t.Fatalf("upstream calls = %d", upstream.calls)
	}
}

func TestCommentServicePostgresHitRefillsRedis(t *testing.T) {
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	body := []byte(`{"count":1,"comments":[{"cid":1,"p":"0.00,1,16777215,abc","m":"one"}]}`)
	snapshot := compressedSnapshot(t, 123, "v1|withRelated=1", body, now.Add(time.Hour))
	cache := &fakeSnapshotCache{getStatus: CacheMiss}
	store := &fakeSnapshotStore{snapshot: snapshot}
	upstream := &fakeUpstreamClient{}
	service := NewCommentService(CommentServiceOptions{
		Cache:              cache,
		Store:              store,
		Upstream:           upstream,
		Policy:             testPolicy(),
		RedisSnapshotTTL:   48 * time.Hour,
		RefreshQueueSize:   10,
		RefreshWorkerCount: 0,
		Now:                func() time.Time { return now },
	})

	result, err := service.GetComments(context.Background(), "123", mustQuery(t, "withRelated=true"))
	if err != nil {
		t.Fatalf("GetComments returned error: %v", err)
	}

	if string(result.Payload) != string(body) {
		t.Fatalf("payload = %s", result.Payload)
	}
	if result.CacheStatus != "postgres" {
		t.Fatalf("CacheStatus = %q", result.CacheStatus)
	}
	if len(cache.setSnapshots) != 1 {
		t.Fatalf("cache set count = %d", len(cache.setSnapshots))
	}
	if upstream.calls != 0 {
		t.Fatalf("upstream calls = %d", upstream.calls)
	}
}

func TestCommentServiceFirstLoadWritesStoreBeforeRedis(t *testing.T) {
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	body := []byte(`{"count":1,"comments":[{"cid":1,"p":"0.00,1,16777215,abc","m":"one"}]}`)
	ops := []string{}
	cache := &fakeSnapshotCache{
		getStatus: CacheMiss,
		onSet: func() {
			ops = append(ops, "cache")
		},
	}
	store := &fakeSnapshotStore{
		getErr: storage.ErrSnapshotNotFound,
		onUpsert: func() {
			ops = append(ops, "store")
		},
	}
	upstream := &fakeUpstreamClient{payload: body}
	service := NewCommentService(CommentServiceOptions{
		Cache:              cache,
		Store:              store,
		Upstream:           upstream,
		Policy:             testPolicy(),
		RedisSnapshotTTL:   48 * time.Hour,
		RefreshQueueSize:   10,
		RefreshWorkerCount: 0,
		Now:                func() time.Time { return now },
	})

	result, err := service.GetComments(context.Background(), "123", mustQuery(t, "withRelated=true"))
	if err != nil {
		t.Fatalf("GetComments returned error: %v", err)
	}

	if string(result.Payload) != string(body) {
		t.Fatalf("payload = %s", result.Payload)
	}
	if result.CacheStatus != "upstream" {
		t.Fatalf("CacheStatus = %q", result.CacheStatus)
	}
	if !reflect.DeepEqual(ops, []string{"store", "cache"}) {
		t.Fatalf("ops = %#v", ops)
	}
	if len(store.upserts) != 1 {
		t.Fatalf("upsert count = %d", len(store.upserts))
	}
	if store.upserts[0].NextRefreshAt != now.Add(24*time.Hour) {
		t.Fatalf("NextRefreshAt = %s", store.upserts[0].NextRefreshAt)
	}
}

func TestCommentServiceNoSnapshotUpstreamFailure(t *testing.T) {
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	cache := &fakeSnapshotCache{getStatus: CacheMiss}
	store := &fakeSnapshotStore{getErr: storage.ErrSnapshotNotFound}
	upstream := &fakeUpstreamClient{err: errors.New("upstream unavailable")}
	service := NewCommentService(CommentServiceOptions{
		Cache:              cache,
		Store:              store,
		Upstream:           upstream,
		Policy:             testPolicy(),
		RedisSnapshotTTL:   48 * time.Hour,
		RefreshQueueSize:   10,
		RefreshWorkerCount: 0,
		Now:                func() time.Time { return now },
	})

	_, err := service.GetComments(context.Background(), "123", mustQuery(t, "withRelated=true"))
	if !errors.Is(err, ErrUpstreamUnavailable) {
		t.Fatalf("error = %v", err)
	}
}

func TestCommentServiceStaleSnapshotReturnsWhileRefreshFails(t *testing.T) {
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	body := []byte(`{"count":1,"comments":[{"cid":1,"p":"0.00,1,16777215,abc","m":"one"}]}`)
	stale := compressedSnapshot(t, 123, "v1|withRelated=1", body, now.Add(-time.Minute))
	cache := &fakeSnapshotCache{getStatus: CacheMiss}
	store := &fakeSnapshotStore{
		snapshot:  stale,
		markError: make(chan struct{}),
	}
	upstream := &fakeUpstreamClient{err: errors.New("upstream unavailable")}
	service := NewCommentService(CommentServiceOptions{
		Cache:              cache,
		Store:              store,
		Upstream:           upstream,
		Policy:             testPolicy(),
		RedisSnapshotTTL:   48 * time.Hour,
		RefreshQueueSize:   10,
		RefreshWorkerCount: 1,
		Now:                func() time.Time { return now },
	})
	defer service.Close()

	result, err := service.GetComments(context.Background(), "123", mustQuery(t, "withRelated=true"))
	if err != nil {
		t.Fatalf("GetComments returned error: %v", err)
	}
	if string(result.Payload) != string(body) {
		t.Fatalf("payload = %s", result.Payload)
	}
	if result.CacheStatus != "stale" {
		t.Fatalf("CacheStatus = %q", result.CacheStatus)
	}

	select {
	case <-store.markError:
	case <-time.After(2 * time.Second):
		t.Fatal("refresh error was not marked")
	}
}

func mustQuery(t *testing.T, raw string) url.Values {
	t.Helper()
	values, err := url.ParseQuery(raw)
	if err != nil {
		t.Fatalf("parse query: %v", err)
	}
	return values
}

type fakeSnapshotCache struct {
	getSnapshot  *storage.Snapshot
	getStatus    CacheStatus
	getErr       error
	setSnapshots []*storage.Snapshot
	onSet        func()
}

func (c *fakeSnapshotCache) Get(ctx context.Context, dandanEpisodeID int64, variantKey string) (*storage.Snapshot, CacheStatus, error) {
	return c.getSnapshot, c.getStatus, c.getErr
}

func (c *fakeSnapshotCache) Set(ctx context.Context, snapshot *storage.Snapshot, ttl time.Duration) error {
	if c.onSet != nil {
		c.onSet()
	}
	c.setSnapshots = append(c.setSnapshots, snapshot)
	return nil
}

func (c *fakeSnapshotCache) Delete(ctx context.Context, dandanEpisodeID int64, variantKey string) error {
	return nil
}

type fakeSnapshotStore struct {
	mu        sync.Mutex
	snapshot  *storage.Snapshot
	getErr    error
	getCalls  int
	upserts   []*storage.Snapshot
	onUpsert  func()
	markError chan struct{}
}

func (s *fakeSnapshotStore) Get(ctx context.Context, dandanEpisodeID int64, variantKey string) (*storage.Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.getCalls++
	if s.getErr != nil {
		return nil, s.getErr
	}
	if s.snapshot == nil {
		return nil, storage.ErrSnapshotNotFound
	}
	return s.snapshot, nil
}

func (s *fakeSnapshotStore) Upsert(ctx context.Context, snapshot *storage.Snapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.onUpsert != nil {
		s.onUpsert()
	}
	s.upserts = append(s.upserts, snapshot)
	s.snapshot = snapshot
	return nil
}

func (s *fakeSnapshotStore) MarkRefreshError(ctx context.Context, dandanEpisodeID int64, variantKey string, retryAt time.Time, message string) error {
	if s.markError != nil {
		close(s.markError)
		s.markError = nil
	}
	return nil
}

type fakeUpstreamClient struct {
	payload []byte
	err     error
	calls   int
}

func (c *fakeUpstreamClient) FetchComments(ctx context.Context, dandanEpisodeID string, query string) ([]byte, error) {
	c.calls++
	if c.err != nil {
		return nil, c.err
	}
	return c.payload, nil
}
