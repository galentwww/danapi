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

func TestNextRefreshDecisionUsesEmptyDanmakuInterval(t *testing.T) {
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	policy := testDynamicPolicy()

	decision := policy.NextRefreshDecision(now, RefreshContext{
		Info:                    PayloadInfo{DanmakuCount: 0, ContentHash: "empty-new"},
		PreviousContentHash:     "empty-old",
		PreviousUnchangedStreak: 6,
		RecentAccessCount:       100,
	})

	if !decision.NextRefreshAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("NextRefreshAt = %s", decision.NextRefreshAt)
	}
	if decision.UnchangedStreak != 0 {
		t.Fatalf("UnchangedStreak = %d", decision.UnchangedStreak)
	}
}

func TestNextRefreshDecisionUsesHotChangedInterval(t *testing.T) {
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	policy := testDynamicPolicy()

	decision := policy.NextRefreshDecision(now, RefreshContext{
		Info:                PayloadInfo{DanmakuCount: 12, ContentHash: "new"},
		PreviousContentHash: "old",
		RecentAccessCount:   10,
	})

	if !decision.NextRefreshAt.Equal(now.Add(2 * time.Hour)) {
		t.Fatalf("NextRefreshAt = %s", decision.NextRefreshAt)
	}
	if decision.UnchangedStreak != 0 {
		t.Fatalf("UnchangedStreak = %d", decision.UnchangedStreak)
	}
}

func TestNextRefreshDecisionUsesHotUnchangedInterval(t *testing.T) {
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	policy := testDynamicPolicy()

	decision := policy.NextRefreshDecision(now, RefreshContext{
		Info:                    PayloadInfo{DanmakuCount: 12, ContentHash: "same"},
		PreviousContentHash:     "same",
		PreviousUnchangedStreak: 1,
		RecentAccessCount:       10,
	})

	if !decision.NextRefreshAt.Equal(now.Add(6 * time.Hour)) {
		t.Fatalf("NextRefreshAt = %s", decision.NextRefreshAt)
	}
	if decision.UnchangedStreak != 2 {
		t.Fatalf("UnchangedStreak = %d", decision.UnchangedStreak)
	}
}

func TestNextRefreshDecisionUsesNormalChangedInterval(t *testing.T) {
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	policy := testDynamicPolicy()

	decision := policy.NextRefreshDecision(now, RefreshContext{
		Info:                PayloadInfo{DanmakuCount: 12, ContentHash: "new"},
		PreviousContentHash: "old",
		RecentAccessCount:   9,
	})

	if !decision.NextRefreshAt.Equal(now.Add(12 * time.Hour)) {
		t.Fatalf("NextRefreshAt = %s", decision.NextRefreshAt)
	}
}

func TestNextRefreshDecisionUsesNormalUnchangedInterval(t *testing.T) {
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	policy := testDynamicPolicy()

	decision := policy.NextRefreshDecision(now, RefreshContext{
		Info:                    PayloadInfo{DanmakuCount: 12, ContentHash: "same"},
		PreviousContentHash:     "same",
		PreviousUnchangedStreak: 0,
		RecentAccessCount:       9,
	})

	if !decision.NextRefreshAt.Equal(now.Add(24 * time.Hour)) {
		t.Fatalf("NextRefreshAt = %s", decision.NextRefreshAt)
	}
	if decision.UnchangedStreak != 1 {
		t.Fatalf("UnchangedStreak = %d", decision.UnchangedStreak)
	}
}

func TestNextRefreshDecisionUsesStableIntervalsForUnchangedStreaks(t *testing.T) {
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	policy := testDynamicPolicy()

	three := policy.NextRefreshDecision(now, RefreshContext{
		Info:                    PayloadInfo{DanmakuCount: 12, ContentHash: "same"},
		PreviousContentHash:     "same",
		PreviousUnchangedStreak: 2,
		RecentAccessCount:       100,
	})
	seven := policy.NextRefreshDecision(now, RefreshContext{
		Info:                    PayloadInfo{DanmakuCount: 12, ContentHash: "same"},
		PreviousContentHash:     "same",
		PreviousUnchangedStreak: 6,
		RecentAccessCount:       100,
	})

	if !three.NextRefreshAt.Equal(now.Add(72 * time.Hour)) {
		t.Fatalf("three NextRefreshAt = %s", three.NextRefreshAt)
	}
	if three.UnchangedStreak != 3 {
		t.Fatalf("three UnchangedStreak = %d", three.UnchangedStreak)
	}
	if !seven.NextRefreshAt.Equal(now.Add(168 * time.Hour)) {
		t.Fatalf("seven NextRefreshAt = %s", seven.NextRefreshAt)
	}
	if seven.UnchangedStreak != 7 {
		t.Fatalf("seven UnchangedStreak = %d", seven.UnchangedStreak)
	}
}

func testDynamicPolicy() RefreshPolicy {
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
