package main

import (
	"context"
	"dandanplay-middleware/config"
	"dandanplay-middleware/handlers"
	"dandanplay-middleware/services"
	danmakuService "dandanplay-middleware/services/danmaku"
	"dandanplay-middleware/storage"
	"dandanplay-middleware/utils"
	"github.com/gin-gonic/gin"
	"log"
)

// main 程序入口
// 负责初始化配置、Redis连接，并启动HTTP服务器
func main() {
	// 从.env文件加载配置
	if err := config.LoadConfig(); err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	// 初始化Redis连接
	if err := utils.InitRedis(); err != nil {
		log.Fatalf("Error connecting to Redis: %v", err)
	}

	db, err := storage.OpenPostgres(context.Background(), config.Config.DatabaseURL)
	if err != nil {
		log.Fatalf("Error connecting to PostgreSQL: %v", err)
	}
	defer db.Close()
	if err := storage.Migrate(context.Background(), db); err != nil {
		log.Fatalf("Error running database migrations: %v", err)
	}

	refreshPolicy := danmakuService.RefreshPolicy{
		DefaultRefreshInterval:       config.Config.DefaultRefreshInterval,
		EmptyDanmakuRefreshInterval:  config.Config.EmptyDanmakuRefreshInterval,
		RefreshFailureRetryInterval:  config.Config.RefreshFailureRetryInterval,
		AccessWindow:                 config.Config.RefreshAccessWindow,
		HotAccessThreshold:           config.Config.HotAccessThreshold,
		HotChangedRefreshInterval:    config.Config.HotChangedRefreshInterval,
		HotUnchangedRefreshInterval:  config.Config.HotUnchangedRefreshInterval,
		NormalChangedRefreshInterval: config.Config.NormalChangedRefreshInterval,
		StableRefreshInterval:        config.Config.StableRefreshInterval,
		ArchivedRefreshInterval:      config.Config.ArchivedRefreshInterval,
	}
	commentService := danmakuService.NewCommentService(danmakuService.CommentServiceOptions{
		Cache:              danmakuService.NewRedisSnapshotCache(utils.RedisClient),
		Store:              storage.NewPostgresSnapshotStore(db),
		Upstream:           services.NewDandanplayService(),
		Policy:             refreshPolicy,
		RedisSnapshotTTL:   config.Config.RedisSnapshotTTL,
		RefreshQueueSize:   config.Config.RefreshQueueSize,
		RefreshWorkerCount: config.Config.RefreshWorkerCount,
	})
	defer commentService.Close()
	handlers.SetCommentService(commentService)

	// 创建Gin路由实例
	r := gin.Default()

	// 注册自定义 CORS 中间件（通过 .env 配置）
	r.Use(utils.CORS())

	// 注册API路由
	// 保持与弹弹Play API相同的路由结构
	r.GET("/api/v2/search/episodes", handlers.SearchEpisodes)               // 搜索剧集
	r.GET("/api/v2/comment/:id", handlers.GetDanmaku)                       // 获取弹幕
	r.GET("/api/v2/bangumi/bgmtv/:id", handlers.GetBangumiByBgmtvSubjectID) // 通过Bangumi.tv subjectId获取番剧详情
	r.GET("/api/v2/related/:id", handlers.GetRelated)                       // 获取关联数据（兼容旧版本）

	// 启动HTTP服务器
	log.Printf("Server starting on port %s", config.Config.ServerPort)
	if err := r.Run(":" + config.Config.ServerPort); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}
