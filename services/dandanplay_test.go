package services

import (
	"bytes"
	"dandanplay-middleware/config"
	"dandanplay-middleware/utils"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type sequenceCredentialProvider struct {
	credentials []config.DandanplayCredential
	next        int
}

func (p *sequenceCredentialProvider) Next() utils.CredentialSelection {
	credential := p.credentials[p.next%len(p.credentials)]
	index := p.next % len(p.credentials)
	p.next++
	return utils.CredentialSelection{Credential: credential, Index: index}
}

func TestDandanplayServiceRotatesCredentialsAcrossRequests(t *testing.T) {
	seenAppIDs := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAppIDs = append(seenAppIDs, r.Header.Get("X-AppId"))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer server.Close()

	config.Config.DandanplayBaseURL = server.URL
	service := NewDandanplayServiceWithCredentialProvider(&sequenceCredentialProvider{
		credentials: []config.DandanplayCredential{
			{AppID: "app-a", AppSecret: "secret-a"},
			{AppID: "app-b", AppSecret: "secret-b"},
		},
	})

	if _, err := service.SearchEpisodes("anime=test"); err != nil {
		t.Fatalf("first SearchEpisodes returned error: %v", err)
	}
	if _, err := service.SearchEpisodes("anime=test"); err != nil {
		t.Fatalf("second SearchEpisodes returned error: %v", err)
	}

	if len(seenAppIDs) != 2 {
		t.Fatalf("seenAppIDs len = %d", len(seenAppIDs))
	}
	if seenAppIDs[0] != "app-a" {
		t.Fatalf("first X-AppId = %q", seenAppIDs[0])
	}
	if seenAppIDs[1] != "app-b" {
		t.Fatalf("second X-AppId = %q", seenAppIDs[1])
	}
}

func TestDandanplayServiceUsesSameCredentialForRedirect(t *testing.T) {
	seenAppIDs := make([]string, 0, 2)
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAppIDs = append(seenAppIDs, r.Header.Get("X-AppId"))
		if r.URL.Path == "/api/v2/comment/123" {
			http.Redirect(w, r, server.URL+"/redirected/comment/123", http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"count":0,"comments":[]}`))
	}))
	defer server.Close()

	config.Config.DandanplayBaseURL = server.URL
	service := NewDandanplayServiceWithCredentialProvider(&sequenceCredentialProvider{
		credentials: []config.DandanplayCredential{
			{AppID: "app-a", AppSecret: "secret-a"},
			{AppID: "app-b", AppSecret: "secret-b"},
		},
	})

	if _, err := service.FetchComments(t.Context(), "123", "withRelated=true"); err != nil {
		t.Fatalf("FetchComments returned error: %v", err)
	}

	if len(seenAppIDs) != 2 {
		t.Fatalf("seenAppIDs len = %d", len(seenAppIDs))
	}
	if seenAppIDs[0] != "app-a" {
		t.Fatalf("first X-AppId = %q", seenAppIDs[0])
	}
	if seenAppIDs[1] != "app-a" {
		t.Fatalf("redirect X-AppId = %q", seenAppIDs[1])
	}
}

func TestDandanplayServiceDoesNotLogCredentialByDefault(t *testing.T) {
	originalConfig := config.Config
	t.Cleanup(func() {
		config.Config = originalConfig
	})

	var logBuffer bytes.Buffer
	originalLogWriter := log.Writer()
	log.SetOutput(&logBuffer)
	t.Cleanup(func() {
		log.SetOutput(originalLogWriter)
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer server.Close()

	config.Config = config.Configuration{
		DandanplayBaseURL: server.URL,
	}
	service := NewDandanplayServiceWithCredentialProvider(&sequenceCredentialProvider{
		credentials: []config.DandanplayCredential{
			{AppID: "abcdef1234", AppSecret: "secret-value"},
		},
	})

	if _, err := service.SearchEpisodes("anime=test"); err != nil {
		t.Fatalf("SearchEpisodes returned error: %v", err)
	}

	if logBuffer.Len() != 0 {
		t.Fatalf("log output = %q", logBuffer.String())
	}
}

func TestDandanplayServiceLogsMaskedCredentialWhenEnabled(t *testing.T) {
	originalConfig := config.Config
	t.Cleanup(func() {
		config.Config = originalConfig
	})

	var logBuffer bytes.Buffer
	originalLogWriter := log.Writer()
	log.SetOutput(&logBuffer)
	t.Cleanup(func() {
		log.SetOutput(originalLogWriter)
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer server.Close()

	config.Config = config.Configuration{
		DandanplayBaseURL:       server.URL,
		DandanplayCredentialLog: true,
	}
	service := NewDandanplayServiceWithCredentialProvider(&sequenceCredentialProvider{
		credentials: []config.DandanplayCredential{
			{AppID: "abcdef1234", AppSecret: "secret-value"},
		},
	})

	if _, err := service.SearchEpisodes("anime=test"); err != nil {
		t.Fatalf("SearchEpisodes returned error: %v", err)
	}

	output := logBuffer.String()
	if !strings.Contains(output, "DandanPlay credential selected") {
		t.Fatalf("log output = %q", output)
	}
	if !strings.Contains(output, "credential_index=1") {
		t.Fatalf("log output missing credential index: %q", output)
	}
	if !strings.Contains(output, "app_id=ab***34") {
		t.Fatalf("log output missing masked AppID: %q", output)
	}
	if strings.Contains(output, "abcdef1234") {
		t.Fatalf("log output contains full AppID: %q", output)
	}
	if strings.Contains(output, "secret-value") {
		t.Fatalf("log output contains AppSecret: %q", output)
	}
}
