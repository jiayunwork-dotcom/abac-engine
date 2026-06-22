package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"abac-engine/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PolicyRepository interface {
	GetTenantPolicies(ctx context.Context, tenantID string, projectID string) ([]models.Policy, error)
	GetGlobalPolicies(ctx context.Context) ([]models.Policy, error)
	GetPolicyByID(ctx context.Context, tenantID, policyID string) (*models.Policy, error)
	CreatePolicy(ctx context.Context, policy *models.Policy) error
	UpdatePolicy(ctx context.Context, policy *models.Policy) error
	DeletePolicy(ctx context.Context, tenantID, policyID string) error
	SetPolicyStatus(ctx context.Context, tenantID, policyID string, status models.PolicyStatus) error
	CountTenantPolicies(ctx context.Context, tenantID string) (int, error)
	GetPolicyVersions(ctx context.Context, tenantID, policyID string) ([]models.PolicyVersion, error)
	GetPolicyVersion(ctx context.Context, tenantID, policyID string, version int) (*models.PolicyVersion, error)
	CreatePolicyVersion(ctx context.Context, v *models.PolicyVersion) error
	RollbackPolicy(ctx context.Context, tenantID, policyID string, toVersion int, note string, createdBy string) (*models.Policy, error)
	GetMaxGlobalVersion(ctx context.Context) (int64, error)
	GetMaxTenantVersion(ctx context.Context, tenantID string) (int64, error)
}

type TenantRepository interface {
	GetTenantByAPIKey(ctx context.Context, apiKey string) (*models.Tenant, error)
	GetTenantByID(ctx context.Context, id string) (*models.Tenant, error)
	ListTenants(ctx context.Context, offset, limit int) ([]models.Tenant, int64, error)
	CreateTenant(ctx context.Context, t *models.Tenant) error
	UpdateTenant(ctx context.Context, t *models.Tenant) error
	DeleteTenant(ctx context.Context, id string) error
}

type AuditRepository interface {
	InsertAuditLog(ctx context.Context, log *models.AuditLog) error
	QueryAuditLogs(ctx context.Context, tenantID string, start, end time.Time, decision string, offset, limit int) ([]models.AuditLog, int64, error)
	CleanupOldLogs(ctx context.Context, days int) (int64, error)
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) scanPolicy(row pgx.Row) (*models.Policy, error) {
	var p models.Policy
	var targetJSON string
	var resourceTypes, actions []string
	err := row.Scan(
		&p.ID, &p.TenantID, &p.ProjectID, &p.Level, &p.Description,
		&targetJSON, &p.Effect, &p.Priority, &p.Status, &p.Version,
		&p.ForceDeny, &p.CreatedAt, &p.UpdatedAt, &resourceTypes, &actions,
	)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(targetJSON), &p.Target); err != nil {
		return nil, fmt.Errorf("unmarshal target: %v", err)
	}
	p.ResourceTypes = resourceTypes
	p.Actions = actions
	return &p, nil
}

func (r *PostgresRepository) GetTenantPolicies(ctx context.Context, tenantID string, projectID string) ([]models.Policy, error) {
	query := `
		SELECT id, tenant_id, project_id, level, description, target::text,
		       effect, priority, status, version, force_deny, created_at, updated_at,
		       COALESCE(resource_types, ARRAY[]::text[]), COALESCE(actions, ARRAY[]::text[])
		FROM policies
		WHERE status = 'enabled'
		  AND tenant_id = $1
		  AND level IN ('tenant', 'project')
	`
	args := []interface{}{tenantID}
	if projectID != "" {
		query += " AND (project_id = $2 OR project_id IS NULL OR project_id = '')"
		args = append(args, projectID)
	}
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var policies []models.Policy
	for rows.Next() {
		p, err := r.scanPolicy(rows)
		if err != nil {
			return nil, err
		}
		policies = append(policies, *p)
	}
	return policies, rows.Err()
}

func (r *PostgresRepository) GetGlobalPolicies(ctx context.Context) ([]models.Policy, error) {
	query := `
		SELECT id, tenant_id, project_id, level, description, target::text,
		       effect, priority, status, version, force_deny, created_at, updated_at,
		       COALESCE(resource_types, ARRAY[]::text[]), COALESCE(actions, ARRAY[]::text[])
		FROM policies
		WHERE status = 'enabled' AND level = 'global'
	`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var policies []models.Policy
	for rows.Next() {
		p, err := r.scanPolicy(rows)
		if err != nil {
			return nil, err
		}
		policies = append(policies, *p)
	}
	return policies, rows.Err()
}

