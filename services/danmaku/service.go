package danmaku

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"sync"
	"time"

	"dandanplay-middleware/storage"

	"golang.org/x/sync/singleflight"
)

var ErrUpstreamUnavailable = errors.New("upstream unavailable and no danmaku snapshot exists")

type SnapshotStore interface {
	Get(ctx context.Context, dandanEpisodeID int64, variantKey string) (*storage.Snapshot, error)
	Upsert(ctx context.Context, snapshot *storage.Snapshot) error
	MarkRefreshError(ctx context.Context, dandanEpisodeID int64, variantKey string, retryAt time.Time, message string) error
	RecordAccess(ctx context.Context, dandanEpisodeID int64, variantKey string, accessedAt time.Time, window time.Duration) error
}

type SnapshotCache interface {
	Get(ctx context.Context, dandanEpisodeID int64, variantKey string) (*storage.Snapshot, CacheStatus, error)
	Set(ctx context.Context, snapshot *storage.Snapshot, ttl time.Duration) error
	Delete(ctx context.Context, dandanEpisodeID int64, variantKey string) error
}

type UpstreamClient interface {
	FetchComments(ctx context.Context, dandanEpisodeID string, query string) ([]byte, error)
}

type CommentResult struct {
	Payload       []byte
	CacheStatus   string
	VariantKey    string
	FetchedAt     time.Time
	NextRefreshAt time.Time
}

type CommentServiceOptions struct {
	Cache              SnapshotCache
	Store              SnapshotStore
	Upstream           UpstreamClient
	Policy             RefreshPolicy
	RedisSnapshotTTL   time.Duration
	RefreshQueueSize   int
	RefreshWorkerCount int
	DecisionLog        bool
	Now                func() time.Time
}

type CommentService struct {
	cache            SnapshotCache
	store            SnapshotStore
	upstream         UpstreamClient
	policy           RefreshPolicy
	redisSnapshotTTL time.Duration
	now              func() time.Time
	loadGroup        singleflight.Group
	refreshGroup     singleflight.Group
	refreshQueue     chan refreshRequest
	refreshMu        sync.Mutex
	refreshPending   map[string]struct{}
	decisionLog      bool
	closeOnce        sync.Once
	cancel           context.CancelFunc
}

type refreshRequest struct {
	dandanEpisodeID int64
	episodeIDString string
	variant         Variant
}

func NewCommentService(options CommentServiceOptions) *CommentService {
	now := options.Now
	if now == nil {
		now = time.Now
	}
	queueSize := options.RefreshQueueSize
	if queueSize <= 0 {
		queueSize = 1
	}
	ctx, cancel := context.WithCancel(context.Background())
	service := &CommentService{
		cache:            options.Cache,
		store:            options.Store,
		upstream:         options.Upstream,
		policy:           options.Policy,
		redisSnapshotTTL: options.RedisSnapshotTTL,
		now:              now,
		refreshQueue:     make(chan refreshRequest, queueSize),
		refreshPending:   make(map[string]struct{}),
		decisionLog:      options.DecisionLog,
		cancel:           cancel,
	}
	for i := 0; i < options.RefreshWorkerCount; i++ {
		go service.refreshWorker(ctx)
	}
	return service
}

func (s *CommentService) Close() {
	s.closeOnce.Do(func() {
		if s.cancel != nil {
			s.cancel()
		}
	})
}

func (s *CommentService) GetComments(ctx context.Context, dandanEpisodeID string, query url.Values) (*CommentResult, error) {
	episodeID, err := strconv.ParseInt(dandanEpisodeID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid dandan episode id: %w", err)
	}
	variant := NormalizeVariant(query)
	variantKey := variant.Key()

	if snapshot, status, err := s.cache.Get(ctx, episodeID, variantKey); err == nil && status == CacheHit {
		payload, err := snapshotPayload(snapshot)
		if err != nil {
			return nil, err
		}
		s.recordAccess(ctx, episodeID, variantKey)
		if !snapshot.NextRefreshAt.After(s.now()) {
			_ = s.cache.Delete(ctx, episodeID, variantKey)
			s.enqueueRefresh(episodeID, dandanEpisodeID, variant)
			return s.resultWithLog(episodeID, payload, "stale", snapshot), nil
		}
		return s.resultWithLog(episodeID, payload, "redis", snapshot), nil
	}

	snapshot, err := s.store.Get(ctx, episodeID, variantKey)
	if err == nil {
		payload, err := snapshotPayload(snapshot)
		if err != nil {
			return nil, err
		}
		s.recordAccess(ctx, episodeID, variantKey)
		if !snapshot.NextRefreshAt.After(s.now()) {
			s.enqueueRefresh(episodeID, dandanEpisodeID, variant)
			return s.resultWithLog(episodeID, payload, "stale", snapshot), nil
		}
		_ = s.cache.Set(ctx, snapshot, s.redisSnapshotTTL)
		return s.resultWithLog(episodeID, payload, "postgres", snapshot), nil
	}
	if !errors.Is(err, storage.ErrSnapshotNotFound) {
		return nil, err
	}

	return s.firstLoad(ctx, episodeID, dandanEpisodeID, variant)
}

