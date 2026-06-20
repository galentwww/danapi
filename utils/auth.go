package utils

import (
	"crypto/sha256"
	"dandanplay-middleware/config"
	"encoding/base64"
	"fmt"
	"sync/atomic"
	"time"
)

type CredentialProvider interface {
	Next() CredentialSelection
}

type CredentialSelection struct {
	Credential config.DandanplayCredential
	Index      int
}

type RoundRobinCredentialProvider struct {
	credentials []config.DandanplayCredential
	next        uint64
}

func NewRoundRobinCredentialProvider(credentials []config.DandanplayCredential) *RoundRobinCredentialProvider {
	copied := make([]config.DandanplayCredential, len(credentials))
	copy(copied, credentials)
	return &RoundRobinCredentialProvider{credentials: copied}
}

func (p *RoundRobinCredentialProvider) Next() CredentialSelection {
	if p == nil {
		return CredentialSelection{Credential: defaultCredential(), Index: 0}
	}
	credentials := p.credentials
	if len(credentials) == 0 {
		credentials = config.Config.DandanplayCredentials
	}
	if len(credentials) == 0 {
		return CredentialSelection{Credential: defaultCredential(), Index: 0}
	}
	index := atomic.AddUint64(&p.next, 1) - 1
	selectedIndex := int(index % uint64(len(credentials)))
	return CredentialSelection{
		Credential: credentials[selectedIndex],
		Index:      selectedIndex,
	}
}

// GenerateAuthHeaders 生成API请求的鉴权头
// path: API路径（不包含域名和查询参数）
// 返回包含所有必要鉴权头的map
func GenerateAuthHeaders(path string) map[string]string {
	return GenerateAuthHeadersForCredential(path, time.Now().Unix(), defaultCredential())
}

func GenerateAuthHeadersForCredential(path string, timestamp int64, credential config.DandanplayCredential) map[string]string {
	signature := generateSignature(path, timestamp, credential)

	return map[string]string{
		"X-AppId":     credential.AppID,
		"X-Timestamp": fmt.Sprintf("%d", timestamp),
		"X-Signature": signature,
	}
}

// generateSignature 生成API请求签名
func generateSignature(path string, timestamp int64, credential config.DandanplayCredential) string {
	// 按顺序拼接：AppId + Timestamp + Path + AppSecret
	data := fmt.Sprintf("%s%d%s%s",
		credential.AppID,
		timestamp,
		path,
		credential.AppSecret)

	// 计算SHA256
	hash := sha256.Sum256([]byte(data))

	// 转换为Base64
	return base64.StdEncoding.EncodeToString(hash[:])
}

func defaultCredential() config.DandanplayCredential {
	if len(config.Config.DandanplayCredentials) > 0 {
		return config.Config.DandanplayCredentials[0]
	}
	return config.DandanplayCredential{
		AppID:     config.Config.AppId,
		AppSecret: config.Config.AppSecret,
	}
}

func MaskCredentialAppID(appID string) string {
	if len(appID) <= 4 {
		return "***"
	}
	return appID[:2] + "***" + appID[len(appID)-2:]
}
