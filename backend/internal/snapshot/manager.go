package snapshot

import (
	"context"
	"log"
	"sync"
	"time"

	"abac-engine/internal/engine"
	"abac-engine/internal/repository"
)

type SnapshotManager struct {
	engine   *engine.ABACEngine
	repo     *repository.PostgresRepository
	interval time.Duration
	stopCh   chan struct{}
	once     sync.Once
}

func NewSnapshotManager(e *engine.ABACEngine, repo *repository.PostgresRepository) *SnapshotManager {
	return &SnapshotManager{
		engine:   e,
		repo:     repo,
		interval: 2 * time.Second,
		stopCh:   make(chan struct{}),
	}
}

func (sm *SnapshotManager) Start() {
	sm.once.Do(func() {
		ctx := context.Background()
		sm.refreshGlobal(ctx)

		ticker := time.NewTicker(sm.interval)
		go func() {
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
					sm.refreshGlobal(ctx)
					cancel()
				case <-sm.stopCh:
					return
				}
			}
		}()
		log.Println("[SnapshotManager] Global policy reloader started")
	})
}

func (sm *SnapshotManager) Stop() {
	close(sm.stopCh)
}

func (sm *SnapshotManager) refreshGlobal(ctx context.Context) {
	currentVersion, _ := sm.repo.GetMaxGlobalVersion(ctx)
	snap := sm.engine.GetGlobalSnapshot()
	if snap != nil && snap.Version >= currentVersion {
		return
	}
	policies, err := sm.repo.GetGlobalPolicies(ctx)
	if err != nil {
		log.Printf("[SnapshotManager] Failed to load global policies: %v", err)
		return
	}
	sm.engine.UpdateGlobalSnapshot(policies, currentVersion)
	log.Printf("[SnapshotManager] Global snapshot refreshed: %d policies, version=%d", len(policies), currentVersion)
}

func (sm *SnapshotManager) RefreshTenant(ctx context.Context, tenantID string) error {
	currentVersion, err := sm.repo.GetMaxTenantVersion(ctx, tenantID)
	if err != nil {
		return err
	}
	policies, err := sm.repo.GetTenantPolicies(ctx, tenantID, "")
	if err != nil {
		return err
	}
	sm.engine.UpdateTenantSnapshot(tenantID, policies, currentVersion)
	log.Printf("[SnapshotManager] Tenant %s snapshot refreshed: %d policies, version=%d",
		tenantID, len(policies), currentVersion)
	return nil
}
