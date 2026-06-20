package utils

import (
	"crypto/sha256"
	"dandanplay-middleware/config"
	"encoding/base64"
	"fmt"
	"testing"
)

func TestGenerateAuthHeadersForCredentialSignsWithSelectedCredential(t *testing.T) {
	credential := config.DandanplayCredential{
		AppID:     "app-a",
		AppSecret: "secret-a",
	}
	timestamp := int64(1710000000)
	path := "/api/v2/comment/123"

	headers := GenerateAuthHeadersForCredential(path, timestamp, credential)

	expectedSignature := expectedSignature(credential.AppID, timestamp, path, credential.AppSecret)
	if headers["X-AppId"] != credential.AppID {
		t.Fatalf("X-AppId = %q", headers["X-AppId"])
	}
	if headers["X-Timestamp"] != fmt.Sprintf("%d", timestamp) {
		t.Fatalf("X-Timestamp = %q", headers["X-Timestamp"])
	}
	if headers["X-Signature"] != expectedSignature {
		t.Fatalf("X-Signature = %q, want %q", headers["X-Signature"], expectedSignature)
	}
}

func TestRoundRobinCredentialProviderCyclesCredentials(t *testing.T) {
	provider := NewRoundRobinCredentialProvider([]config.DandanplayCredential{
		{AppID: "app-a", AppSecret: "secret-a"},
		{AppID: "app-b", AppSecret: "secret-b"},
	})

	first := provider.Next()
	second := provider.Next()
	third := provider.Next()

	if first.Credential.AppID != "app-a" {
		t.Fatalf("first AppID = %q", first.Credential.AppID)
	}
	if first.Index != 0 {
		t.Fatalf("first Index = %d", first.Index)
	}
	if second.Credential.AppID != "app-b" {
		t.Fatalf("second AppID = %q", second.Credential.AppID)
	}
	if second.Index != 1 {
		t.Fatalf("second Index = %d", second.Index)
	}
	if third.Credential.AppID != "app-a" {
		t.Fatalf("third AppID = %q", third.Credential.AppID)
	}
	if third.Index != 0 {
		t.Fatalf("third Index = %d", third.Index)
	}
}

func TestRoundRobinCredentialProviderUsesCurrentConfigWhenConstructedEmpty(t *testing.T) {
	originalConfig := config.Config
	t.Cleanup(func() {
		config.Config = originalConfig
	})
	config.Config = config.Configuration{
		DandanplayCredentials: []config.DandanplayCredential{
			{AppID: "app-a", AppSecret: "secret-a"},
			{AppID: "app-b", AppSecret: "secret-b"},
		},
	}
	provider := NewRoundRobinCredentialProvider(nil)

	first := provider.Next()
	second := provider.Next()
	third := provider.Next()

	if first.Credential.AppID != "app-a" {
		t.Fatalf("first AppID = %q", first.Credential.AppID)
	}
	if first.Index != 0 {
		t.Fatalf("first Index = %d", first.Index)
	}
	if second.Credential.AppID != "app-b" {
		t.Fatalf("second AppID = %q", second.Credential.AppID)
	}
	if second.Index != 1 {
		t.Fatalf("second Index = %d", second.Index)
	}
	if third.Credential.AppID != "app-a" {
		t.Fatalf("third AppID = %q", third.Credential.AppID)
	}
	if third.Index != 0 {
		t.Fatalf("third Index = %d", third.Index)
	}
}

func TestMaskCredentialAppID(t *testing.T) {
	if got := MaskCredentialAppID("abcdef1234"); got != "ab***34" {
		t.Fatalf("MaskCredentialAppID = %q", got)
	}
	if got := MaskCredentialAppID("abc"); got != "***" {
		t.Fatalf("MaskCredentialAppID short value = %q", got)
	}
}

func expectedSignature(appID string, timestamp int64, path string, appSecret string) string {
	data := fmt.Sprintf("%s%d%s%s", appID, timestamp, path, appSecret)
	hash := sha256.Sum256([]byte(data))
	return base64.StdEncoding.EncodeToString(hash[:])
}
