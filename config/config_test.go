package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadConfigUsesEnvironmentWhenDotEnvIsMissing(t *testing.T) {
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("change working directory: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalDir)
	})

	t.Setenv("DANDANPLAY_BASE_URL", "https://api.example.test")
	t.Setenv("REDIS_HOST", "redis")
	t.Setenv("REDIS_PORT", "6379")
	t.Setenv("REDIS_PASSWORD", "secret")
	t.Setenv("REDIS_DB", "2")
	t.Setenv("SERVER_PORT", "18080")
	t.Setenv("SEARCH_CACHE_DURATION", "120")
	t.Setenv("DANMAKU_CACHE_DURATION", "60")
	t.Setenv("APP_ID", "app-id")
	t.Setenv("APP_SECRET", "app-secret")
	t.Setenv("DANDANPLAY_CREDENTIAL_LOG", "true")
	t.Setenv("CORS_ALLOW_ORIGINS", "https://example.test")
	t.Setenv("CORS_ALLOW_CREDENTIALS", "true")
	t.Setenv("CORS_MAX_AGE", "600")

	if err := LoadConfig(); err != nil {
		t.Fatalf("LoadConfig returned error without .env: %v", err)
	}

	if Config.DandanplayBaseURL != "https://api.example.test" {
		t.Fatalf("DandanplayBaseURL = %q", Config.DandanplayBaseURL)
	}
	if Config.RedisHost != "redis" {
		t.Fatalf("RedisHost = %q", Config.RedisHost)
	}
	if Config.RedisDB != 2 {
		t.Fatalf("RedisDB = %d", Config.RedisDB)
	}
	if Config.ServerPort != "18080" {
		t.Fatalf("ServerPort = %q", Config.ServerPort)
	}
	if Config.SearchCacheDuration != 120*time.Second {
		t.Fatalf("SearchCacheDuration = %v", Config.SearchCacheDuration)
	}
	if Config.DanmakuCacheDuration != 60*time.Second {
		t.Fatalf("DanmakuCacheDuration = %v", Config.DanmakuCacheDuration)
	}
	if !Config.CORSAllowCredentials {
		t.Fatal("CORSAllowCredentials = false")
	}
	if Config.CORSMaxAge != 600 {
		t.Fatalf("CORSMaxAge = %d", Config.CORSMaxAge)
	}
	if len(Config.DandanplayCredentials) != 1 {
		t.Fatalf("DandanplayCredentials len = %d", len(Config.DandanplayCredentials))
	}
	if Config.DandanplayCredentials[0].AppID != "app-id" {
		t.Fatalf("DandanplayCredentials[0].AppID = %q", Config.DandanplayCredentials[0].AppID)
	}
	if Config.DandanplayCredentials[0].AppSecret != "app-secret" {
		t.Fatalf("DandanplayCredentials[0].AppSecret = %q", Config.DandanplayCredentials[0].AppSecret)
	}
	if !Config.DandanplayCredentialLog {
		t.Fatal("DandanplayCredentialLog = false")
	}
}

func TestLoadConfigParsesDandanplayKeys(t *testing.T) {
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("change working directory: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalDir)
	})

	t.Setenv("APP_ID", "legacy-id")
	t.Setenv("APP_SECRET", "legacy-secret")
	t.Setenv("DANDANPLAY_KEYS", "app-a:secret-a, app-b:secret-b")

	if err := LoadConfig(); err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	if len(Config.DandanplayCredentials) != 2 {
		t.Fatalf("DandanplayCredentials len = %d", len(Config.DandanplayCredentials))
	}
	if Config.DandanplayCredentials[0].AppID != "app-a" || Config.DandanplayCredentials[0].AppSecret != "secret-a" {
		t.Fatalf("first credential = %#v", Config.DandanplayCredentials[0])
	}
	if Config.DandanplayCredentials[1].AppID != "app-b" || Config.DandanplayCredentials[1].AppSecret != "secret-b" {
		t.Fatalf("second credential = %#v", Config.DandanplayCredentials[1])
	}
}

func TestLoadConfigRejectsMalformedDandanplayKeys(t *testing.T) {
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("change working directory: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalDir)
	})

	t.Setenv("DANDANPLAY_KEYS", "app-a-secret-a")

	if err := LoadConfig(); err == nil {
		t.Fatal("LoadConfig returned nil for malformed DANDANPLAY_KEYS")
	}
}

