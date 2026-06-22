package handlers

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"abac-engine/internal/audit"
	"abac-engine/internal/cache"
	"abac-engine/internal/engine"
	"abac-engine/internal/models"
	"abac-engine/internal/repository"
	"abac-engine/internal/snapshot"
	"abac-engine/pkg/expression"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

type Handler struct {
	Repo        *repository.PostgresRepository
	Engine      *engine.ABACEngine
	Cache       *cache.DecisionCache
	RL          *cache.RateLimiter
	SnapMgr     *snapshot.SnapshotManager
	AuditWriter *audit.AuditWriter
	AdminToken  string
}

func NewHandler(
	repo *repository.PostgresRepository,
	eng *engine.ABACEngine,
	dc *cache.DecisionCache,
	rl *cache.RateLimiter,
	sm *snapshot.SnapshotManager,
	aw *audit.AuditWriter,
	adminToken string,
) *Handler {
	return &Handler{
		Repo:        repo,
		Engine:      eng,
		Cache:       dc,
		RL:          rl,
		SnapMgr:     sm,
		AuditWriter: aw,
		AdminToken:  adminToken,
	}
}

func (h *Handler) getTenant(c *gin.Context) *models.Tenant {
	t, _ := c.Get("tenant")
	if t == nil {
		return nil
	}
	return t.(*models.Tenant)
}

func (h *Handler) getTenantID(c *gin.Context) string {
	id, _ := c.Get("tenant_id")
	if id == nil {
		return ""
	}
	return id.(string)
}

func (h *Handler) Decide(c *gin.Context) {
	tenant := h.getTenant(c)
	if tenant == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req models.AccessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.TenantID = tenant.ID

	cached, err := h.Cache.Get(c.Request.Context(), &req)
	if err == nil && cached != nil {
		cached.RequestID = req.RequestID
		h.writeAudit(c, &req, cached)
		c.JSON(http.StatusOK, cached)
		return
	}

	result, err := h.Engine.Decide(&req, tenant.CombiningAlgorithm)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	_ = h.Cache.Set(c.Request.Context(), &req, result)
	h.writeAudit(c, &req, result)
	c.JSON(http.StatusOK, result)
}

func (h *Handler) DecideWithTrace(c *gin.Context) {
	tenant := h.getTenant(c)
	if tenant == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req models.AccessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.TenantID = tenant.ID

	result, trace, err := h.Engine.DecideWithTrace(&req, tenant.CombiningAlgorithm)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"result": result, "trace": trace})
}

func (h *Handler) writeAudit(c *gin.Context, req *models.AccessRequest, result *models.DecisionResult) {
	log := &models.AuditLog{
		TenantID:        req.TenantID,
		ProjectID:       req.ProjectID,
		RequestID:       result.RequestID,
		SubjectSummary:  audit.SummarizeAttrs(req.Subject),
		ResourceSummary: audit.SummarizeAttrs(req.Resource),
		Action:          req.Action,
		Decision:        result.Effect,
		MatchedPolicies: result.MatchedPolicies,
		DurationUs:      result.DecisionTime,
		EnvSummary:      audit.SummarizeAttrs(req.Environment),
	}
	h.AuditWriter.Write(c.Request.Context(), log)
}

func (h *Handler) ListPolicies(c *gin.Context) {
	tenantID := h.getTenantID(c)
	policies, err := h.Repo.ListTenantAllPolicies(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"policies": policies, "total": len(policies)})
}

func (h *Handler) GetPolicy(c *gin.Context) {
	tenantID := h.getTenantID(c)
	id := c.Param("id")
	p, err := h.Repo.GetPolicyByID(c.Request.Context(), tenantID, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "policy not found"})
		return
	}
	yamlBytes, _ := yaml.Marshal(p)
	c.JSON(http.StatusOK, gin.H{"policy": p, "yaml": string(yamlBytes)})
}

func (h *Handler) parsePolicyFromYAML(yamlStr string, policy *models.Policy) error {
	return yaml.Unmarshal([]byte(yamlStr), policy)
}