func (s *CommentService) firstLoad(ctx context.Context, episodeID int64, episodeIDString string, variant Variant) (*CommentResult, error) {
	key := fmt.Sprintf("%d:%s", episodeID, variant.Key())
	result, err, _ := s.loadGroup.Do(key, func() (interface{}, error) {
		if snapshot, status, err := s.cache.Get(ctx, episodeID, variant.Key()); err == nil && status == CacheHit {
			payload, err := snapshotPayload(snapshot)
			if err != nil {
				return nil, err
			}
			s.recordAccess(ctx, episodeID, variant.Key())
			status := "redis"
			if !snapshot.NextRefreshAt.After(s.now()) {
				status = "stale"
			}
			return s.resultWithLog(episodeID, payload, status, snapshot), nil
		}
		if snapshot, err := s.store.Get(ctx, episodeID, variant.Key()); err == nil {
			payload, err := snapshotPayload(snapshot)
			if err != nil {
				return nil, err
			}
			s.recordAccess(ctx, episodeID, variant.Key())
			status := "postgres"
			if !snapshot.NextRefreshAt.After(s.now()) {
				status = "stale"
				s.enqueueRefresh(episodeID, episodeIDString, variant)
			} else {
				_ = s.cache.Set(ctx, snapshot, s.redisSnapshotTTL)
			}
			return s.resultWithLog(episodeID, payload, status, snapshot), nil
		} else if !errors.Is(err, storage.ErrSnapshotNotFound) {
			return nil, err
		}

		payload, err := s.upstream.FetchComments(ctx, episodeIDString, variant.UpstreamQuery())
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrUpstreamUnavailable, err)
		}
		snapshot, err := s.snapshotFromPayload(episodeID, variant.Key(), payload, nil)
		if err != nil {
			return nil, err
		}
		if err := s.store.Upsert(ctx, snapshot); err != nil {
			return nil, err
		}
		_ = s.cache.Set(ctx, snapshot, s.redisSnapshotTTL)
		return s.resultWithLog(episodeID, payload, "upstream", snapshot), nil
	})
	if err != nil {
		return nil, err
	}
	return result.(*CommentResult), nil
}

func (s *CommentService) enqueueRefresh(episodeID int64, episodeIDString string, variant Variant) {
	key := refreshKey(episodeID, variant.Key())
	if !s.markRefreshPending(key) {
		s.logDecision("danmaku refresh suppressed episode_id=%d variant_key=%s reason=duplicate_refresh_pending", episodeID, variant.Key())
		return
	}
	request := refreshRequest{
		dandanEpisodeID: episodeID,
		episodeIDString: episodeIDString,
		variant:         variant,
	}
	select {
	case s.refreshQueue <- request:
		s.logDecision("danmaku refresh queued episode_id=%d variant_key=%s", episodeID, variant.Key())
	default:
		s.clearRefreshPending(key)
		s.logDecision("danmaku refresh skipped episode_id=%d variant_key=%s reason=refresh_queue_full", episodeID, variant.Key())
	}
}

func (s *CommentService) refreshWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case request := <-s.refreshQueue:
			s.refresh(ctx, request)
		}
	}
}

func (s *CommentService) refresh(ctx context.Context, request refreshRequest) {
	key := refreshKey(request.dandanEpisodeID, request.variant.Key())
	defer s.clearRefreshPending(key)

	_, _, _ = s.refreshGroup.Do(key, func() (interface{}, error) {
		snapshot, err := s.store.Get(ctx, request.dandanEpisodeID, request.variant.Key())
		if err == nil && snapshot.NextRefreshAt.After(s.now()) {
			s.logDecision("danmaku refresh skipped episode_id=%d variant_key=%s reason=snapshot_already_fresh next_refresh_at=%s", request.dandanEpisodeID, request.variant.Key(), snapshot.NextRefreshAt.UTC().Format(time.RFC3339))
			return nil, nil
		}
		if err != nil && !errors.Is(err, storage.ErrSnapshotNotFound) {
			return nil, err
		}

		payload, err := s.upstream.FetchComments(ctx, request.episodeIDString, request.variant.UpstreamQuery())
		if err != nil {
			_ = s.store.MarkRefreshError(ctx, request.dandanEpisodeID, request.variant.Key(), s.policy.RefreshFailureRetryAt(s.now()), err.Error())
			s.logDecision("danmaku refresh failed episode_id=%d variant_key=%s rule=refresh_failed_retry retry_at=%s error=%v", request.dandanEpisodeID, request.variant.Key(), s.policy.RefreshFailureRetryAt(s.now()).UTC().Format(time.RFC3339), err)
			return nil, err
		}
		snapshot, err = s.snapshotFromPayload(request.dandanEpisodeID, request.variant.Key(), payload, snapshot)
		if err != nil {
			_ = s.store.MarkRefreshError(ctx, request.dandanEpisodeID, request.variant.Key(), s.policy.RefreshFailureRetryAt(s.now()), err.Error())
			s.logDecision("danmaku refresh failed episode_id=%d variant_key=%s rule=refresh_failed_retry retry_at=%s error=%v", request.dandanEpisodeID, request.variant.Key(), s.policy.RefreshFailureRetryAt(s.now()).UTC().Format(time.RFC3339), err)
			return nil, err
		}
		if err := s.store.Upsert(ctx, snapshot); err != nil {
			return nil, err
		}
		_ = s.cache.Set(ctx, snapshot, s.redisSnapshotTTL)
		s.logDecision("danmaku refresh stored episode_id=%d variant_key=%s fetched_at=%s next_refresh_at=%s", request.dandanEpisodeID, request.variant.Key(), snapshot.FetchedAt.UTC().Format(time.RFC3339), snapshot.NextRefreshAt.UTC().Format(time.RFC3339))
		return nil, nil
	})
}

