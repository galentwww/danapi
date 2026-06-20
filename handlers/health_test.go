package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHealthzReturnsOKWithoutDependencyChecks(t *testing.T) {
	gin.SetMode(gin.TestMode)
	SetHealthChecks(func(context.Context) error {
		t.Fatal("redis check should not be called")
		return nil
	}, func(context.Context) error {
		t.Fatal("postgres check should not be called")
		return nil
	})

	router := gin.New()
	router.GET("/healthz", Healthz)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), `"status":"ok"`) {
		t.Fatalf("body = %s", recorder.Body.String())
	}
}

func TestReadyzReturnsOKWhenDependenciesAreHealthy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	SetHealthChecks(func(context.Context) error {
		return nil
	}, func(context.Context) error {
		return nil
	})

	router := gin.New()
	router.GET("/readyz", Readyz)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"status":"ok"`) {
		t.Fatalf("body = %s", body)
	}
	if !strings.Contains(body, `"redis":"ok"`) || !strings.Contains(body, `"postgres":"ok"`) {
		t.Fatalf("body = %s", body)
	}
}

func TestReadyzReturnsUnavailableWhenDependencyFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	SetHealthChecks(func(context.Context) error {
		return errors.New("redis down")
	}, func(context.Context) error {
		return nil
	})

	router := gin.New()
	router.GET("/readyz", Readyz)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", recorder.Code)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"status":"unavailable"`) {
		t.Fatalf("body = %s", body)
	}
	if !strings.Contains(body, `"redis":"error"`) || !strings.Contains(body, `"postgres":"ok"`) {
		t.Fatalf("body = %s", body)
	}
}
