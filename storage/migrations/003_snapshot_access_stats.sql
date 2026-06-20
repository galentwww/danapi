alter table danmaku_snapshots
    add column if not exists last_accessed_at timestamptz null,
    add column if not exists access_count bigint not null default 0,
    add column if not exists recent_access_count integer not null default 0,
    add column if not exists recent_access_window_started_at timestamptz null;