func refreshKey(episodeID int64, variantKey string) string {
	return fmt.Sprintf("%d:%s", episodeID, variantKey)
}

func (s *CommentService) markRefreshPending(key string) bool {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()
	if _, ok := s.refreshPending[key]; ok {
		return false
	}
	s.refreshPending[key] = struct{}{}
	return true
}

func (s *CommentService) clearRefreshPending(key string) {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()
	delete(s.refreshPending, key)
}

func (s *CommentService) recordAccess(ctx context.Context, episodeID int64, variantKey string) {
	_ = s.store.RecordAccess(ctx, episodeID, variantKey, s.now(), s.policy.accessWindow())
}

func (s *CommentService) snapshotFromPayload(episodeID int64, variantKey string, payload []byte, previous *storage.Snapshot) (*storage.Snapshot, error) {
	info, err := ValidatePayload(payload)
	if err != nil {
		return nil, err
	}
	compressed, err := GzipPayload(payload)
	if err != nil {
		return nil, err
	}
	now := s.now()
	context := RefreshContext{
		Info:              info,
		RecentAccessCount: 1,
	}
	accessCount := int64(1)
	recentAccessCount := 1
	lastAccessedAt := now
	recentAccessWindowStartedAt := now
	if previous != nil {
		context.PreviousContentHash = previous.ContentHash
		context.PreviousUnchangedStreak = previous.UnchangedStreak
		context.RecentAccessCount = previous.RecentAccessCount
		accessCount = previous.AccessCount
		recentAccessCount = previous.RecentAccessCount
		lastAccessedAt = previous.LastAccessedAt
		recentAccessWindowStartedAt = previous.RecentAccessWindowStartedAt
	}
	decision := s.policy.NextRefreshDecision(now, context)
	s.logDecision(
		"danmaku refresh decision episode_id=%d variant_key=%s rule=%s danmaku_count=%d recent_access_count=%d unchanged_streak=%d next_refresh_at=%s",
		episodeID,
		variantKey,
		decision.Rule,
		info.DanmakuCount,
		recentAccessCount,
		decision.UnchangedStreak,
		decision.NextRefreshAt.UTC().Format(time.RFC3339),
	)
	return &storage.Snapshot{
		DandanEpisodeID:             episodeID,
		VariantKey:                  variantKey,
		Payload:                     compressed,
		PayloadEncoding:             "gzip",
		FetchedAt:                   now,
		NextRefreshAt:               decision.NextRefreshAt,
		DanmakuCount:                info.DanmakuCount,
		ContentHash:                 info.ContentHash,
		UnchangedStreak:             decision.UnchangedStreak,
		Version:                     1,
		LastRefreshStatus:           "success",
		LastAccessedAt:              lastAccessedAt,
		AccessCount:                 accessCount,
		RecentAccessCount:           recentAccessCount,
		RecentAccessWindowStartedAt: recentAccessWindowStartedAt,
	}, nil
}

func (s *CommentService) resultWithLog(episodeID int64, payload []byte, cacheStatus string, snapshot *storage.Snapshot) *CommentResult {
	result := commentResult(payload, cacheStatus, snapshot)
	s.logDecision(
		"danmaku request resolved episode_id=%d variant_key=%s source=%s fetched_at=%s next_refresh_at=%s",
		episodeID,
		result.VariantKey,
		result.CacheStatus,
		result.FetchedAt.UTC().Format(time.RFC3339),
		result.NextRefreshAt.UTC().Format(time.RFC3339),
	)
	return result
}

func (s *CommentService) logDecision(format string, args ...interface{}) {
	if !s.decisionLog {
		return
	}
	log.Printf(format, args...)
}

func snapshotPayload(snapshot *storage.Snapshot) ([]byte, error) {
	switch snapshot.PayloadEncoding {
	case "", "gzip":
		return GunzipPayload(snapshot.Payload)
	default:
		return nil, fmt.Errorf("unsupported payload encoding: %s", snapshot.PayloadEncoding)
	}
}

func commentResult(payload []byte, cacheStatus string, snapshot *storage.Snapshot) *CommentResult {
	return &CommentResult{
		Payload:       payload,
		CacheStatus:   cacheStatus,
		VariantKey:    snapshot.VariantKey,
		FetchedAt:     snapshot.FetchedAt,
		NextRefreshAt: snapshot.NextRefreshAt,
	}
}
