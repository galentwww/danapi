package utils

import (
	"dandanplay-middleware/config"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// CORS 返回一个根据配置生成的 CORS 中间件
// 支持通过 .env 自定义允许的 Origin、Method、Header、是否携带凭证、预检缓存时间等
func CORS() gin.HandlerFunc {
	allowOrigins := splitAndTrim(config.Config.CORSAllowOrigins)
	allowMethods := joinIfEmpty(config.Config.CORSAllowMethods, "GET,POST,PUT,DELETE,OPTIONS,PATCH,HEAD")
	allowHeaders := joinIfEmpty(config.Config.CORSAllowHeaders, "Origin,Content-Type,Accept,Authorization,X-Requested-With")
	exposeHeaders := config.Config.CORSExposeHeaders
	allowCredentials := config.Config.CORSAllowCredentials
	maxAge := strconv.Itoa(config.Config.CORSMaxAge)

	allowAll := len(allowOrigins) == 0 || contains(allowOrigins, "*")

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		originMatched := origin != "" && (allowAll || originAllowed(origin, allowOrigins))

		// 声明按 Origin 分缓存键。多数 CDN（含腾讯云 CDN / EdgeOne 默认）不认
		// Vary: Origin，所以这只是给浏览器和合规中间层用的，真正防污染靠下面两件事：
		// 1) 不管 Origin 是否匹配，永远下发一个确定的 ACAO（不匹配时给 "null"）
		// 2) 对带 Origin 的请求强制 no-store，把浏览器流量从 CDN 缓存路径上摘出去
		// 用 Add 追加而不是 Set 覆盖，避免清掉 Vary: Accept-Encoding。
		c.Writer.Header().Add("Vary", "Origin")

		switch {
		case allowAll && !allowCredentials:
			c.Header("Access-Control-Allow-Origin", "*")
		case originMatched:
			// 命中白名单（含 allowAll+credentials 场景），回显请求方 Origin
			c.Header("Access-Control-Allow-Origin", origin)
		default:
			// 没带 Origin 或不在白名单：仍然下发一个确定值，避免 CDN 把"无 ACAO
			// 的 200"缓存后回放给合法 Origin 触发 CORS 错误。浏览器看到 "null"
			// 与自身 Origin 不匹配会自行拦截，行为与原先"缺头"等价但对缓存友好。
			c.Header("Access-Control-Allow-Origin", "null")
		}

		if originMatched {
			if allowCredentials {
				c.Header("Access-Control-Allow-Credentials", "true")
			}
			if exposeHeaders != "" {
				c.Header("Access-Control-Expose-Headers", exposeHeaders)
			}
		}

		// 所有响应强制不走 CDN 缓存：响应内容依赖 Origin，而腾讯云 CDN 不支持
		// 按请求头分缓存键，任何被缓存的版本回放给不同 Origin 的请求都会污染。
		// 中间件自身有 Redis 缓存挡上游（见 DANMAKU_CACHE_DURATION），源站压力不增加。
		c.Header("Cache-Control", "private, no-store")

		// 预检请求
		if c.Request.Method == http.MethodOptions {
			c.Header("Access-Control-Allow-Methods", allowMethods)
			// 优先回显请求的 headers，未指定则使用配置
			if reqHeaders := c.GetHeader("Access-Control-Request-Headers"); reqHeaders != "" {
				c.Header("Access-Control-Allow-Headers", reqHeaders)
			} else {
				c.Header("Access-Control-Allow-Headers", allowHeaders)
			}
			c.Header("Access-Control-Max-Age", maxAge)
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// originAllowed 判断 origin 是否在允许列表中，支持以 * 开头的通配子域，例如 *.example.com
func originAllowed(origin string, allowList []string) bool {
	for _, item := range allowList {
		if item == origin {
			return true
		}
		if strings.HasPrefix(item, "*.") {
			suffix := item[1:] // ".example.com"
			if strings.HasSuffix(origin, suffix) {
				return true
			}
		}
	}
	return false
}

func splitAndTrim(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func joinIfEmpty(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}

func contains(list []string, target string) bool {
	for _, v := range list {
		if v == target {
			return true
		}
	}
	return false
}
