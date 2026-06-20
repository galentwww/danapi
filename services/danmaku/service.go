package danmaku

import (
	"context"
	"errors"
	"fmt"
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
		if !snapshot.NextRefreshAt.After(s.now()) {
			s.enqueueRefresh(episodeID, dandanEpisodeID, variant)
			return commentResult(payload, "stale", snapshot), nil
		}
		return commentResult(payload, "redis", snapshot), nil
	}

	snapshot, err := s.store.Get(ctx, episodeID, variantKey)
	if err == nil {
		_ = s.cache.Set(ctx, snapshot, s.redisSnapshotTTL)
		payload, err := snapshotPayload(snapshot)
		if err != nil {
			return nil, err
		}
		if !snapshot.NextRefreshAt.After(s.now()) {
			s.enqueueRefresh(episodeID, dandanEpisodeID, variant)
			return commentResult(payload, "stale", snapshot), nil
		}
		return commentResult(payload, "postgres", snapshot), nil
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
			status := "redis"
			if !snapshot.NextRefreshAt.After(s.now()) {
				status = "stale"
			}
			return commentResult(payload, status, snapshot), nil
		}
		if snapshot, err := s.store.Get(ctx, episodeID, variant.Key()); err == nil {
			_ = s.cache.Set(ctx, snapshot, s.redisSnapshotTTL)
			payload, err := snapshotPayload(snapshot)
			if err != nil {
				return nil, err
			}
			status := "postgres"
			if !snapshot.NextRefreshAt.After(s.now()) {
				status = "stale"
			}
			return commentResult(payload, status, snapshot), nil
		} else if !errors.Is(err, storage.ErrSnapshotNotFound) {
			return nil, err
		}

		payload, err := s.upstream.FetchComments(ctx, episodeIDString, variant.UpstreamQuery())
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrUpstreamUnavailable, err)
		}
		snapshot, err := s.snapshotFromPayload(episodeID, variant.Key(), payload)
		if err != nil {
			return nil, err
		}
		if err := s.store.Upsert(ctx, snapshot); err != nil {
			return nil, err
		}
		_ = s.cache.Set(ctx, snapshot, s.redisSnapshotTTL)
		return commentResult(payload, "upstream", snapshot), nil
	})
	if err != nil {
		return nil, err
	}
	return result.(*CommentResult), nil
}

func (s *CommentService) enqueueRefresh(episodeID int64, episodeIDString string, variant Variant) {
	request := refreshRequest{
		dandanEpisodeID: episodeID,
		episodeIDString: episodeIDString,
		variant:         variant,
	}
	select {
	case s.refreshQueue <- request:
	default:
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
	key := fmt.Sprintf("%d:%s", request.dandanEpisodeID, request.variant.Key())
	_, _, _ = s.refreshGroup.Do(key, func() (interface{}, error) {
		payload, err := s.upstream.FetchComments(ctx, request.episodeIDString, request.variant.UpstreamQuery())
		if err != nil {
			_ = s.store.MarkRefreshError(ctx, request.dandanEpisodeID, request.variant.Key(), s.policy.RefreshFailureRetryAt(s.now()), err.Error())
			return nil, err
		}
		snapshot, err := s.snapshotFromPayload(request.dandanEpisodeID, request.variant.Key(), payload)
		if err != nil {
			_ = s.store.MarkRefreshError(ctx, request.dandanEpisodeID, request.variant.Key(), s.policy.RefreshFailureRetryAt(s.now()), err.Error())
			return nil, err
		}
		if err := s.store.Upsert(ctx, snapshot); err != nil {
			return nil, err
		}
		_ = s.cache.Set(ctx, snapshot, s.redisSnapshotTTL)
		return nil, nil
	})
}

func (s *CommentService) snapshotFromPayload(episodeID int64, variantKey string, payload []byte) (*storage.Snapshot, error) {
	info, err := ValidatePayload(payload)
	if err != nil {
		return nil, err
	}
	compressed, err := GzipPayload(payload)
	if err != nil {
		return nil, err
	}
	now := s.now()
	return &storage.Snapshot{
		DandanEpisodeID:   episodeID,
		VariantKey:        variantKey,
		Payload:           compressed,
		PayloadEncoding:   "gzip",
		FetchedAt:         now,
		NextRefreshAt:     s.policy.NextRefreshAt(now, info),
		CommentCount:      info.CommentCount,
		ContentHash:       info.ContentHash,
		UnchangedStreak:   0,
		Version:           1,
		LastRefreshStatus: "success",
	}, nil
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
