package danmaku

import "time"

type RefreshPolicy struct {
	DefaultRefreshInterval       time.Duration
	EmptyCommentsRefreshInterval time.Duration
	RefreshFailureRetryInterval  time.Duration
}

func (p RefreshPolicy) NextRefreshAt(now time.Time, info PayloadInfo) time.Time {
	if info.CommentCount == 0 {
		return now.Add(p.EmptyCommentsRefreshInterval)
	}
	return now.Add(p.DefaultRefreshInterval)
}

func (p RefreshPolicy) RefreshFailureRetryAt(now time.Time) time.Time {
	return now.Add(p.RefreshFailureRetryInterval)
}
