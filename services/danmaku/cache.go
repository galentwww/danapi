package danmaku

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"dandanplay-middleware/storage"

	"github.com/go-redis/redis/v8"
)

type CacheStatus int

const (
	CacheHit CacheStatus = iota
	CacheMiss
	CacheUnavailable
	CacheCorrupted
)

type RedisSnapshotCache struct {
	client *redis.Client
}

func NewRedisSnapshotCache(client *redis.Client) *RedisSnapshotCache {
	return &RedisSnapshotCache{client: client}
}

func SnapshotCacheKey(dandanEpisodeID int64, variantKey string) string {
	hash := sha256.Sum256([]byte(variantKey))
	return fmt.Sprintf("ddp:comments:v1:%d:%s", dandanEpisodeID, hex.EncodeToString(hash[:])[:16])
}

func (c *RedisSnapshotCache) Get(ctx context.Context, dandanEpisodeID int64, variantKey string) (*storage.Snapshot, CacheStatus, error) {
	data, err := c.client.Get(ctx, SnapshotCacheKey(dandanEpisodeID, variantKey)).Bytes()
	if err == redis.Nil {
		return nil, CacheMiss, nil
	}
	if err != nil {
		return nil, CacheUnavailable, err
	}
	return decodeCacheEnvelope(data)
}

func (c *RedisSnapshotCache) Set(ctx context.Context, snapshot *storage.Snapshot, ttl time.Duration) error {
	data, err := encodeCacheEnvelope(snapshot)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, SnapshotCacheKey(snapshot.DandanEpisodeID, snapshot.VariantKey), data, ttl).Err()
}

func (c *RedisSnapshotCache) Delete(ctx context.Context, dandanEpisodeID int64, variantKey string) error {
	return c.client.Del(ctx, SnapshotCacheKey(dandanEpisodeID, variantKey)).Err()
}

type cacheEnvelope struct {
	DandanEpisodeID     int64     `json:"dandanEpisodeId"`
	VariantKey          string    `json:"variantKey"`
	Payload             []byte    `json:"payload"`
	PayloadEncoding     string    `json:"payloadEncoding"`
	FetchedAt           time.Time `json:"fetchedAt"`
	NextRefreshAt       time.Time `json:"nextRefreshAt"`
	DanmakuCount        int       `json:"danmakuCount"`
	ContentHash         string    `json:"contentHash"`
	UnchangedStreak     int       `json:"unchangedStreak"`
	Version             int64     `json:"version"`
	LastRefreshStatus   string    `json:"lastRefreshStatus"`
	RefreshErrorMessage string    `json:"refreshErrorMessage,omitempty"`
}

func encodeCacheEnvelope(snapshot *storage.Snapshot) ([]byte, error) {
	return json.Marshal(cacheEnvelope{
		DandanEpisodeID:     snapshot.DandanEpisodeID,
		VariantKey:          snapshot.VariantKey,
		Payload:             snapshot.Payload,
		PayloadEncoding:     snapshot.PayloadEncoding,
		FetchedAt:           snapshot.FetchedAt,
		NextRefreshAt:       snapshot.NextRefreshAt,
		DanmakuCount:        snapshot.DanmakuCount,
		ContentHash:         snapshot.ContentHash,
		UnchangedStreak:     snapshot.UnchangedStreak,
		Version:             snapshot.Version,
		LastRefreshStatus:   snapshot.LastRefreshStatus,
		RefreshErrorMessage: snapshot.RefreshErrorMessage,
	})
}

func decodeCacheEnvelope(data []byte) (*storage.Snapshot, CacheStatus, error) {
	var envelope cacheEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, CacheCorrupted, err
	}
	return &storage.Snapshot{
		DandanEpisodeID:     envelope.DandanEpisodeID,
		VariantKey:          envelope.VariantKey,
		Payload:             envelope.Payload,
		PayloadEncoding:     envelope.PayloadEncoding,
		FetchedAt:           envelope.FetchedAt,
		NextRefreshAt:       envelope.NextRefreshAt,
		DanmakuCount:        envelope.DanmakuCount,
		ContentHash:         envelope.ContentHash,
		UnchangedStreak:     envelope.UnchangedStreak,
		Version:             envelope.Version,
		LastRefreshStatus:   envelope.LastRefreshStatus,
		RefreshErrorMessage: envelope.RefreshErrorMessage,
	}, CacheHit, nil
}