func (r *PostgresRepository) GetPolicyByID(ctx context.Context, tenantID, policyID string) (*models.Policy, error) {
	query := `
		SELECT id, tenant_id, project_id, level, description, target::text,
		       effect, priority, status, version, force_deny, created_at, updated_at,
		       COALESCE(resource_types, ARRAY[]::text[]), COALESCE(actions, ARRAY[]::text[])
		FROM policies
		WHERE id = $1 AND (tenant_id = $2 OR level = 'global')
	`
	row := r.pool.QueryRow(ctx, query, policyID, tenantID)
	return r.scanPolicy(row)
}

func (r *PostgresRepository) CreatePolicy(ctx context.Context, policy *models.Policy) error {
	targetJSON, err := json.Marshal(policy.Target)
	if err != nil {
		return err
	}
	now := time.Now()
	policy.CreatedAt = now
	policy.UpdatedAt = now
	policy.Version = 1

	query := `
		INSERT INTO policies (id, tenant_id, project_id, level, description, target,
			effect, priority, status, version, force_deny, created_at, updated_at,
			resource_types, actions)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
	`
	_, err = r.pool.Exec(ctx, query,
		policy.ID, policy.TenantID, policy.ProjectID, policy.Level, policy.Description,
		targetJSON, policy.Effect, policy.Priority, policy.Status, policy.Version,
		policy.ForceDeny, policy.CreatedAt, policy.UpdatedAt,
		policy.ResourceTypes, policy.Actions,
	)
	if err != nil {
		return err
	}
	return nil
}

func (r *PostgresRepository) UpdatePolicy(ctx context.Context, policy *models.Policy) error {
	targetJSON, err := json.Marshal(policy.Target)
	if err != nil {
		return err
	}
	policy.UpdatedAt = time.Now()
	policy.Version += 1

	query := `
		UPDATE policies SET
			project_id = $1, description = $2, target = $3, effect = $4,
			priority = $5, status = $6, version = $7, force_deny = $8,
			updated_at = $9, resource_types = $10, actions = $11
		WHERE id = $12 AND tenant_id = $13
	`
	_, err = r.pool.Exec(ctx, query,
		policy.ProjectID, policy.Description, targetJSON, policy.Effect,
		policy.Priority, policy.Status, policy.Version, policy.ForceDeny,
		policy.UpdatedAt, policy.ResourceTypes, policy.Actions,
		policy.ID, policy.TenantID,
	)
	return err
}

func (r *PostgresRepository) DeletePolicy(ctx context.Context, tenantID, policyID string) error {
	_, err := r.pool.Exec(ctx,
		"DELETE FROM policies WHERE id = $1 AND tenant_id = $2 AND level != 'global'",
		policyID, tenantID,
	)
	return err
}

func (r *PostgresRepository) SetPolicyStatus(ctx context.Context, tenantID, policyID string, status models.PolicyStatus) error {
	_, err := r.pool.Exec(ctx,
		"UPDATE policies SET status = $1, updated_at = NOW() WHERE id = $2 AND (tenant_id = $3 OR level = 'global')",
		status, policyID, tenantID,
	)
	return err
}

func (r *PostgresRepository) CountTenantPolicies(ctx context.Context, tenantID string) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM policies WHERE tenant_id = $1 AND level != 'global'",
		tenantID,
	).Scan(&count)
	return count, err
}