func (h *Handler) ValidatePolicy(c *gin.Context) {
	var body struct {
		YAML string `json:"yaml"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var p models.Policy
	if err := parsePolicyYAML(body.YAML, &p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"valid": false, "errors": []string{"YAML parse error: " + err.Error()}})
		return
	}

	var errs []string
	eval := expression.NewGroupEvaluator()
	if p.Target.Subject != nil {
		if e := eval.ValidateGroup(p.Target.Subject, expression.ValidSubjectAttributes); len(e) > 0 {
			for _, err := range e {
				errs = append(errs, "subject: "+err.Error())
			}
		}
	}
	if p.Target.Resource != nil {
		if e := eval.ValidateGroup(p.Target.Resource, expression.ValidResourceAttributes); len(e) > 0 {
			for _, err := range e {
				errs = append(errs, "resource: "+err.Error())
			}
		}
	}
	if p.Target.Action != nil {
		if e := eval.ValidateGroup(p.Target.Action, expression.ValidActionAttributes); len(e) > 0 {
			for _, err := range e {
				errs = append(errs, "action: "+err.Error())
			}
		}
	}
	if p.Target.Environment != nil {
		if e := eval.ValidateGroup(p.Target.Environment, expression.ValidEnvAttributes); len(e) > 0 {
			for _, err := range e {
				errs = append(errs, "environment: "+err.Error())
			}
		}
	}

	if p.Effect != models.EffectPermit && p.Effect != models.EffectDeny {
		errs = append(errs, fmt.Sprintf("invalid effect: %s (must be permit/deny)", p.Effect))
	}

	hasConstraint := !expression.IsTargetEmpty(p.Target) ||
		(len(p.ResourceTypes) > 0 && p.ResourceTypes[0] != "*") ||
		(len(p.Actions) > 0 && p.Actions[0] != "*")
	if !hasConstraint {
		errs = append(errs, "policy has no constraints: at least one target condition or non-wildcard resource_type/action is required")
	}

	tenantID := h.getTenantID(c)
	count, _ := h.Repo.CountTenantPolicies(c.Request.Context(), tenantID)
	tenant := h.getTenant(c)
	if count >= tenant.MaxPolicies {
		errs = append(errs, fmt.Sprintf("policy quota exceeded: max %d", tenant.MaxPolicies))
	}

	c.JSON(http.StatusOK, gin.H{"valid": len(errs) == 0, "errors": errs})
}

func (h *Handler) CreatePolicy(c *gin.Context) {
	tenant := h.getTenant(c)
	tenantID := h.getTenantID(c)

	var body struct {
		YAML       string `json:"yaml"`
		ChangeNote string `json:"change_note"`
		CreatedBy  string `json:"created_by"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var p models.Policy
	if err := parsePolicyYAML(body.YAML, &p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "YAML parse error: " + err.Error()})
		return
	}

	if p.ID == "" {
		p.ID = "pol-" + uuid.New().String()[:8]
	}
	p.TenantID = tenantID
	if p.Level == "" {
		p.Level = models.LevelTenant
	}
	if p.Status == "" {
		p.Status = models.StatusEnabled
	}
	if p.Level == models.LevelGlobal {
		c.JSON(http.StatusForbidden, gin.H{"error": "cannot create global policy via tenant API"})
		return
	}

	count, err := h.Repo.CountTenantPolicies(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if count >= tenant.MaxPolicies {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": fmt.Sprintf("policy quota exceeded: max %d", tenant.MaxPolicies)})
		return
	}

	hasConstraint := !expression.IsTargetEmpty(p.Target) ||
		(len(p.ResourceTypes) > 0 && p.ResourceTypes[0] != "*") ||
		(len(p.Actions) > 0 && p.Actions[0] != "*")
	if !hasConstraint {
		c.JSON(http.StatusBadRequest, gin.H{"error": "policy has no constraints: at least one target condition or non-wildcard resource_type/action is required"})
		return
	}

	if err := h.Repo.CreatePolicy(c.Request.Context(), &p); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	yamlContent, _ := json.Marshal(p)
	_ = h.Repo.CreatePolicyVersion(c.Request.Context(), &models.PolicyVersion{
		PolicyID:   p.ID,
		TenantID:   tenantID,
		Version:    1,
		Content:    string(yamlContent),
		ChangeNote: body.ChangeNote,
		CreatedBy:  body.CreatedBy,
	})

	_ = h.SnapMgr.RefreshTenant(c.Request.Context(), tenantID)
	_ = h.RL.SetPolicyCount(c.Request.Context(), tenantID, count+1)

	c.JSON(http.StatusCreated, p)
}

