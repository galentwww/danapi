package danmaku

import "time"

type RefreshPolicy struct {
	DefaultRefreshInterval      time.Duration
	EmptyDanmakuRefreshInterval time.Duration
	RefreshFailureRetryInterval time.Duration
}

func (p RefreshPolicy) NextRefreshAt(now time.Time, info PayloadInfo) time.Time {
	if info.DanmakuCount == 0 {
		return now.Add(p.EmptyDanmakuRefreshInterval)
	}
	return now.Add(p.DefaultRefreshInterval)
}

func (p RefreshPolicy) RefreshFailureRetryAt(now time.Time) time.Time {
	return now.Add(p.RefreshFailureRetryInterval)
}
