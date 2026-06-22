package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"time"

	"abac-engine/internal/audit"
	"abac-engine/internal/cache"
	"abac-engine/internal/engine"
	"abac-engine/internal/handlers"
	"abac-engine/internal/middleware"
	"abac-engine/internal/models"
	"abac-engine/internal/repository"
	"abac-engine/internal/snapshot"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	pgURL := getEnv("DATABASE_URL", "postgres://abac:abac123@localhost:5432/abac?sslmode=disable")
	redisAddr := getEnv("REDIS_ADDR", "localhost:6379")
	redisPassword := getEnv("REDIS_PASSWORD", "")
	serverPort := getEnv("SERVER_PORT", "8080")
	adminToken := getEnv("ADMIN_TOKEN", "admin-secret-token-change-me")
	ginMode := getEnv("GIN_MODE", "debug")

	gin.SetMode(ginMode)

	pool, err := pgxpool.New(context.Background(), pgURL)
	if err != nil {
		log.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("PostgreSQL ping failed: %v", err)
	}
	log.Println("Connected to PostgreSQL")

	rdb := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
		DB:       0,
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer rdb.Close()
	log.Println("Connected to Redis")

	repo := repository.NewPostgresRepository(pool)
	abacEngine := engine.NewABACEngine()
	dc := cache.NewDecisionCache(rdb)
	rl := cache.NewRateLimiter(rdb)
	sm := snapshot.NewSnapshotManager(abacEngine, repo)
	aw := audit.NewAuditWriter(repo)
	defer aw.Stop()

	h := handlers.NewHandler(repo, abacEngine, dc, rl, sm, aw, adminToken)

	if err := autoMigrate(pool); err != nil {
		log.Printf("Migration warning: %v", err)
	}

	initData(repo, adminToken)

	sm.Start()

	r := gin.Default()
	r.Use(middleware.CORSMiddleware())

	api := r.Group("/api/v1")
	{
		api.GET("/health", h.Health)
		api.GET("/attributes", h.GetValidAttributes)

		tenantAPI := api.Group("")
		tenantAPI.Use(middleware.TenantAuth(repo), middleware.RateLimit(rl, repo))
		{
			tenantAPI.POST("/decide", h.Decide)
			tenantAPI.POST("/decide/trace", h.DecideWithTrace)
			tenantAPI.POST("/simulate", h.Simulate)
			tenantAPI.POST("/simulate/batch", h.SimulateBatch)

			tenantAPI.GET("/policies", h.ListPolicies)
			tenantAPI.GET("/policies/dependency/graph", h.GetDependencyGraph)
			tenantAPI.POST("/policies/validate", h.ValidatePolicy)
			tenantAPI.GET("/policies/:id", h.GetPolicy)
			tenantAPI.POST("/policies", h.CreatePolicy)
			tenantAPI.PUT("/policies/:id", h.UpdatePolicy)
			tenantAPI.DELETE("/policies/:id", h.DeletePolicy)
			tenantAPI.POST("/policies/:id/status", h.TogglePolicy)
			tenantAPI.GET("/policies/:id/versions", h.ListVersions)
			tenantAPI.POST("/policies/:id/rollback", h.RollbackPolicy)

			tenantAPI.GET("/audit", h.QueryAuditLogs)
			tenantAPI.GET("/audit/export", h.ExportAuditCSV)
		}

		adminAPI := api.Group("/admin")
		adminAPI.Use(middleware.PlatformAdminAuth())
		{
			adminAPI.GET("/tenants", h.ListTenants)
			adminAPI.POST("/tenants", h.CreateTenant)
			adminAPI.PUT("/tenants/:id", h.UpdateTenant)
			adminAPI.DELETE("/tenants/:id", h.DeleteTenant)
		}
	}

	log.Printf("ABAC Engine server starting on :%s", serverPort)
	if err := r.Run(":" + serverPort); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}