func TestLoadConfigUsesSnapshotDefaults(t *testing.T) {
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("change working directory: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalDir)
	})

	t.Setenv("DATABASE_URL", "postgres://middleware:secret@postgres:5432/dandanplay_middleware?sslmode=disable")

	if err := LoadConfig(); err != nil {
		t.Fatalf("LoadConfig returned error without .env: %v", err)
	}

	if Config.DatabaseURL != "postgres://middleware:secret@postgres:5432/dandanplay_middleware?sslmode=disable" {
		t.Fatalf("DatabaseURL = %q", Config.DatabaseURL)
	}
	if Config.RedisSnapshotTTL != 48*time.Hour {
		t.Fatalf("RedisSnapshotTTL = %v", Config.RedisSnapshotTTL)
	}
	if Config.DefaultRefreshInterval != 24*time.Hour {
		t.Fatalf("DefaultRefreshInterval = %v", Config.DefaultRefreshInterval)
	}
	if Config.EmptyDanmakuRefreshInterval != time.Hour {
		t.Fatalf("EmptyDanmakuRefreshInterval = %v", Config.EmptyDanmakuRefreshInterval)
	}
	if Config.RefreshFailureRetryInterval != 30*time.Minute {
		t.Fatalf("RefreshFailureRetryInterval = %v", Config.RefreshFailureRetryInterval)
	}
	if Config.RefreshQueueSize != 100 {
		t.Fatalf("RefreshQueueSize = %d", Config.RefreshQueueSize)
	}
	if Config.RefreshWorkerCount != 2 {
		t.Fatalf("RefreshWorkerCount = %d", Config.RefreshWorkerCount)
	}
	if Config.DandanplayCredentialLog {
		t.Fatal("DandanplayCredentialLog = true")
	}
	if Config.RefreshAccessWindow != 24*time.Hour {
		t.Fatalf("RefreshAccessWindow = %v", Config.RefreshAccessWindow)
	}
	if Config.HotAccessThreshold != 10 {
		t.Fatalf("HotAccessThreshold = %d", Config.HotAccessThreshold)
	}
	if Config.HotChangedRefreshInterval != 2*time.Hour {
		t.Fatalf("HotChangedRefreshInterval = %v", Config.HotChangedRefreshInterval)
	}
	if Config.HotUnchangedRefreshInterval != 6*time.Hour {
		t.Fatalf("HotUnchangedRefreshInterval = %v", Config.HotUnchangedRefreshInterval)
	}
	if Config.NormalChangedRefreshInterval != 12*time.Hour {
		t.Fatalf("NormalChangedRefreshInterval = %v", Config.NormalChangedRefreshInterval)
	}
	if Config.StableRefreshInterval != 72*time.Hour {
		t.Fatalf("StableRefreshInterval = %v", Config.StableRefreshInterval)
	}
	if Config.ArchivedRefreshInterval != 168*time.Hour {
		t.Fatalf("ArchivedRefreshInterval = %v", Config.ArchivedRefreshInterval)
	}
}

func TestLoadConfigUsesDynamicRefreshOverrides(t *testing.T) {
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("change working directory: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalDir)
	})

	t.Setenv("REFRESH_ACCESS_WINDOW", "43200")
	t.Setenv("HOT_ACCESS_THRESHOLD", "20")
	t.Setenv("HOT_CHANGED_REFRESH_INTERVAL", "3600")
	t.Setenv("HOT_UNCHANGED_REFRESH_INTERVAL", "7200")
	t.Setenv("NORMAL_CHANGED_REFRESH_INTERVAL", "21600")
	t.Setenv("STABLE_REFRESH_INTERVAL", "172800")
	t.Setenv("ARCHIVED_REFRESH_INTERVAL", "1209600")

	if err := LoadConfig(); err != nil {
		t.Fatalf("LoadConfig returned error without .env: %v", err)
	}

	if Config.RefreshAccessWindow != 12*time.Hour {
		t.Fatalf("RefreshAccessWindow = %v", Config.RefreshAccessWindow)
	}
	if Config.HotAccessThreshold != 20 {
		t.Fatalf("HotAccessThreshold = %d", Config.HotAccessThreshold)
	}
	if Config.HotChangedRefreshInterval != time.Hour {
		t.Fatalf("HotChangedRefreshInterval = %v", Config.HotChangedRefreshInterval)
	}
	if Config.HotUnchangedRefreshInterval != 2*time.Hour {
		t.Fatalf("HotUnchangedRefreshInterval = %v", Config.HotUnchangedRefreshInterval)
	}
	if Config.NormalChangedRefreshInterval != 6*time.Hour {
		t.Fatalf("NormalChangedRefreshInterval = %v", Config.NormalChangedRefreshInterval)
	}
	if Config.StableRefreshInterval != 48*time.Hour {
		t.Fatalf("StableRefreshInterval = %v", Config.StableRefreshInterval)
	}
	if Config.ArchivedRefreshInterval != 14*24*time.Hour {
		t.Fatalf("ArchivedRefreshInterval = %v", Config.ArchivedRefreshInterval)
	}
}

func TestLoadConfigSupportsLegacyEmptyCommentsRefreshInterval(t *testing.T) {
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("change working directory: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalDir)
	})

	t.Setenv("EMPTY_COMMENTS_REFRESH_INTERVAL", "7200")

	if err := LoadConfig(); err != nil {
		t.Fatalf("LoadConfig returned error without .env: %v", err)
	}

	if Config.EmptyDanmakuRefreshInterval != 2*time.Hour {
		t.Fatalf("EmptyDanmakuRefreshInterval = %v", Config.EmptyDanmakuRefreshInterval)
	}
}

func TestLoadConfigPrefersEmptyDanmakuRefreshInterval(t *testing.T) {
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("change working directory: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalDir)
	})

	t.Setenv("EMPTY_COMMENTS_REFRESH_INTERVAL", "7200")
	t.Setenv("EMPTY_DANMAKU_REFRESH_INTERVAL", "1800")

	if err := LoadConfig(); err != nil {
		t.Fatalf("LoadConfig returned error without .env: %v", err)
	}

	if Config.EmptyDanmakuRefreshInterval != 30*time.Minute {
		t.Fatalf("EmptyDanmakuRefreshInterval = %v", Config.EmptyDanmakuRefreshInterval)
	}
}
