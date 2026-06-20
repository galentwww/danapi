package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type healthCheckFunc func(context.Context) error

var redisHealthCheck healthCheckFunc
var postgresHealthCheck healthCheckFunc

func SetHealthChecks(redisCheck healthCheckFunc, postgresCheck healthCheckFunc) {
	redisHealthCheck = redisCheck
	postgresHealthCheck = postgresCheck
}

func Healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func Readyz(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	dependencies := gin.H{}
	status := http.StatusOK
	overall := "ok"

	if err := runHealthCheck(ctx, redisHealthCheck); err != nil {
		dependencies["redis"] = "error"
		status = http.StatusServiceUnavailable
		overall = "unavailable"
	} else {
		dependencies["redis"] = "ok"
	}

	if err := runHealthCheck(ctx, postgresHealthCheck); err != nil {
		dependencies["postgres"] = "error"
		status = http.StatusServiceUnavailable
		overall = "unavailable"
	} else {
		dependencies["postgres"] = "ok"
	}

	c.JSON(status, gin.H{
		"status":       overall,
		"dependencies": dependencies,
	})
}

func runHealthCheck(ctx context.Context, check healthCheckFunc) error {
	if check == nil {
		return nil
	}
	return check(ctx)
}