func (h *Handler) UpdatePolicy(c *gin.Context) {
	tenantID := h.getTenantID(c)
	id := c.Param("id")

	var body struct {
		YAML       string `json:"yaml"`
		ChangeNote string `json:"change_note"`
		CreatedBy  string `json:"created_by"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	existing, err := h.Repo.GetPolicyByID(c.Request.Context(), tenantID, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "policy not found"})
		return
	}
	if existing.Level == models.LevelGlobal {
		c.JSON(http.StatusForbidden, gin.H{"error": "cannot modify global policy via tenant API"})
		return
	}

	var p models.Policy
	if err := parsePolicyYAML(body.YAML, &p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "YAML parse error: " + err.Error()})
		return
	}
	p.ID = id
	p.TenantID = tenantID
	p.CreatedAt = existing.CreatedAt
	p.Version = existing.Version

	hasConstraint := !expression.IsTargetEmpty(p.Target) ||
		(len(p.ResourceTypes) > 0 && p.ResourceTypes[0] != "*") ||
		(len(p.Actions) > 0 && p.Actions[0] != "*")
	if !hasConstraint {
		c.JSON(http.StatusBadRequest, gin.H{"error": "policy has no constraints: at least one target condition or non-wildcard resource_type/action is required"})
		return
	}

	if err := h.Repo.UpdatePolicy(c.Request.Context(), &p); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	yamlContent, _ := json.Marshal(p)
	_ = h.Repo.CreatePolicyVersion(c.Request.Context(), &models.PolicyVersion{
		PolicyID:   p.ID,
		TenantID:   tenantID,
		Version:    p.Version,
		Content:    string(yamlContent),
		ChangeNote: body.ChangeNote,
		CreatedBy:  body.CreatedBy,
	})

	_ = h.SnapMgr.RefreshTenant(c.Request.Context(), tenantID)
	c.JSON(http.StatusOK, p)
}

func (h *Handler) DeletePolicy(c *gin.Context) {
	tenantID := h.getTenantID(c)
	id := c.Param("id")
	if err := h.Repo.DeletePolicy(c.Request.Context(), tenantID, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	_ = h.SnapMgr.RefreshTenant(c.Request.Context(), tenantID)
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func (h *Handler) TogglePolicy(c *gin.Context) {
	tenantID := h.getTenantID(c)
	id := c.Param("id")
	var body struct {
		Status models.PolicyStatus `json:"status"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.Repo.SetPolicyStatus(c.Request.Context(), tenantID, id, body.Status); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	_ = h.SnapMgr.RefreshTenant(c.Request.Context(), tenantID)
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) ListVersions(c *gin.Context) {
	tenantID := h.getTenantID(c)
	policyID := c.Param("id")
	vs, err := h.Repo.GetPolicyVersions(c.Request.Context(), tenantID, policyID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"versions": vs})
}

