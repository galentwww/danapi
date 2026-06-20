package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Configuration 配置结构体
// 包含所有应用程序需要的配置项
type Configuration struct {
	DandanplayBaseURL       string                 // 弹弹Play API的基础URL
	RedisHost               string                 // Redis服务器地址
	RedisPort               string                 // Redis服务器端口
	RedisPassword           string                 // Redis密码
	RedisDB                 int                    // Redis数据库编号
	ServerPort              string                 // 服务器监听端口
	SearchCacheDuration     time.Duration          // 搜索结果的缓存时间
	DanmakuCacheDuration    time.Duration          // 弹幕数据的缓存时间
	AppId                   string                 // API鉴权AppId
	AppSecret               string                 // API鉴权AppSecret
	DandanplayCredentials   []DandanplayCredential // 弹弹Play API鉴权凭据列表
	DandanplayCredentialLog bool                   // 是否记录上游凭据选择日志
	DatabaseURL             string                 // PostgreSQL连接字符串

	// 弹幕快照刷新配置
	RedisSnapshotTTL            time.Duration // Redis热快照驻留时间
	DefaultRefreshInterval      time.Duration // 默认上游刷新间隔
	EmptyDanmakuRefreshInterval time.Duration // 空弹幕刷新间隔
	RefreshFailureRetryInterval time.Duration // 刷新失败重试间隔
	RefreshQueueSize            int           // 后台刷新队列容量
	RefreshWorkerCount          int           // 后台刷新worker数量

	// CORS 相关配置
	CORSAllowOrigins     string // 允许的来源，多个用英文逗号分隔，支持 * 与 *.example.com
	CORSAllowMethods     string // 允许的方法，多个用英文逗号分隔
	CORSAllowHeaders     string // 允许的请求头，多个用英文逗号分隔
	CORSExposeHeaders    string // 暴露给浏览器的响应头，多个用英文逗号分隔
	CORSAllowCredentials bool   // 是否允许携带 Cookie/凭证
	CORSMaxAge           int    // 预检请求结果缓存秒数
}

// DandanplayCredential is one AppId/AppSecret pair used for upstream signing.
type DandanplayCredential struct {
	AppID     string
	AppSecret string
}

// Config 全局配置实例
var Config Configuration

// parseDuration 解析环境变量中的时间配置
// 如果解析失败则返回默认值
func parseDuration(env string, defaultDuration time.Duration) time.Duration {
	if durationStr := os.Getenv(env); durationStr != "" {
		if duration, err := time.ParseDuration(durationStr + "s"); err == nil {
			return duration
		}
	}
	return defaultDuration
}

// parseDurationWithFallback parses the primary environment variable first, then
// falls back to a legacy name for existing deployments.
func parseDurationWithFallback(env, fallbackEnv string, defaultDuration time.Duration) time.Duration {
	if strings.TrimSpace(os.Getenv(env)) != "" {
		return parseDuration(env, defaultDuration)
	}
	return parseDuration(fallbackEnv, defaultDuration)
}

// getEnvDefault 读取环境变量，未设置时返回默认值
func getEnvDefault(env, defaultValue string) string {
	if v := strings.TrimSpace(os.Getenv(env)); v != "" {
		return v
	}
	return defaultValue
}

// getEnvBool 读取布尔类型环境变量
func getEnvBool(env string, defaultValue bool) bool {
	v := strings.TrimSpace(os.Getenv(env))
	if v == "" {
		return defaultValue
	}
	if b, err := strconv.ParseBool(v); err == nil {
		return b
	}
	return defaultValue
}

// getEnvInt 读取整型环境变量
func getEnvInt(env string, defaultValue int) int {
	v := strings.TrimSpace(os.Getenv(env))
	if v == "" {
		return defaultValue
	}
	if i, err := strconv.Atoi(v); err == nil {
		return i
	}
	return defaultValue
}

