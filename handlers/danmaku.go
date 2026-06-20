package handlers

import (
	"dandanplay-middleware/config"
	"dandanplay-middleware/services"
	danmakuService "dandanplay-middleware/services/danmaku"
	"dandanplay-middleware/utils"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
)

// dandanplayService 全局服务实例
var dandanplayService = services.NewDandanplayService()
var commentService *danmakuService.CommentService

// SetCommentService 注入弹幕快照服务
func SetCommentService(service *danmakuService.CommentService) {
	commentService = service
}

// EmptyRelatedResponse 空关联数据响应结构
type EmptyRelatedResponse struct {
	Relateds     []interface{} `json:"relateds"`
	ErrorCode    int           `json:"errorCode"`
	Success      bool          `json:"success"`
	ErrorMessage string        `json:"errorMessage"`
}

// GetRelated 处理获取关联数据的请求（兼容旧版本）
// 始终返回空数据
func GetRelated(c *gin.Context) {
	response := EmptyRelatedResponse{
		Relateds:     []interface{}{},
		ErrorCode:    0,
		Success:      true,
		ErrorMessage: "",
	}

	log.Printf("处理旧版本关联数据请求 - ID: %s", c.Param("id"))
	c.JSON(http.StatusOK, response)
}

// SearchEpisodes 处理搜索剧集的请求
// 支持缓存搜索结果
func SearchEpisodes(c *gin.Context) {
	query := c.Request.URL.RawQuery
	cacheKey := fmt.Sprintf("search:%s", query)

	// 检查缓存TTL
	ttl := utils.GetCacheTTL(cacheKey)
	log.Printf("搜索请求 - Key: %s, 剩余TTL: %v", cacheKey, ttl)

	// 尝试从缓存获取数据
	var cachedData []byte
	err := utils.GetCache(cacheKey, &cachedData)
	if err == nil {
		log.Printf("返回缓存的搜索结果 - Key: %s", cacheKey)
		c.Data(http.StatusOK, "application/json", cachedData)
		return
	}

	log.Printf("从API获取搜索结果 - Query: %s", query)
	// 缓存未命中，从API获取数据
	data, err := dandanplayService.SearchEpisodes(query)
	if err != nil {
		log.Printf("API请求失败 - Error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 将结果存入缓存
	_ = utils.SetCache(cacheKey, data, config.Config.SearchCacheDuration)
	log.Printf("API结果已缓存 - Key: %s", cacheKey)

	c.Data(http.StatusOK, "application/json", data)
}

// GetDanmaku 处理获取弹幕的请求
// 支持缓存弹幕数据
func GetDanmaku(c *gin.Context) {
	if commentService != nil {
		data, err := commentService.GetComments(c.Request.Context(), c.Param("id"), c.Request.URL.Query())
		if err != nil {
			log.Printf("弹幕服务请求失败 - ID: %s, Error: %v", c.Param("id"), err)
			status := http.StatusInternalServerError
			if errors.Is(err, danmakuService.ErrUpstreamUnavailable) {
				status = http.StatusServiceUnavailable
			}
			c.JSON(status, gin.H{"error": err.Error()})
			return
		}
		c.Data(http.StatusOK, "application/json", data)
		return
	}

	id := c.Param("id")
	query := c.Request.URL.RawQuery
	cacheKey := fmt.Sprintf("danmaku:%s:%s", id, query)

	// 检查缓存TTL
	ttl := utils.GetCacheTTL(cacheKey)
	log.Printf("弹幕请求 - Key: %s, 剩余TTL: %v", cacheKey, ttl)

	// 尝试从缓存获取数据
	var cachedData []byte
	err := utils.GetCache(cacheKey, &cachedData)
	if err == nil {
		log.Printf("返回缓存的弹幕数据 - Key: %s", cacheKey)
		c.Data(http.StatusOK, "application/json", cachedData)
		return
	}

	log.Printf("从API获取弹幕数据 - ID: %s", id)
	// 缓存未命中，从API获取数据
	data, err := dandanplayService.GetDanmaku(id, query)
	if err != nil {
		log.Printf("API请求失败 - Error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 将结果存入缓存
	_ = utils.SetCache(cacheKey, data, config.Config.DanmakuCacheDuration)
	log.Printf("API结果已缓存 - Key: %s", cacheKey)

	c.Data(http.StatusOK, "application/json", data)
}

// GetBangumiByBgmtvSubjectID 通过Bangumi.tv subjectId获取弹弹Play番剧详情
// 支持缓存公共映射结果
func GetBangumiByBgmtvSubjectID(c *gin.Context) {
	id := c.Param("id")
	cacheKey := fmt.Sprintf("bangumi:bgmtv:%s", id)

	ttl := utils.GetCacheTTL(cacheKey)
	log.Printf("Bangumi.tv映射请求 - Key: %s, 剩余TTL: %v", cacheKey, ttl)

	var cachedData []byte
	err := utils.GetCache(cacheKey, &cachedData)
	if err == nil {
		log.Printf("返回缓存的Bangumi.tv映射数据 - Key: %s", cacheKey)
		c.Data(http.StatusOK, "application/json", cachedData)
		return
	}

	log.Printf("从API获取Bangumi.tv映射数据 - SubjectID: %s", id)
	data, err := dandanplayService.GetBangumiByBgmtvSubjectID(id)
	if err != nil {
		log.Printf("API请求失败 - Error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	_ = utils.SetCache(cacheKey, data, config.Config.SearchCacheDuration)
	log.Printf("API结果已缓存 - Key: %s", cacheKey)

	c.Data(http.StatusOK, "application/json", data)
}
