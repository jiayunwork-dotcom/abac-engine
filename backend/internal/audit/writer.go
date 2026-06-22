package audit

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"abac-engine/internal/models"
	"abac-engine/internal/repository"
)

type AuditWriter struct {
	repo     *repository.PostgresRepository
	buffer   []models.AuditLog
	mu       sync.Mutex
	batchSz  int
	stopCh   chan struct{}
	interval time.Duration
}

func NewAuditWriter(repo *repository.PostgresRepository) *AuditWriter {
	aw := &AuditWriter{
		repo:     repo,
		buffer:   make([]models.AuditLog, 0, 1000),
		batchSz:  100,
		stopCh:   make(chan struct{}),
		interval: 500 * time.Millisecond,
	}
	go aw.flushLoop()
	go aw.cleanupLoop()
	return aw
}

func (aw *AuditWriter) Write(ctx context.Context, l *models.AuditLog) {
	aw.mu.Lock()
	aw.buffer = append(aw.buffer, *l)
	needFlush := len(aw.buffer) >= aw.batchSz
	aw.mu.Unlock()
	if needFlush {
		aw.Flush()
	}
}

func (aw *AuditWriter) Flush() {
	aw.mu.Lock()
	if len(aw.buffer) == 0 {
		aw.mu.Unlock()
		return
	}
	batch := aw.buffer
	aw.buffer = make([]models.AuditLog, 0, aw.batchSz)
	aw.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, l := range batch {
		if err := aw.repo.InsertAuditLog(ctx, &l); err != nil {
			log.Printf("[Audit] Failed to insert log: %v", err)
		}
	}
}

func (aw *AuditWriter) flushLoop() {
	ticker := time.NewTicker(aw.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			aw.Flush()
		case <-aw.stopCh:
			return
		}
	}
}

func (aw *AuditWriter) cleanupLoop() {
	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			n, err := aw.repo.CleanupOldLogs(ctx, 90)
			cancel()
			if err != nil {
				log.Printf("[Audit] Cleanup error: %v", err)
			} else if n > 0 {
				log.Printf("[Audit] Cleaned up %d old audit logs", n)
			}
		case <-aw.stopCh:
			return
		}
	}
}

func (aw *AuditWriter) Stop() {
	close(aw.stopCh)
	aw.Flush()
}

func SummarizeAttrs(m map[string]interface{}) string {
	b, err := json.Marshal(m)
	if err != nil {
		return ""
	}
	s := string(b)
	if len(s) > 500 {
		return s[:500] + "..."
	}
	return s
}
