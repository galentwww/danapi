create table if not exists danmaku_snapshots (
    dandan_episode_id bigint not null,
    variant_key text not null,

    payload bytea not null,
    payload_encoding text not null default 'gzip',

    fetched_at timestamptz not null,
    next_refresh_at timestamptz not null,

    comment_count integer not null default 0,
    content_hash text not null,
    unchanged_streak integer not null default 0,

    version bigint not null default 1,
    last_refresh_status text not null default 'success',
    last_error text null,

    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),

    primary key (dandan_episode_id, variant_key)
);

create index if not exists danmaku_snapshots_next_refresh_at_idx
on danmaku_snapshots (next_refresh_at);