func autoMigrate(pool *pgxpool.Pool) error {
	schema := `
	CREATE TABLE IF NOT EXISTS tenants (
		id VARCHAR(64) PRIMARY KEY,
		name VARCHAR(255) NOT NULL,
		api_key VARCHAR(255) UNIQUE NOT NULL,
		combining_algorithm VARCHAR(32) NOT NULL DEFAULT 'deny-override',
		max_policies INTEGER NOT NULL DEFAULT 500,
		max_rps INTEGER NOT NULL DEFAULT 1000,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS policies (
		id VARCHAR(128) NOT NULL,
		tenant_id VARCHAR(64) NOT NULL DEFAULT '',
		project_id VARCHAR(64) NOT NULL DEFAULT '',
		level VARCHAR(16) NOT NULL DEFAULT 'tenant',
		description TEXT,
		target JSONB NOT NULL DEFAULT '{}'::jsonb,
		effect VARCHAR(16) NOT NULL,
		priority INTEGER NOT NULL DEFAULT 0,
		status VARCHAR(16) NOT NULL DEFAULT 'enabled',
		version INTEGER NOT NULL DEFAULT 1,
		force_deny BOOLEAN NOT NULL DEFAULT FALSE,
		resource_types TEXT[],
		actions TEXT[],
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		PRIMARY KEY (id, tenant_id)
	);
	CREATE INDEX IF NOT EXISTS idx_policies_tenant ON policies(tenant_id);
	CREATE INDEX IF NOT EXISTS idx_policies_level ON policies(level);
	CREATE INDEX IF NOT EXISTS idx_policies_status ON policies(status);

	CREATE TABLE IF NOT EXISTS policy_versions (
		id BIGSERIAL PRIMARY KEY,
		policy_id VARCHAR(128) NOT NULL,
		tenant_id VARCHAR(64) NOT NULL,
		version INTEGER NOT NULL,
		content TEXT NOT NULL,
		change_note TEXT,
		created_by VARCHAR(128),
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	CREATE INDEX IF NOT EXISTS idx_policy_versions_ptv ON policy_versions(policy_id, tenant_id, version DESC);

	CREATE TABLE IF NOT EXISTS audit_logs (
		id BIGSERIAL PRIMARY KEY,
		timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		tenant_id VARCHAR(64) NOT NULL,
		project_id VARCHAR(64),
		request_id VARCHAR(128),
		subject_summary TEXT,
		resource_summary TEXT,
		action VARCHAR(64),
		decision VARCHAR(16),
		matched_policies TEXT[],
		duration_us BIGINT,
		env_summary TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_audit_tenant_time ON audit_logs(tenant_id, timestamp DESC);
	CREATE INDEX IF NOT EXISTS idx_audit_decision ON audit_logs(decision);
	`
	_, err := pool.Exec(context.Background(), schema)
	return err
}

func initData(repo *repository.PostgresRepository, adminToken string) {
	ctx := context.Background()

	_, err := repo.GetTenantByID(ctx, "tenant-demo")
	if err != nil {
		log.Println("[Init] Creating demo tenant...")
		repo.CreateTenant(ctx, &models.Tenant{
			ID:                 "tenant-demo",
			Name:               "Demo Tenant",
			APIKey:             "sk-demo-tenant-key-12345",
			CombiningAlgorithm: models.AlgoDenyOverride,
			MaxPolicies:        500,
			MaxRPS:             1000,
		})
	}

	_, err = repo.GetTenantByID(ctx, "tenant-admin")
	if err != nil {
		log.Println("[Init] Creating admin tenant...")
		repo.CreateTenant(ctx, &models.Tenant{
			ID:                 "tenant-admin",
			Name:               "Platform Admin Tenant",
			APIKey:             "sk-admin-tenant-key-99999",
			CombiningAlgorithm: models.AlgoDenyOverride,
			MaxPolicies:        1000,
			MaxRPS:             5000,
		})
	}

	log.Println("[Init] Admin token for platform operations: " + adminToken)
	log.Println("[Init] Demo tenant API key: sk-demo-tenant-key-12345")
}
