package middleware

import (
	"context"
	"net/http"
	"strings"

	"abac-engine/internal/cache"
	"abac-engine/internal/models"
	"abac-engine/internal/repository"

	"github.com/gin-gonic/gin"
)

type contextKey string

const (
	TenantKey       contextKey = "tenant"
	TenantIDKey     contextKey = "tenant_id"
	PlatformAdminKey contextKey = "platform_admin"
)

func TenantAuth(repo *repository.PostgresRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("X-API-Key")
		if apiKey == "" {
			apiKey = c.Query("api_key")
		}
		if apiKey == "" {
			auth := c.GetHeader("Authorization")
			if strings.HasPrefix(auth, "Bearer ") {
				apiKey = strings.TrimPrefix(auth, "Bearer ")
			}
		}
		if apiKey == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing API key"})
			return
		}

		ctx := c.Request.Context()
		tenant, err := repo.GetTenantByAPIKey(ctx, apiKey)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid API key"})
			return
		}

		c.Set(string(TenantKey), tenant)
		c.Set(string(TenantIDKey), tenant.ID)
		c.Request = c.Request.WithContext(context.WithValue(ctx, TenantIDKey, tenant.ID))
		c.Next()
	}
}

func RateLimit(rl *cache.RateLimiter, repo *repository.PostgresRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenant, exists := c.Get(string(TenantKey))
		if !exists {
			c.Next()
			return
		}
		t := tenant.(*models.Tenant)

		allow, err := rl.Allow(c.Request.Context(), t.ID, t.MaxRPS)
		if err != nil {
			c.Next()
			return
		}
		if !allow {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded", "max_rps": t.MaxRPS})
			return
		}
		c.Next()
	}
}

func PlatformAdminAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		adminToken := c.GetHeader("X-Admin-Token")
		expected := c.GetString("ADMIN_TOKEN")
		if expected == "" {
			expected = "admin-secret-token-change-me"
		}
		if adminToken == "" || adminToken != expected {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "platform admin access required"})
			return
		}
		c.Set(string(PlatformAdminKey), true)
		c.Next()
	}
}

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key, X-Admin-Token")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
