do $$
begin
    if exists (
        select 1
        from information_schema.columns
        where table_schema = 'public'
            and table_name = 'danmaku_snapshots'
            and column_name = 'comment_count'
    ) and not exists (
        select 1
        from information_schema.columns
        where table_schema = 'public'
            and table_name = 'danmaku_snapshots'
            and column_name = 'danmaku_count'
    ) then
        alter table danmaku_snapshots
            rename column comment_count to danmaku_count;
    end if;
end $$;