func (r *PostgresRepository) GetPolicyVersions(ctx context.Context, tenantID, policyID string) ([]models.PolicyVersion, error) {
	query := `
		SELECT id, policy_id, tenant_id, version, content, change_note, created_at, created_by
		FROM policy_versions
		WHERE policy_id = $1 AND tenant_id = $2
		ORDER BY version DESC
		LIMIT 50
	`
	rows, err := r.pool.Query(ctx, query, policyID, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var vs []models.PolicyVersion
	for rows.Next() {
		var v models.PolicyVersion
		err := rows.Scan(&v.ID, &v.PolicyID, &v.TenantID, &v.Version, &v.Content,
			&v.ChangeNote, &v.CreatedAt, &v.CreatedBy)
		if err != nil {
			return nil, err
		}
		vs = append(vs, v)
	}
	return vs, rows.Err()
}

func (r *PostgresRepository) GetPolicyVersion(ctx context.Context, tenantID, policyID string, version int) (*models.PolicyVersion, error) {
	var v models.PolicyVersion
	query := `
		SELECT id, policy_id, tenant_id, version, content, change_note, created_at, created_by
		FROM policy_versions
		WHERE policy_id = $1 AND tenant_id = $2 AND version = $3
	`
	err := r.pool.QueryRow(ctx, query, policyID, tenantID, version).Scan(
		&v.ID, &v.PolicyID, &v.TenantID, &v.Version, &v.Content,
		&v.ChangeNote, &v.CreatedAt, &v.CreatedBy,
	)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (r *PostgresRepository) CreatePolicyVersion(ctx context.Context, v *models.PolicyVersion) error {
	v.CreatedAt = time.Now()
	query := `
		INSERT INTO policy_versions (policy_id, tenant_id, version, content, change_note, created_at, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
	`
	_, err := r.pool.Exec(ctx, query, v.PolicyID, v.TenantID, v.Version, v.Content,
		v.ChangeNote, v.CreatedAt, v.CreatedBy)
	return err
}

func (r *PostgresRepository) RollbackPolicy(ctx context.Context, tenantID, policyID string, toVersion int, note string, createdBy string) (*models.Policy, error) {
	pv, err := r.GetPolicyVersion(ctx, tenantID, policyID, toVersion)
	if err != nil {
		return nil, err
	}
	var oldPolicy models.Policy
	if err := json.Unmarshal([]byte(pv.Content), &oldPolicy); err != nil {
		return nil, fmt.Errorf("unmarshal version content: %v", err)
	}

	if err := r.UpdatePolicy(ctx, &oldPolicy); err != nil {
		return nil, err
	}

	newContent, _ := json.Marshal(oldPolicy)
	_ = r.CreatePolicyVersion(ctx, &models.PolicyVersion{
		PolicyID:   policyID,
		TenantID:   tenantID,
		Version:    oldPolicy.Version,
		Content:    string(newContent),
		ChangeNote: fmt.Sprintf("Rollback to v%d: %s", toVersion, note),
		CreatedBy:  createdBy,
	})

	updated, err := r.GetPolicyByID(ctx, tenantID, policyID)
	return updated, err
}

func (r *PostgresRepository) GetMaxGlobalVersion(ctx context.Context) (int64, error) {
	var v int64
	err := r.pool.QueryRow(ctx,
		"SELECT COALESCE(MAX(EXTRACT(EPOCH FROM updated_at) * 1000000)::bigint, 0) FROM policies WHERE level = 'global'",
	).Scan(&v)
	return v, err
}

func (r *PostgresRepository) GetMaxTenantVersion(ctx context.Context, tenantID string) (int64, error) {
	var v int64
	err := r.pool.QueryRow(ctx,
		"SELECT COALESCE(MAX(EXTRACT(EPOCH FROM updated_at) * 1000000)::bigint, 0) FROM policies WHERE tenant_id = $1",
		tenantID,
	).Scan(&v)
	return v, err
}

func (r *PostgresRepository) GetTenantByAPIKey(ctx context.Context, apiKey string) (*models.Tenant, error) {
	var t models.Tenant
	query := `
		SELECT id, name, api_key, combining_algorithm, max_policies, max_rps, created_at, updated_at
		FROM tenants WHERE api_key = $1
	`
	err := r.pool.QueryRow(ctx, query, apiKey).Scan(
		&t.ID, &t.Name, &t.APIKey, &t.CombiningAlgorithm,
		&t.MaxPolicies, &t.MaxRPS, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *PostgresRepository) GetTenantByID(ctx context.Context, id string) (*models.Tenant, error) {
	var t models.Tenant
	query := `
		SELECT id, name, api_key, combining_algorithm, max_policies, max_rps, created_at, updated_at
		FROM tenants WHERE id = $1
	`
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&t.ID, &t.Name, &t.APIKey, &t.CombiningAlgorithm,
		&t.MaxPolicies, &t.MaxRPS, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *PostgresRepository) ListTenants(ctx context.Context, offset, limit int) ([]models.Tenant, int64, error) {
	var total int64
	err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM tenants").Scan(&total)
	if err != nil {
		return nil, 0, err
	}
	query := `
		SELECT id, name, api_key, combining_algorithm, max_policies, max_rps, created_at, updated_at
		FROM tenants ORDER BY created_at DESC LIMIT $1 OFFSET $2
	`
	rows, err := r.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var tenants []models.Tenant
	for rows.Next() {
		var t models.Tenant
		err := rows.Scan(&t.ID, &t.Name, &t.APIKey, &t.CombiningAlgorithm,
			&t.MaxPolicies, &t.MaxRPS, &t.CreatedAt, &t.UpdatedAt)
		if err != nil {
			return nil, 0, err
		}
		tenants = append(tenants, t)
	}
	return tenants, total, rows.Err()
}

func (r *PostgresRepository) CreateTenant(ctx context.Context, t *models.Tenant) error {
	now := time.Now()
	t.CreatedAt = now
	t.UpdatedAt = now
	query := `
		INSERT INTO tenants (id, name, api_key, combining_algorithm, max_policies, max_rps, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
	`
	_, err := r.pool.Exec(ctx, query, t.ID, t.Name, t.APIKey, t.CombiningAlgorithm,
		t.MaxPolicies, t.MaxRPS, t.CreatedAt, t.UpdatedAt)
	return err
}

func (r *PostgresRepository) UpdateTenant(ctx context.Context, t *models.Tenant) error {
	t.UpdatedAt = time.Now()
	query := `
		UPDATE tenants SET name=$1, combining_algorithm=$2, max_policies=$3, max_rps=$4, updated_at=$5
		WHERE id=$6
	`
	_, err := r.pool.Exec(ctx, query, t.Name, t.CombiningAlgorithm, t.MaxPolicies, t.MaxRPS, t.UpdatedAt, t.ID)
	return err
}

func (r *PostgresRepository) DeleteTenant(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, "DELETE FROM tenants WHERE id=$1", id)
	return err
}

func (r *PostgresRepository) InsertAuditLog(ctx context.Context, log *models.AuditLog) error {
	log.Timestamp = time.Now()
	query := `
		INSERT INTO audit_logs (timestamp, tenant_id, project_id, request_id,
			subject_summary, resource_summary, action, decision, matched_policies, duration_us, env_summary)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
	`
	_, err := r.pool.Exec(ctx, query,
		log.Timestamp, log.TenantID, log.ProjectID, log.RequestID,
		log.SubjectSummary, log.ResourceSummary, log.Action,
		log.Decision, log.MatchedPolicies, log.DurationUs, log.EnvSummary,
	)
	return err
}

func (r *PostgresRepository) QueryAuditLogs(ctx context.Context, tenantID string, start, end time.Time, decision string, offset, limit int) ([]models.AuditLog, int64, error) {
	countArgs := []interface{}{tenantID, start, end}
	listArgs := []interface{}{tenantID, start, end}
	queryCount := `SELECT COUNT(*) FROM audit_logs WHERE tenant_id = $1 AND timestamp >= $2 AND timestamp <= $3`
	queryList := `
		SELECT id, timestamp, tenant_id, project_id, request_id,
			subject_summary, resource_summary, action, decision, matched_policies, duration_us, env_summary
		FROM audit_logs
		WHERE tenant_id = $1 AND timestamp >= $2 AND timestamp <= $3
	`
	paramIdx := 4
	if decision != "" {
		queryCount += fmt.Sprintf(" AND decision = $%d", paramIdx)
		queryList += fmt.Sprintf(" AND decision = $%d", paramIdx)
		countArgs = append(countArgs, decision)
		listArgs = append(listArgs, decision)
		paramIdx++
	}
	queryList += fmt.Sprintf(" ORDER BY timestamp DESC LIMIT $%d OFFSET $%d", paramIdx, paramIdx+1)
	listArgs = append(listArgs, limit, offset)

	var total int64
	err := r.pool.QueryRow(ctx, queryCount, countArgs...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.pool.Query(ctx, queryList, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var logs []models.AuditLog
	for rows.Next() {
		var l models.AuditLog
		err := rows.Scan(&l.ID, &l.Timestamp, &l.TenantID, &l.ProjectID, &l.RequestID,
			&l.SubjectSummary, &l.ResourceSummary, &l.Action, &l.Decision,
			&l.MatchedPolicies, &l.DurationUs, &l.EnvSummary)
		if err != nil {
			return nil, 0, err
		}
		logs = append(logs, l)
	}
	return logs, total, rows.Err()
}

func (r *PostgresRepository) CleanupOldLogs(ctx context.Context, days int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -days)
	tag, err := r.pool.Exec(ctx, "DELETE FROM audit_logs WHERE timestamp < $1", cutoff)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (r *PostgresRepository) ListTenantAllPolicies(ctx context.Context, tenantID string) ([]models.Policy, error) {
	query := `
		SELECT id, tenant_id, project_id, level, description, target::text,
		       effect, priority, status, version, force_deny, created_at, updated_at,
		       COALESCE(resource_types, ARRAY[]::text[]), COALESCE(actions, ARRAY[]::text[])
		FROM policies
		WHERE tenant_id = $1
		ORDER BY level DESC, priority DESC, created_at DESC
	`
	rows, err := r.pool.Query(ctx, query, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var policies []models.Policy
	for rows.Next() {
		p, err := r.scanPolicy(rows)
		if err != nil {
			return nil, err
		}
		policies = append(policies, *p)
	}
	return policies, rows.Err()
}
