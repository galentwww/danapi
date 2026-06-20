package services

import (
	"context"
	"dandanplay-middleware/config"
	"dandanplay-middleware/utils"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"
)

// DandanplayService 弹弹Play API服务
type DandanplayService struct {
	client             *http.Client // HTTP客户端，配置为不自动跟随重定向
	credentialProvider utils.CredentialProvider
}

// NewDandanplayService 创建新的弹弹Play服务实例
func NewDandanplayService() *DandanplayService {
	return NewDandanplayServiceWithCredentialProvider(
		utils.NewRoundRobinCredentialProvider(config.Config.DandanplayCredentials),
	)
}

func NewDandanplayServiceWithCredentialProvider(provider utils.CredentialProvider) *DandanplayService {
	return &DandanplayService{
		client: &http.Client{
			// 禁用自动重定向，以便我们可以手动处理302响应
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		credentialProvider: provider,
	}
}

// SearchEpisodes 搜索剧集
// query: 完整的查询字符串
// 返回原始JSON响应数据
func (s *DandanplayService) SearchEpisodes(query string) ([]byte, error) {
	path := "/api/v2/search/episodes"
	url := fmt.Sprintf("%s%s?%s", config.Config.DandanplayBaseURL, path, query)

	// 创建请求
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	credential := s.nextCredential()
	// 添加鉴权头
	s.setAuthHeaders(req, path, credential.Credential)

	// 发送请求
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

// GetDanmaku 获取弹幕数据
// id: 剧集ID
// query: 完整的查询字符串
// 返回原始JSON响应数据
func (s *DandanplayService) GetDanmaku(id string, query string) ([]byte, error) {
	return s.FetchComments(context.Background(), id, query)
}

func (s *DandanplayService) FetchComments(ctx context.Context, id string, query string) ([]byte, error) {
	path := fmt.Sprintf("/api/v2/comment/%s", id)
	url := fmt.Sprintf("%s%s?%s", config.Config.DandanplayBaseURL, path, simplifiedChineseQuery(query))

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	credential := s.nextCredential()
	// 添加鉴权头
	s.setAuthHeaders(req, path, credential.Credential)

	// 发送请求
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// 处理302重定向
	if resp.StatusCode == http.StatusFound {
		redirectURL := resp.Header.Get("Location")
		req, err = http.NewRequestWithContext(ctx, "GET", redirectURL, nil)
		if err != nil {
			return nil, err
		}

		// 重定向的URL也需要添加鉴权头
		s.setAuthHeaders(req, path, credential.Credential)

		resp, err = s.client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
	}

	return io.ReadAll(resp.Body)
}

// GetBangumiByBgmtvSubjectID 通过Bangumi.tv subjectId获取番剧详情
// id: Bangumi.tv subjectId
// 返回原始JSON响应数据
func (s *DandanplayService) GetBangumiByBgmtvSubjectID(id string) ([]byte, error) {
	path := fmt.Sprintf("/api/v2/bangumi/bgmtv/%s", id)
	url := fmt.Sprintf("%s%s", config.Config.DandanplayBaseURL, path)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	credential := s.nextCredential()
	s.setAuthHeaders(req, path, credential.Credential)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

func (s *DandanplayService) nextCredential() utils.CredentialSelection {
	if s.credentialProvider == nil {
		selection := utils.CredentialSelection{
			Credential: config.DandanplayCredential{
				AppID:     config.Config.AppId,
				AppSecret: config.Config.AppSecret,
			},
			Index: 0,
		}
		s.logCredentialSelection(selection)
		return selection
	}
	selection := s.credentialProvider.Next()
	s.logCredentialSelection(selection)
	return selection
}

func (s *DandanplayService) setAuthHeaders(req *http.Request, path string, credential config.DandanplayCredential) {
	for key, value := range utils.GenerateAuthHeadersForCredential(path, time.Now().Unix(), credential) {
		req.Header.Set(key, value)
	}
}

func (s *DandanplayService) logCredentialSelection(selection utils.CredentialSelection) {
	if !config.Config.DandanplayCredentialLog {
		return
	}
	log.Printf(
		"DandanPlay credential selected - credential_index=%d app_id=%s",
		selection.Index+1,
		utils.MaskCredentialAppID(selection.Credential.AppID),
	)
}

func simplifiedChineseQuery(raw string) string {
	values, err := url.ParseQuery(raw)
	if err != nil {
		if raw == "" {
			return "chConvert=1"
		}
		return raw + "&chConvert=1"
	}
	values.Set("chConvert", "1")
	return values.Encode()
}
