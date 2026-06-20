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
		EmptyDanmakuRefreshInterval:  time.Hour,
		RefreshFailureRetryInterval:  30 * time.Minute,
		HotAccessThreshold:           10,
		HotChangedRefreshInterval:    2 * time.Hour,
		HotUnchangedRefreshInterval:  6 * time.Hour,
		NormalChangedRefreshInterval: 12 * time.Hour,
		StableRefreshInterval:        72 * time.Hour,
		ArchivedRefreshInterval:      168 * time.Hour,
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
		DanmakuCount:      info.DanmakuCount,
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
	if len(store.accessRecords) != 1 {
		t.Fatalf("access record count = %d", len(store.accessRecords))
	}
	if store.accessRecords[0].dandanEpisodeID != 123 {
		t.Fatalf("access dandanEpisodeID = %d", store.accessRecords[0].dandanEpisodeID)
	}
	if store.accessRecords[0].variantKey != "v1|withRelated=1" {
		t.Fatalf("access variantKey = %q", store.accessRecords[0].variantKey)
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
	if len(store.accessRecords) != 1 {
		t.Fatalf("access record count = %d", len(store.accessRecords))
	}
}

func TestCommentServiceStaleRedisHitDeletesCachedSnapshot(t *testing.T) {
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	body := []byte(`{"count":1,"comments":[{"cid":1,"p":"0.00,1,16777215,abc","m":"one"}]}`)
	stale := compressedSnapshot(t, 123, "v1|withRelated=1", body, now.Add(-time.Minute))
	cache := &fakeSnapshotCache{
		getSnapshot: stale,
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

	if result.CacheStatus != "stale" {
		t.Fatalf("CacheStatus = %q", result.CacheStatus)
	}
	if cache.deleteCalls != 1 {
		t.Fatalf("cache delete calls = %d", cache.deleteCalls)
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
	if store.upserts[0].NextRefreshAt != now.Add(12*time.Hour) {
		t.Fatalf("NextRefreshAt = %s", store.upserts[0].NextRefreshAt)
	}
	if store.upserts[0].AccessCount != 1 {
		t.Fatalf("AccessCount = %d", store.upserts[0].AccessCount)
	}
	if store.upserts[0].RecentAccessCount != 1 {
		t.Fatalf("RecentAccessCount = %d", store.upserts[0].RecentAccessCount)
	}
	if !store.upserts[0].LastAccessedAt.Equal(now) {
		t.Fatalf("LastAccessedAt = %s", store.upserts[0].LastAccessedAt)
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

func TestCommentServiceStalePostgresHitDoesNotRefillRedis(t *testing.T) {
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	body := []byte(`{"count":1,"comments":[{"cid":1,"p":"0.00,1,16777215,abc","m":"one"}]}`)
	stale := compressedSnapshot(t, 123, "v1|withRelated=1", body, now.Add(-time.Minute))
	cache := &fakeSnapshotCache{getStatus: CacheMiss}
	store := &fakeSnapshotStore{snapshot: stale}
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

	if result.CacheStatus != "stale" {
		t.Fatalf("CacheStatus = %q", result.CacheStatus)
	}
	if len(cache.setSnapshots) != 0 {
		t.Fatalf("cache set count = %d", len(cache.setSnapshots))
	}
}

func TestCommentServiceDuplicateStaleRequestsEnqueueOneRefresh(t *testing.T) {
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	body := []byte(`{"count":1,"comments":[{"cid":1,"p":"0.00,1,16777215,abc","m":"one"}]}`)
	stale := compressedSnapshot(t, 123, "v1|withRelated=1", body, now.Add(-time.Minute))
	cache := &fakeSnapshotCache{getStatus: CacheMiss}
	store := &fakeSnapshotStore{snapshot: stale}
	upstream := &fakeUpstreamClient{}
	service := NewCommentService(CommentServiceOptions{
		Cache:              cache,
		Store:              store,
		Upstream:           upstream,
		Policy:             testPolicy(),
		RedisSnapshotTTL:   48 * time.Hour,
		RefreshQueueSize:   100,
		RefreshWorkerCount: 0,
		Now:                func() time.Time { return now },
	})

	for i := 0; i < 100; i++ {
		result, err := service.GetComments(context.Background(), "123", mustQuery(t, "withRelated=true"))
		if err != nil {
			t.Fatalf("GetComments request %d returned error: %v", i, err)
		}
		if result.CacheStatus != "stale" {
			t.Fatalf("request %d CacheStatus = %q", i, result.CacheStatus)
		}
	}

	if got := len(service.refreshQueue); got != 1 {
		t.Fatalf("queued refresh count = %d", got)
	}
}

func TestCommentServiceRefreshSkipsUpstreamWhenSnapshotIsAlreadyFresh(t *testing.T) {
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	body := []byte(`{"count":1,"comments":[{"cid":1,"p":"0.00,1,16777215,abc","m":"one"}]}`)
	fresh := compressedSnapshot(t, 123, "v1|withRelated=1", body, now.Add(time.Hour))
	cache := &fakeSnapshotCache{}
	store := &fakeSnapshotStore{snapshot: fresh}
	upstream := &fakeUpstreamClient{payload: []byte(`{"count":1,"comments":[{"cid":2,"p":"1.00,1,16777215,def","m":"two"}]}`)}
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

	service.refresh(context.Background(), refreshRequest{
		dandanEpisodeID: 123,
		episodeIDString: "123",
		variant:         NormalizeVariant(mustQuery(t, "withRelated=true")),
	})

	if upstream.calls != 0 {
		t.Fatalf("upstream calls = %d", upstream.calls)
	}
	if len(store.upserts) != 0 {
		t.Fatalf("upsert count = %d", len(store.upserts))
	}
	if len(cache.setSnapshots) != 0 {
		t.Fatalf("cache set count = %d", len(cache.setSnapshots))
	}
}

func TestCommentServiceRefreshUsesHotChangedInterval(t *testing.T) {
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	oldBody := []byte(`{"count":1,"comments":[{"cid":1,"p":"0.00,1,16777215,abc","m":"one"}]}`)
	newBody := []byte(`{"count":2,"comments":[{"cid":1,"p":"0.00,1,16777215,abc","m":"one"},{"cid":2,"p":"1.00,1,16777215,def","m":"two"}]}`)
	stale := compressedSnapshot(t, 123, "v1|withRelated=1", oldBody, now.Add(-time.Minute))
	stale.RecentAccessCount = 10
	cache := &fakeSnapshotCache{}
	store := &fakeSnapshotStore{snapshot: stale}
	upstream := &fakeUpstreamClient{payload: newBody}
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

	service.refresh(context.Background(), refreshRequest{
		dandanEpisodeID: 123,
		episodeIDString: "123",
		variant:         NormalizeVariant(mustQuery(t, "withRelated=true")),
	})

	if len(store.upserts) != 1 {
		t.Fatalf("upsert count = %d", len(store.upserts))
	}
	if !store.upserts[0].NextRefreshAt.Equal(now.Add(2 * time.Hour)) {
		t.Fatalf("NextRefreshAt = %s", store.upserts[0].NextRefreshAt)
	}
	if store.upserts[0].UnchangedStreak != 0 {
		t.Fatalf("UnchangedStreak = %d", store.upserts[0].UnchangedStreak)
	}
}

func TestCommentServiceRefreshUsesArchivedIntervalAfterRepeatedUnchangedPayloads(t *testing.T) {
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	body := []byte(`{"count":1,"comments":[{"cid":1,"p":"0.00,1,16777215,abc","m":"one"}]}`)
	stale := compressedSnapshot(t, 123, "v1|withRelated=1", body, now.Add(-time.Minute))
	stale.RecentAccessCount = 100
	stale.UnchangedStreak = 6
	cache := &fakeSnapshotCache{}
	store := &fakeSnapshotStore{snapshot: stale}
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

	service.refresh(context.Background(), refreshRequest{
		dandanEpisodeID: 123,
		episodeIDString: "123",
		variant:         NormalizeVariant(mustQuery(t, "withRelated=true")),
	})

	if len(store.upserts) != 1 {
		t.Fatalf("upsert count = %d", len(store.upserts))
	}
	if !store.upserts[0].NextRefreshAt.Equal(now.Add(168 * time.Hour)) {
		t.Fatalf("NextRefreshAt = %s", store.upserts[0].NextRefreshAt)
	}
	if store.upserts[0].UnchangedStreak != 7 {
		t.Fatalf("UnchangedStreak = %d", store.upserts[0].UnchangedStreak)
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
	deleteCalls  int
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
	c.deleteCalls++
	return nil
}

type fakeSnapshotStore struct {
	mu            sync.Mutex
	snapshot      *storage.Snapshot
	getErr        error
	getCalls      int
	upserts       []*storage.Snapshot
	accessRecords []accessRecord
	onUpsert      func()
	markError     chan struct{}
}

type accessRecord struct {
	dandanEpisodeID int64
	variantKey      string
	accessedAt      time.Time
	window          time.Duration
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

func (s *fakeSnapshotStore) RecordAccess(ctx context.Context, dandanEpisodeID int64, variantKey string, accessedAt time.Time, window time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.accessRecords = append(s.accessRecords, accessRecord{
		dandanEpisodeID: dandanEpisodeID,
		variantKey:      variantKey,
		accessedAt:      accessedAt,
		window:          window,
	})
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
