package storage

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrSnapshotNotFound = errors.New("danmaku snapshot not found")

type Snapshot struct {
	DandanEpisodeID             int64
	VariantKey                  string
	Payload                     []byte
	PayloadEncoding             string
	FetchedAt                   time.Time
	NextRefreshAt               time.Time
	DanmakuCount                int
	ContentHash                 string
	UnchangedStreak             int
	Version                     int64
	LastRefreshStatus           string
	RefreshErrorMessage         string
	LastAccessedAt              time.Time
	AccessCount                 int64
	RecentAccessCount           int
	RecentAccessWindowStartedAt time.Time
	CreatedAt                   time.Time
	UpdatedAt                   time.Time
}

type PostgresSnapshotStore struct {
	db *pgxpool.Pool
}

func NewPostgresSnapshotStore(db *pgxpool.Pool) *PostgresSnapshotStore {
	return &PostgresSnapshotStore{db: db}
}

func (s *PostgresSnapshotStore) Get(ctx context.Context, dandanEpisodeID int64, variantKey string) (*Snapshot, error) {
	const query = `
select
	dandan_episode_id,
	variant_key,
	payload,
	payload_encoding,
	fetched_at,
	next_refresh_at,
	danmaku_count,
	content_hash,
	unchanged_streak,
	version,
	last_refresh_status,
	coalesce(last_error, ''),
	coalesce(last_accessed_at, '0001-01-01 00:00:00+00'::timestamptz),
	access_count,
	recent_access_count,
	coalesce(recent_access_window_started_at, '0001-01-01 00:00:00+00'::timestamptz),
	created_at,
	updated_at
from danmaku_snapshots
where dandan_episode_id = $1 and variant_key = $2`

	var snapshot Snapshot
	err := s.db.QueryRow(ctx, query, dandanEpisodeID, variantKey).Scan(
		&snapshot.DandanEpisodeID,
		&snapshot.VariantKey,
		&snapshot.Payload,
		&snapshot.PayloadEncoding,
		&snapshot.FetchedAt,
		&snapshot.NextRefreshAt,
		&snapshot.DanmakuCount,
		&snapshot.ContentHash,
		&snapshot.UnchangedStreak,
		&snapshot.Version,
		&snapshot.LastRefreshStatus,
		&snapshot.RefreshErrorMessage,
		&snapshot.LastAccessedAt,
		&snapshot.AccessCount,
		&snapshot.RecentAccessCount,
		&snapshot.RecentAccessWindowStartedAt,
		&snapshot.CreatedAt,
		&snapshot.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrSnapshotNotFound
	}
	if err != nil {
		return nil, err
	}
	return &snapshot, nil
}

func (s *PostgresSnapshotStore) Upsert(ctx context.Context, snapshot *Snapshot) error {
	if snapshot.PayloadEncoding == "" {
		snapshot.PayloadEncoding = "gzip"
	}
	if snapshot.LastRefreshStatus == "" {
		snapshot.LastRefreshStatus = "success"
	}
	if snapshot.Version == 0 {
		snapshot.Version = 1
	}

	const query = `
insert into danmaku_snapshots (
	dandan_episode_id,
	variant_key,
	payload,
	payload_encoding,
	fetched_at,
	next_refresh_at,
	danmaku_count,
	content_hash,
	unchanged_streak,
	version,
	last_refresh_status,
	last_error,
	last_accessed_at,
	access_count,
	recent_access_count,
	recent_access_window_started_at,
	updated_at
) values (
	$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, nullif($12, ''), $13, $14, $15, $16, now()
)
on conflict (dandan_episode_id, variant_key)
do update set
	payload = excluded.payload,
	payload_encoding = excluded.payload_encoding,
	fetched_at = excluded.fetched_at,
	next_refresh_at = excluded.next_refresh_at,
	danmaku_count = excluded.danmaku_count,
	content_hash = excluded.content_hash,
	unchanged_streak = excluded.unchanged_streak,
	version = danmaku_snapshots.version + 1,
	last_refresh_status = excluded.last_refresh_status,
	last_error = excluded.last_error,
	updated_at = now()`

	_, err := s.db.Exec(ctx, query,
		snapshot.DandanEpisodeID,
		snapshot.VariantKey,
		snapshot.Payload,
		snapshot.PayloadEncoding,
		snapshot.FetchedAt,
		snapshot.NextRefreshAt,
		snapshot.DanmakuCount,
		snapshot.ContentHash,
		snapshot.UnchangedStreak,
		snapshot.Version,
		snapshot.LastRefreshStatus,
		snapshot.RefreshErrorMessage,
		snapshot.LastAccessedAt,
		snapshot.AccessCount,
		snapshot.RecentAccessCount,
		snapshot.RecentAccessWindowStartedAt,
	)
	return err
}

func (s *PostgresSnapshotStore) RecordAccess(ctx context.Context, dandanEpisodeID int64, variantKey string, accessedAt time.Time, window time.Duration) error {
	if window <= 0 {
		window = 24 * time.Hour
	}
	const query = `
update danmaku_snapshots
set
	last_accessed_at = $3,
	access_count = access_count + 1,
	recent_access_count = case
		when recent_access_window_started_at is null then 1
		when $3 - recent_access_window_started_at >= ($4 * interval '1 second') then 1
		else recent_access_count + 1
	end,
	recent_access_window_started_at = case
		when recent_access_window_started_at is null then $3
		when $3 - recent_access_window_started_at >= ($4 * interval '1 second') then $3
		else recent_access_window_started_at
	end,
	updated_at = now()
where dandan_episode_id = $1 and variant_key = $2`

	tag, err := s.db.Exec(ctx, query, dandanEpisodeID, variantKey, accessedAt, int64(window.Seconds()))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrSnapshotNotFound
	}
	return nil
}

func (s *PostgresSnapshotStore) MarkRefreshError(ctx context.Context, dandanEpisodeID int64, variantKey string, retryAt time.Time, message string) error {
	const query = `
update danmaku_snapshots
set
	next_refresh_at = $3,
	last_refresh_status = 'error',
	last_error = $4,
	updated_at = now()
where dandan_episode_id = $1 and variant_key = $2`

	tag, err := s.db.Exec(ctx, query, dandanEpisodeID, variantKey, retryAt, message)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrSnapshotNotFound
	}
	return nil
}
