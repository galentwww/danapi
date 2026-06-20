package utils

import (
	"context"
	"dandanplay-middleware/config"
	"encoding/json"
	"fmt"
	"github.com/go-redis/redis/v8"
	"log"
	"time"
)

// RedisClient Redis客户端实例
var RedisClient *redis.Client
// Ctx 全局上下文
var Ctx = context.Background()

// InitRedis 初始化Redis连接
// 使用配置文件中的Redis设置创建连接
func InitRedis() error {
	RedisClient = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", config.Config.RedisHost, config.Config.RedisPort),
		Password: config.Config.RedisPassword,
		DB:       config.Config.RedisDB,
	})

	// 测试连接
	_, err := RedisClient.Ping(Ctx).Result()
	return err
}

// SetCache 将数据存入Redis缓存
// key: 缓存键
// value: 要缓存的数据
// duration: 缓存时间
func SetCache(key string, value interface{}, duration time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		log.Printf("缓存序列化失败 - Key: %s, Error: %v", key, err)
		return err
	}
	log.Printf("写入缓存 - Key: %s, 过期时间: %v", key, duration)
	return RedisClient.Set(Ctx, key, data, duration).Err()
}

// GetCache 从Redis缓存获取数据
// key: 缓存键
// dest: 用于存储获取到的数据的目标变量
func GetCache(key string, dest interface{}) error {
	data, err := RedisClient.Get(Ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			log.Printf("缓存未命中 - Key: %s", key)
		} else {
			log.Printf("获取缓存出错 - Key: %s, Error: %v", key, err)
		}
		return err
	}
	log.Printf("缓存命中 - Key: %s", key)
	return json.Unmarshal(data, dest)
}

// 添加新的辅助函数用于检查缓存状态
func GetCacheTTL(key string) time.Duration {
	ttl, err := RedisClient.TTL(Ctx, key).Result()
	if err != nil {
		log.Printf("获取缓存TTL出错 - Key: %s, Error: %v", key, err)
		return -1
	}
	return ttl
} 