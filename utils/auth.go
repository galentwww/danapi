package utils

import (
	"crypto/sha256"
	"dandanplay-middleware/config"
	"encoding/base64"
	"fmt"
	"time"
)

// GenerateAuthHeaders 生成API请求的鉴权头
// path: API路径（不包含域名和查询参数）
// 返回包含所有必要鉴权头的map
func GenerateAuthHeaders(path string) map[string]string {
	timestamp := time.Now().Unix()
	signature := generateSignature(path, timestamp)

	return map[string]string{
		"X-AppId":     config.Config.AppId,
		"X-Timestamp": fmt.Sprintf("%d", timestamp),
		"X-Signature": signature,
	}
}

// generateSignature 生成API请求签名
func generateSignature(path string, timestamp int64) string {
	// 按顺序拼接：AppId + Timestamp + Path + AppSecret
	data := fmt.Sprintf("%s%d%s%s",
		config.Config.AppId,
		timestamp,
		path,
		config.Config.AppSecret)

	// 计算SHA256
	hash := sha256.Sum256([]byte(data))

	// 转换为Base64
	return base64.StdEncoding.EncodeToString(hash[:])
} 