func (h *Handler) RollbackPolicy(c *gin.Context) {
	tenantID := h.getTenantID(c)
	policyID := c.Param("id")
	var body struct {
		Version   int    `json:"version"`
		Note      string `json:"note"`
		CreatedBy string `json:"created_by"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	p, err := h.Repo.RollbackPolicy(c.Request.Context(), tenantID, policyID, body.Version, body.Note, body.CreatedBy)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	_ = h.SnapMgr.RefreshTenant(c.Request.Context(), tenantID)
	c.JSON(http.StatusOK, p)
}

func (h *Handler) Simulate(c *gin.Context) {
	tenant := h.getTenant(c)
	var req models.AccessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.TenantID = tenant.ID
	result, trace, err := h.Engine.DecideWithTrace(&req, tenant.CombiningAlgorithm)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"result": result, "trace": trace})
}

func (h *Handler) SimulateBatch(c *gin.Context) {
	tenant := h.getTenant(c)
	var body models.SimulateBatchRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result := models.SimulateBatchResult{
		TotalRequests: len(body.Requests),
		Differences:   make([]models.SimulateDiff, 0),
	}

	for i, req := range body.Requests {
		req.TenantID = tenant.ID
		oldDec, err := h.Engine.Decide(&req, tenant.CombiningAlgorithm)
		if err != nil {
			continue
		}

		var newDec *models.DecisionResult
		if body.NewPolicyContent != "" && body.PolicyID != "" {
			var tempPolicy models.Policy
			if err := parsePolicyYAML(body.NewPolicyContent, &tempPolicy); err == nil {
				snap := h.Engine.GetTenantSnapshot(tenant.ID)
				tempPolicies := make([]models.Policy, 0)
				if snap != nil {
					for _, p := range snap.Policies {
						if p.ID != body.PolicyID {
							tempPolicies = append(tempPolicies, p)
						}
					}
				}
				tempPolicy.ID = body.PolicyID
				tempPolicy.TenantID = tenant.ID
				tempPolicies = append(tempPolicies, tempPolicy)

				tempEngine := engine.NewABACEngine()
				tempEngine.UpdateTenantSnapshot(tenant.ID, tempPolicies, 0)
				tempEngine.UpdateGlobalSnapshot([]models.Policy{}, 0)
				if gSnap := h.Engine.GetGlobalSnapshot(); gSnap != nil {
					tempEngine.UpdateGlobalSnapshot(gSnap.Policies, gSnap.Version)
				}
				newDec, _ = tempEngine.Decide(&req, tenant.CombiningAlgorithm)
			}
		}
		if newDec == nil {
			newDec = oldDec
		}

		if oldDec.Effect != newDec.Effect {
			result.ChangedCount++
			if oldDec.Effect == models.EffectPermit && newDec.Effect == models.EffectDeny {
				result.PermitToDeny++
			} else if oldDec.Effect == models.EffectDeny && newDec.Effect == models.EffectPermit {
				result.DenyToPermit++
			}
			if len(result.Differences) < 100 {
				result.Differences = append(result.Differences, models.SimulateDiff{
					Index:       i,
					Request:     req,
					OldDecision: *oldDec,
					NewDecision: *newDec,
				})
			}
		}
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) QueryAuditLogs(c *gin.Context) {
	tenantID := h.getTenantID(c)
	startStr := c.DefaultQuery("start", time.Now().AddDate(0, 0, -7).Format(time.RFC3339))
	endStr := c.DefaultQuery("end", time.Now().Format(time.RFC3339))
	decision := c.Query("decision")
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		start = time.Now().AddDate(0, 0, -7)
	}
	end, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		end = time.Now()
	}

	logs, total, err := h.Repo.QueryAuditLogs(c.Request.Context(), tenantID, start, end, decision, offset, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"logs": logs, "total": total, "offset": offset, "limit": limit})
}

func (h *Handler) ExportAuditCSV(c *gin.Context) {
	tenantID := h.getTenantID(c)
	startStr := c.DefaultQuery("start", time.Now().AddDate(0, 0, -30).Format(time.RFC3339))
	endStr := c.DefaultQuery("end", time.Now().Format(time.RFC3339))
	decision := c.Query("decision")

	start, _ := time.Parse(time.RFC3339, startStr)
	end, _ := time.Parse(time.RFC3339, endStr)

	logs, _, err := h.Repo.QueryAuditLogs(c.Request.Context(), tenantID, start, end, decision, 0, 10000)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=audit-%s-%d.csv", tenantID, time.Now().Unix()))
	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()

	writer.Write([]string{"ID", "Timestamp", "TenantID", "ProjectID", "RequestID", "Action", "Decision", "Duration(us)", "MatchedPolicies"})
	for _, l := range logs {
		writer.Write([]string{
			fmt.Sprintf("%d", l.ID),
			l.Timestamp.Format(time.RFC3339),
			l.TenantID,
			l.ProjectID,
			l.RequestID,
			l.Action,
			string(l.Decision),
			fmt.Sprintf("%d", l.DurationUs),
			strings.Join(l.MatchedPolicies, "|"),
		})
	}
}

func (h *Handler) ListTenants(c *gin.Context) {
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	tenants, total, err := h.Repo.ListTenants(c.Request.Context(), offset, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	for i := range tenants {
		tenants[i].APIKey = ""
	}
	c.JSON(http.StatusOK, gin.H{"tenants": tenants, "total": total})
}

func (h *Handler) CreateTenant(c *gin.Context) {
	var t models.Tenant
	if err := c.ShouldBindJSON(&t); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if t.ID == "" {
		t.ID = "tenant-" + uuid.New().String()[:8]
	}
	if t.APIKey == "" {
		t.APIKey = "sk-" + uuid.New().String()
	}
	if t.CombiningAlgorithm == "" {
		t.CombiningAlgorithm = models.AlgoDenyOverride
	}
	if t.MaxPolicies == 0 {
		t.MaxPolicies = 500
	}
	if t.MaxRPS == 0 {
		t.MaxRPS = 1000
	}
	if err := h.Repo.CreateTenant(c.Request.Context(), &t); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, t)
}

func (h *Handler) UpdateTenant(c *gin.Context) {
	id := c.Param("id")
	existing, err := h.Repo.GetTenantByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "tenant not found"})
		return
	}
	var body models.Tenant
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	existing.Name = body.Name
	if body.CombiningAlgorithm != "" {
		existing.CombiningAlgorithm = body.CombiningAlgorithm
	}
	if body.MaxPolicies > 0 {
		existing.MaxPolicies = body.MaxPolicies
	}
	if body.MaxRPS > 0 {
		existing.MaxRPS = body.MaxRPS
	}
	if err := h.Repo.UpdateTenant(c.Request.Context(), existing); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	existing.APIKey = ""
	c.JSON(http.StatusOK, existing)
}

func (h *Handler) DeleteTenant(c *gin.Context) {
	id := c.Param("id")
	if err := h.Repo.DeleteTenant(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func (h *Handler) GetValidAttributes(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"subject":     expression.ValidSubjectAttributes,
		"resource":    expression.ValidResourceAttributes,
		"action":      expression.ValidActionAttributes,
		"environment": expression.ValidEnvAttributes,
		"operators": []string{
			"equals", "not_equals", "contains", "not_contains", "regex_match",
			"gt", "gte", "lt", "lte", "in", "not_in",
			"ip_in_cidr", "time_range", "weekday_range", "intersects", "exists",
		},
	})
}

func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "time": time.Now().Format(time.RFC3339)})
}

func normalizeYAMLValue(v interface{}) interface{} {
	switch val := v.(type) {
	case map[interface{}]interface{}:
		m := make(map[string]interface{})
		for k, vv := range val {
			m[fmt.Sprintf("%v", k)] = normalizeYAMLValue(vv)
		}
		return m
	case map[string]interface{}:
		for k, vv := range val {
			val[k] = normalizeYAMLValue(vv)
		}
		return val
	case []interface{}:
		for i, vv := range val {
			val[i] = normalizeYAMLValue(vv)
		}
		return val
	default:
		return v
	}
}

func normalizeConditionGroup(g *models.ConditionGroup) *models.ConditionGroup {
	if g == nil {
		return nil
	}
	for i := range g.Conditions {
		g.Conditions[i].Value = normalizeYAMLValue(g.Conditions[i].Value)
	}
	for i := range g.Groups {
		g.Groups[i] = *normalizeConditionGroup(&g.Groups[i])
	}
	return g
}

func normalizePolicyTarget(p *models.Policy) {
	p.Target.Subject = normalizeConditionGroup(p.Target.Subject)
	p.Target.Resource = normalizeConditionGroup(p.Target.Resource)
	p.Target.Action = normalizeConditionGroup(p.Target.Action)
	p.Target.Environment = normalizeConditionGroup(p.Target.Environment)
}

func parsePolicyYAML(yamlStr string, p *models.Policy) error {
	if err := yaml.Unmarshal([]byte(yamlStr), p); err != nil {
		return err
	}
	normalizePolicyTarget(p)
	return nil
}
