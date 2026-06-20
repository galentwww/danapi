package danmaku

import "time"

type RefreshPolicy struct {
	DefaultRefreshInterval       time.Duration
	EmptyDanmakuRefreshInterval  time.Duration
	RefreshFailureRetryInterval  time.Duration
	HotAccessThreshold           int
	AccessWindow                 time.Duration
	HotChangedRefreshInterval    time.Duration
	HotUnchangedRefreshInterval  time.Duration
	NormalChangedRefreshInterval time.Duration
	StableRefreshInterval        time.Duration
	ArchivedRefreshInterval      time.Duration
}

func (p RefreshPolicy) NextRefreshAt(now time.Time, info PayloadInfo) time.Time {
	if info.DanmakuCount == 0 {
		return now.Add(p.EmptyDanmakuRefreshInterval)
	}
	return now.Add(p.DefaultRefreshInterval)
}

type RefreshContext struct {
	Info                    PayloadInfo
	PreviousContentHash     string
	PreviousUnchangedStreak int
	RecentAccessCount       int
}

type RefreshDecision struct {
	NextRefreshAt   time.Time
	UnchangedStreak int
	Rule            string
}

func (p RefreshPolicy) NextRefreshDecision(now time.Time, context RefreshContext) RefreshDecision {
	changed := context.PreviousContentHash == "" || context.PreviousContentHash != context.Info.ContentHash
	unchangedStreak := 0
	if !changed {
		unchangedStreak = context.PreviousUnchangedStreak + 1
	}

	interval, rule := p.refreshInterval(context, changed, unchangedStreak)
	return RefreshDecision{
		NextRefreshAt:   now.Add(interval),
		UnchangedStreak: unchangedStreak,
		Rule:            rule,
	}
}

func (p RefreshPolicy) refreshInterval(context RefreshContext, changed bool, unchangedStreak int) (time.Duration, string) {
	if context.Info.DanmakuCount == 0 {
		return p.emptyDanmakuRefreshInterval(), "empty_danmaku"
	}
	if unchangedStreak >= 7 {
		return p.archivedRefreshInterval(), "archived_unchanged"
	}
	if unchangedStreak >= 3 {
		return p.stableRefreshInterval(), "stable_unchanged"
	}
	if p.isHot(context.RecentAccessCount) {
		if changed {
			return p.hotChangedRefreshInterval(), "hot_changed"
		}
		return p.hotUnchangedRefreshInterval(), "hot_unchanged"
	}
	if changed {
		return p.normalChangedRefreshInterval(), "normal_changed"
	}
	return p.defaultRefreshInterval(), "normal_unchanged"
}

func (p RefreshPolicy) RefreshFailureRetryAt(now time.Time) time.Time {
	return now.Add(p.RefreshFailureRetryInterval)
}

func (p RefreshPolicy) isHot(recentAccessCount int) bool {
	threshold := p.HotAccessThreshold
	if threshold <= 0 {
		threshold = 10
	}
	return recentAccessCount >= threshold
}

func (p RefreshPolicy) accessWindow() time.Duration {
	if p.AccessWindow > 0 {
		return p.AccessWindow
	}
	return 24 * time.Hour
}

func (p RefreshPolicy) defaultRefreshInterval() time.Duration {
	if p.DefaultRefreshInterval > 0 {
		return p.DefaultRefreshInterval
	}
	return 24 * time.Hour
}

func (p RefreshPolicy) emptyDanmakuRefreshInterval() time.Duration {
	if p.EmptyDanmakuRefreshInterval > 0 {
		return p.EmptyDanmakuRefreshInterval
	}
	return time.Hour
}

func (p RefreshPolicy) hotChangedRefreshInterval() time.Duration {
	if p.HotChangedRefreshInterval > 0 {
		return p.HotChangedRefreshInterval
	}
	return 2 * time.Hour
}

func (p RefreshPolicy) hotUnchangedRefreshInterval() time.Duration {
	if p.HotUnchangedRefreshInterval > 0 {
		return p.HotUnchangedRefreshInterval
	}
	return 6 * time.Hour
}

func (p RefreshPolicy) normalChangedRefreshInterval() time.Duration {
	if p.NormalChangedRefreshInterval > 0 {
		return p.NormalChangedRefreshInterval
	}
	return 12 * time.Hour
}

func (p RefreshPolicy) stableRefreshInterval() time.Duration {
	if p.StableRefreshInterval > 0 {
		return p.StableRefreshInterval
	}
	return 72 * time.Hour
}

func (p RefreshPolicy) archivedRefreshInterval() time.Duration {
	if p.ArchivedRefreshInterval > 0 {
		return p.ArchivedRefreshInterval
	}
	return 168 * time.Hour
}
