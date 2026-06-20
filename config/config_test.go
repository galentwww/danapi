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
}