func parseDandanplayCredentials(keys string, legacyAppID string, legacyAppSecret string) ([]DandanplayCredential, error) {
	keys = strings.TrimSpace(keys)
	if keys == "" {
		if strings.TrimSpace(legacyAppID) != "" && strings.TrimSpace(legacyAppSecret) != "" {
			return []DandanplayCredential{{
				AppID:     strings.TrimSpace(legacyAppID),
				AppSecret: strings.TrimSpace(legacyAppSecret),
			}}, nil
		}
		return nil, nil
	}

	parts := strings.Split(keys, ",")
	credentials := make([]DandanplayCredential, 0, len(parts))
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		pair := strings.SplitN(part, ":", 2)
		if len(pair) != 2 {
			return nil, fmt.Errorf("invalid DANDANPLAY_KEYS entry %d: expected app_id:app_secret", i+1)
		}
		appID := strings.TrimSpace(pair[0])
		appSecret := strings.TrimSpace(pair[1])
		if appID == "" || appSecret == "" {
			return nil, fmt.Errorf("invalid DANDANPLAY_KEYS entry %d: app_id and app_secret are required", i+1)
		}
		credentials = append(credentials, DandanplayCredential{
			AppID:     appID,
			AppSecret: appSecret,
		})
	}
	if len(credentials) == 0 {
		return nil, fmt.Errorf("DANDANPLAY_KEYS did not contain any credentials")
	}
	return credentials, nil
}

// LoadConfig 从.env文件加载配置
// 设置全局Config变量
func LoadConfig() error {
	if err := godotenv.Load(); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	}

	appID := os.Getenv("APP_ID")
	appSecret := os.Getenv("APP_SECRET")
	credentials, err := parseDandanplayCredentials(os.Getenv("DANDANPLAY_KEYS"), appID, appSecret)
	if err != nil {
		return err
	}

	Config = Configuration{
		DandanplayBaseURL:       os.Getenv("DANDANPLAY_BASE_URL"),
		RedisHost:               os.Getenv("REDIS_HOST"),
		RedisPort:               os.Getenv("REDIS_PORT"),
		RedisPassword:           os.Getenv("REDIS_PASSWORD"),
		RedisDB:                 getEnvInt("REDIS_DB", 0),
		ServerPort:              os.Getenv("SERVER_PORT"),
		SearchCacheDuration:     parseDuration("SEARCH_CACHE_DURATION", 1*time.Hour),     // 默认1小时
		DanmakuCacheDuration:    parseDuration("DANMAKU_CACHE_DURATION", 30*time.Minute), // 默认30分钟
		AppId:                   appID,
		AppSecret:               appSecret,
		DandanplayCredentials:   credentials,
		DandanplayCredentialLog: getEnvBool("DANDANPLAY_CREDENTIAL_LOG", false),
		DatabaseURL:             os.Getenv("DATABASE_URL"),

		RedisSnapshotTTL:            parseDuration("REDIS_SNAPSHOT_TTL", 48*time.Hour),
		DefaultRefreshInterval:      parseDuration("DEFAULT_REFRESH_INTERVAL", 24*time.Hour),
		EmptyDanmakuRefreshInterval: parseDurationWithFallback("EMPTY_DANMAKU_REFRESH_INTERVAL", "EMPTY_COMMENTS_REFRESH_INTERVAL", 1*time.Hour),
		RefreshFailureRetryInterval: parseDuration("REFRESH_FAILURE_RETRY_INTERVAL", 30*time.Minute),
		RefreshQueueSize:            getEnvInt("REFRESH_QUEUE_SIZE", 100),
		RefreshWorkerCount:          getEnvInt("REFRESH_WORKER_COUNT", 2),

		CORSAllowOrigins:     getEnvDefault("CORS_ALLOW_ORIGINS", "*"),
		CORSAllowMethods:     getEnvDefault("CORS_ALLOW_METHODS", "GET,POST,PUT,DELETE,OPTIONS,PATCH,HEAD"),
		CORSAllowHeaders:     getEnvDefault("CORS_ALLOW_HEADERS", "Origin,Content-Type,Accept,Authorization,X-Requested-With"),
		CORSExposeHeaders:    os.Getenv("CORS_EXPOSE_HEADERS"),
		CORSAllowCredentials: getEnvBool("CORS_ALLOW_CREDENTIALS", false),
		CORSMaxAge:           getEnvInt("CORS_MAX_AGE", 86400),
	}

	return nil
}
