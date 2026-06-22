package models

import (
	"time"
)

type PolicyLevel string

const (
	LevelGlobal   PolicyLevel = "global"
	LevelTenant   PolicyLevel = "tenant"
	LevelProject  PolicyLevel = "project"
)

type Effect string

const (
	EffectPermit Effect = "permit"
	EffectDeny   Effect = "deny"
)

type CombiningAlgorithm string

const (
	AlgoDenyOverride     CombiningAlgorithm = "deny-override"
	AlgoPermitOverride   CombiningAlgorithm = "permit-override"
	AlgoPriorityFirst    CombiningAlgorithm = "priority-first"
)

type PolicyStatus string

const (
	StatusEnabled  PolicyStatus = "enabled"
	StatusDisabled PolicyStatus = "disabled"
)

type AttributeMap map[string]interface{}

type Condition struct {
	Attribute string      `json:"attribute" yaml:"attribute"`
	Operator  string      `json:"operator" yaml:"operator"`
	Value     interface{} `json:"value" yaml:"value"`
}

type ConditionGroup struct {
	Logic      string           `json:"logic,omitempty" yaml:"logic,omitempty"`
	Conditions []Condition      `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	Groups     []ConditionGroup `json:"groups,omitempty" yaml:"groups,omitempty"`
}

type Target struct {
	Subject     *ConditionGroup `json:"subject,omitempty" yaml:"subject,omitempty"`
	Resource    *ConditionGroup `json:"resource,omitempty" yaml:"resource,omitempty"`
	Action      *ConditionGroup `json:"action,omitempty" yaml:"action,omitempty"`
	Environment *ConditionGroup `json:"environment,omitempty" yaml:"environment,omitempty"`
}

type Policy struct {
	ID          string              `json:"id" yaml:"id"`
	TenantID    string              `json:"tenant_id,omitempty" yaml:"-"`
	ProjectID   string              `json:"project_id,omitempty" yaml:"project_id,omitempty"`
	Level       PolicyLevel         `json:"level" yaml:"level"`
	Description string              `json:"description" yaml:"description"`
	Target      Target              `json:"target" yaml:"target"`
	Effect      Effect              `json:"effect" yaml:"effect"`
	Priority    int                 `json:"priority" yaml:"priority"`
	Status      PolicyStatus        `json:"status" yaml:"status"`
	Version     int                 `json:"version" yaml:"version"`
	ForceDeny   bool                `json:"force_deny,omitempty" yaml:"force_deny,omitempty"`
	CreatedAt   time.Time           `json:"created_at" yaml:"-"`
	UpdatedAt   time.Time           `json:"updated_at" yaml:"-"`
	ResourceTypes []string          `json:"resource_types,omitempty" yaml:"resource_types,omitempty"`
	Actions     []string            `json:"actions,omitempty" yaml:"actions,omitempty"`
}

type PolicyVersion struct {
	ID         int64     `json:"id"`
	PolicyID   string    `json:"policy_id"`
	TenantID   string    `json:"tenant_id"`
	Version    int       `json:"version"`
	Content    string    `json:"content"`
	ChangeNote string    `json:"change_note"`
	CreatedAt  time.Time `json:"created_at"`
	CreatedBy  string    `json:"created_by"`
}

type PolicyDiff struct {
	VersionA    int    `json:"version_a"`
	VersionB    int    `json:"version_b"`
	Added       []string `json:"added"`
	Removed     []string `json:"removed"`
	Modified    []string `json:"modified"`
}

type AccessRequest struct {
	TenantID    string       `json:"tenant_id"`
	ProjectID   string       `json:"project_id,omitempty"`
	Subject     AttributeMap `json:"subject"`
	Resource    AttributeMap `json:"resource"`
	Action      string       `json:"action"`
	Environment AttributeMap `json:"environment"`
	RequestID   string       `json:"request_id,omitempty"`
}

type DecisionResult struct {
	Effect         Effect    `json:"effect"`
	MatchedPolicies []string `json:"matched_policies"`
	DecisionTime   int64     `json:"decision_time_us"`
	RequestID      string    `json:"request_id,omitempty"`
	Reason         string    `json:"reason,omitempty"`
}

type Tenant struct {
	ID                 string             `json:"id"`
	Name               string             `json:"name"`
	APIKey             string             `json:"api_key,omitempty"`
	CombiningAlgorithm CombiningAlgorithm `json:"combining_algorithm"`
	MaxPolicies        int                `json:"max_policies"`
	MaxRPS             int                `json:"max_rps"`
	CreatedAt          time.Time          `json:"created_at"`
	UpdatedAt          time.Time          `json:"updated_at"`
}

type AuditLog struct {
	ID              int64       `json:"id"`
	Timestamp       time.Time   `json:"timestamp"`
	TenantID        string      `json:"tenant_id"`
	ProjectID       string      `json:"project_id,omitempty"`
	RequestID       string      `json:"request_id"`
	SubjectSummary  string      `json:"subject_summary"`
	ResourceSummary string      `json:"resource_summary"`
	Action          string      `json:"action"`
	Decision        Effect      `json:"decision"`
	MatchedPolicies []string    `json:"matched_policies"`
	DurationUs      int64       `json:"duration_us"`
	EnvSummary      string      `json:"env_summary,omitempty"`
}

type SimulateBatchRequest struct {
	Requests []AccessRequest `json:"requests"`
	NewPolicyContent string  `json:"new_policy_content,omitempty"`
	PolicyID        string   `json:"policy_id,omitempty"`
}

type SimulateBatchResult struct {
	TotalRequests   int             `json:"total_requests"`
	ChangedCount    int             `json:"changed_count"`
	PermitToDeny    int             `json:"permit_to_deny"`
	DenyToPermit    int             `json:"deny_to_permit"`
	Differences     []SimulateDiff  `json:"differences"`
}

type SimulateDiff struct {
	Index        int            `json:"index"`
	Request      AccessRequest  `json:"request"`
	OldDecision  DecisionResult `json:"old_decision"`
	NewDecision  DecisionResult `json:"new_decision"`
}

type PolicyConflict struct {
	PolicyID        string   `json:"policy_id"`
	OverlapDims     []string `json:"overlap_dims"`
	OverlapDesc     string   `json:"overlap_desc"`
	WinnerPolicyID  string   `json:"winner_policy_id"`
	WinnerReason    string   `json:"winner_reason"`
}

type DependencyGraph struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

type GraphNode struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Effect      string `json:"effect"`
	Priority    int    `json:"priority"`
	Level       string `json:"level"`
}

type GraphEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type"`
	Desc   string `json:"desc"`
}

const (
	EdgeTypeConflict  = "conflict"
	EdgeTypeOverride  = "override"
	EdgeTypeComplement = "complement"
)
