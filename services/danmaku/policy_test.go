package danmaku

import (
	"testing"
	"time"
)

func TestNextRefreshAtUsesEmptyCommentsInterval(t *testing.T) {
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	policy := RefreshPolicy{
		DefaultRefreshInterval:      24 * time.Hour,
		EmptyDanmakuRefreshInterval: time.Hour,
		RefreshFailureRetryInterval: 30 * time.Minute,
	}

	next := policy.NextRefreshAt(now, PayloadInfo{DanmakuCount: 0})

	if !next.Equal(now.Add(time.Hour)) {
		t.Fatalf("NextRefreshAt = %s", next)
	}
}

func TestNextRefreshAtUsesDefaultIntervalForNonEmptyComments(t *testing.T) {
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	policy := RefreshPolicy{
		DefaultRefreshInterval:      24 * time.Hour,
		EmptyDanmakuRefreshInterval: time.Hour,
		RefreshFailureRetryInterval: 30 * time.Minute,
	}

	next := policy.NextRefreshAt(now, PayloadInfo{DanmakuCount: 3})

	if !next.Equal(now.Add(24 * time.Hour)) {
		t.Fatalf("NextRefreshAt = %s", next)
	}
}

func TestRefreshFailureRetryAt(t *testing.T) {
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	policy := RefreshPolicy{
		DefaultRefreshInterval:      24 * time.Hour,
		EmptyDanmakuRefreshInterval: time.Hour,
		RefreshFailureRetryInterval: 30 * time.Minute,
	}

	next := policy.RefreshFailureRetryAt(now)

	if !next.Equal(now.Add(30 * time.Minute)) {
		t.Fatalf("RefreshFailureRetryAt = %s", next)
	}
}
