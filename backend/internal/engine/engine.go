package engine

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"abac-engine/internal/models"
	"abac-engine/pkg/expression"
)

type PolicySnapshot struct {
	Version  int64
	Policies []models.Policy
	Index    PolicyIndex
}

type PolicyIndex struct {
	ByResourceType map[string][]int
	ByAction       map[string][]int
	All            []int
}

type ABACEngine struct {
	mu              sync.RWMutex
	globalSnapshot  *PolicySnapshot
	tenantSnapshots map[string]*PolicySnapshot

	groupEval *expression.GroupEvaluator
}

func NewABACEngine() *ABACEngine {
	return &ABACEngine{
		tenantSnapshots: make(map[string]*PolicySnapshot),
		groupEval:       expression.NewGroupEvaluator(),
	}
}

func BuildIndex(policies []models.Policy) PolicyIndex {
	idx := PolicyIndex{
		ByResourceType: make(map[string][]int),
		ByAction:       make(map[string][]int),
	}
	for i, p := range policies {
		idx.All = append(idx.All, i)
		if len(p.ResourceTypes) > 0 {
			for _, rt := range p.ResourceTypes {
				rt = strings.ToLower(rt)
				idx.ByResourceType[rt] = append(idx.ByResourceType[rt], i)
			}
		} else {
			idx.ByResourceType["*"] = append(idx.ByResourceType["*"], i)
		}
		if len(p.Actions) > 0 {
			for _, a := range p.Actions {
				a = strings.ToLower(a)
				idx.ByAction[a] = append(idx.ByAction[a], i)
			}
		} else {
			idx.ByAction["*"] = append(idx.ByAction["*"], i)
		}
	}
	return idx
}

func (e *ABACEngine) UpdateGlobalSnapshot(policies []models.Policy, version int64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.globalSnapshot = &PolicySnapshot{
		Version:  version,
		Policies: policies,
		Index:    BuildIndex(policies),
	}
}

func (e *ABACEngine) UpdateTenantSnapshot(tenantID string, policies []models.Policy, version int64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.tenantSnapshots[tenantID] = &PolicySnapshot{
		Version:  version,
		Policies: policies,
		Index:    BuildIndex(policies),
	}
}

func (e *ABACEngine) GetGlobalSnapshot() *PolicySnapshot {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.globalSnapshot
}

func (e *ABACEngine) GetTenantSnapshot(tenantID string) *PolicySnapshot {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.tenantSnapshots[tenantID]
}

type matchedPolicy struct {
	idx    int
	policy models.Policy
}

func (e *ABACEngine) filterCandidates(snap *PolicySnapshot, resType, action string) []int {
	if snap == nil {
		return nil
	}
	resType = strings.ToLower(resType)
	action = strings.ToLower(action)

	rtSet := make(map[int]bool)
	rtList, ok1 := snap.Index.ByResourceType[resType]
	allList, ok2 := snap.Index.ByResourceType["*"]
	for _, i := range rtList {
		rtSet[i] = true
	}
	for _, i := range allList {
		rtSet[i] = true
	}
	if !ok1 && !ok2 {
		return snap.Index.All
	}

	actSet := make(map[int]bool)
	actList, ok3 := snap.Index.ByAction[action]
	allActList, ok4 := snap.Index.ByAction["*"]
	for _, i := range actList {
		actSet[i] = true
	}
	for _, i := range allActList {
		actSet[i] = true
	}
	if !ok3 && !ok4 {
		result := make([]int, 0, len(rtSet))
		for i := range rtSet {
			result = append(result, i)
		}
		return result
	}

	var result []int
	if len(rtSet) < len(actSet) {
		for i := range rtSet {
			if actSet[i] {
				result = append(result, i)
			}
		}
	} else {
		for i := range actSet {
			if rtSet[i] {
				result = append(result, i)
			}
		}
	}
	return result
}

func (e *ABACEngine) evaluatePolicies(snap *PolicySnapshot, candidates []int, req *models.AccessRequest) []matchedPolicy {
	if snap == nil {
		return nil
	}
	var matched []matchedPolicy
	for _, idx := range candidates {
		p := snap.Policies[idx]
		if p.Status != models.StatusEnabled {
			continue
		}
		if p.ProjectID != "" && req.ProjectID != "" && p.ProjectID != req.ProjectID {
			continue
		}
		if e.matchPolicy(&p, req) {
			matched = append(matched, matchedPolicy{idx: idx, policy: p})
		}
	}
	return matched
}

func (e *ABACEngine) matchPolicy(p *models.Policy, req *models.AccessRequest) bool {
	targetEmpty := expression.IsTargetEmpty(p.Target)
	hasResTypeConstraint := hasNonWildcardSlice(p.ResourceTypes)
	hasActionConstraint := hasNonWildcardSlice(p.Actions)

	if targetEmpty && !hasResTypeConstraint && !hasActionConstraint {
		return false
	}

	if hasResTypeConstraint {
		reqResType, _ := req.Resource["type"].(string)
		reqResType = strings.ToLower(reqResType)
		matched := false
		for _, rt := range p.ResourceTypes {
			if strings.ToLower(rt) == reqResType {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	if p.Target.Action != nil {
		actionMap := map[string]interface{}{"name": req.Action}
		ok, err := e.groupEval.EvaluateGroup(p.Target.Action, actionMap)
		if err != nil || !ok {
			return false
		}
	} else if hasActionConstraint {
		reqAction := strings.ToLower(req.Action)
		matched := false
		for _, a := range p.Actions {
			if strings.ToLower(a) == reqAction {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	if p.Target.Subject != nil {
		ok, err := e.groupEval.EvaluateGroup(p.Target.Subject, req.Subject)
		if err != nil || !ok {
			return false
		}
	}

	if p.Target.Resource != nil {
		ok, err := e.groupEval.EvaluateGroup(p.Target.Resource, req.Resource)
		if err != nil || !ok {
			return false
		}
	}

	if p.Target.Environment != nil {
		envMap := make(map[string]interface{})
		for k, v := range req.Environment {
			envMap[k] = v
		}
		if _, ok := envMap["timestamp"]; !ok {
			envMap["timestamp"] = time.Now().Format(time.RFC3339)
		}
		ok, err := e.groupEval.EvaluateGroup(p.Target.Environment, envMap)
		if err != nil || !ok {
			return false
		}
	}

	return true
}

func hasNonWildcardSlice(s []string) bool {
	for _, v := range s {
		if v != "*" {
			return true
		}
	}
	return false
}

func (e *ABACEngine) combineDecisions(matched []matchedPolicy, algo models.CombiningAlgorithm) (models.Effect, []string) {
	if len(matched) == 0 {
		return "not-applicable", nil
	}

	matchedIDs := make([]string, 0, len(matched))
	for _, m := range matched {
		matchedIDs = append(matchedIDs, m.policy.ID)
	}

	switch algo {
	case models.AlgoDenyOverride:
		for _, m := range matched {
			if m.policy.Effect == models.EffectDeny {
				return models.EffectDeny, matchedIDs
			}
		}
		return models.EffectPermit, matchedIDs

	case models.AlgoPermitOverride:
		for _, m := range matched {
			if m.policy.Effect == models.EffectPermit {
				return models.EffectPermit, matchedIDs
			}
		}
		return models.EffectDeny, matchedIDs

	case models.AlgoPriorityFirst:
		sort.Slice(matched, func(i, j int) bool {
			return matched[i].policy.Priority > matched[j].policy.Priority
		})
		return matched[0].policy.Effect, matchedIDs

	default:
		return models.EffectDeny, matchedIDs
	}
}

func (e *ABACEngine) Decide(req *models.AccessRequest, algo models.CombiningAlgorithm) (*models.DecisionResult, error) {
	start := time.Now()
	reqID := req.RequestID
	if reqID == "" {
		reqID = fmt.Sprintf("req-%d", time.Now().UnixNano())
	}

	globalSnap := e.GetGlobalSnapshot()
	tenantSnap := e.GetTenantSnapshot(req.TenantID)

	resType, _ := req.Resource["type"].(string)

	var allMatched []matchedPolicy

	projectLevel := make(map[string]bool)
	tenantLevel := make(map[string]bool)
	globalLevel := make(map[string]bool)

	if tenantSnap != nil {
		candidates := e.filterCandidates(tenantSnap, resType, req.Action)
		matched := e.evaluatePolicies(tenantSnap, candidates, req)
		for _, m := range matched {
			if m.policy.Level == models.LevelProject {
				projectLevel[m.policy.ID] = true
			} else {
				tenantLevel[m.policy.ID] = true
			}
			allMatched = append(allMatched, m)
		}
	}

	if globalSnap != nil {
		candidates := e.filterCandidates(globalSnap, resType, req.Action)
		matched := e.evaluatePolicies(globalSnap, candidates, req)
		for _, m := range matched {
			globalLevel[m.policy.ID] = true
			allMatched = append(allMatched, m)
		}
	}

	projectMatched := filterByLevel(allMatched, models.LevelProject)
	tenantMatched := filterByLevel(allMatched, models.LevelTenant)
	globalMatched := filterByLevel(allMatched, models.LevelGlobal)

	finalEffect := models.Effect("not-applicable")
	allMatchedIDs := make([]string, 0)

	projectEffect, projectIDs := e.combineDecisions(projectMatched, algo)
	if projectEffect != "not-applicable" {
		finalEffect = projectEffect
		allMatchedIDs = append(allMatchedIDs, projectIDs...)
	}

	tenantEffect, tenantIDs := e.combineDecisions(tenantMatched, algo)
	if tenantEffect != "not-applicable" {
		allMatchedIDs = append(allMatchedIDs, tenantIDs...)
		if finalEffect == "not-applicable" {
			finalEffect = tenantEffect
		} else if finalEffect != tenantEffect {
			if algo == models.AlgoDenyOverride {
				if tenantEffect == models.EffectDeny {
					finalEffect = models.EffectDeny
				}
			} else if algo == models.AlgoPermitOverride {
				if tenantEffect == models.EffectPermit {
					finalEffect = models.EffectPermit
				}
			} else {
				finalEffect = tenantEffect
			}
		}
	}

	for _, m := range globalMatched {
		if m.policy.ForceDeny && m.policy.Effect == models.EffectDeny {
			finalEffect = models.EffectDeny
			break
		}
	}

	globalEffect, globalIDs := e.combineDecisions(globalMatched, models.AlgoDenyOverride)
	allMatchedIDs = append(allMatchedIDs, globalIDs...)
	if globalEffect != "not-applicable" {
		if algo == models.AlgoDenyOverride {
			if globalEffect == models.EffectDeny {
				finalEffect = models.EffectDeny
			}
		} else if finalEffect == "not-applicable" {
			finalEffect = globalEffect
		}
	}

	var hasForceDeny bool
	for _, m := range globalMatched {
		if m.policy.ForceDeny && m.policy.Effect == models.EffectDeny {
			hasForceDeny = true
			break
		}
	}
	if hasForceDeny {
		finalEffect = models.EffectDeny
	}

	duration := time.Since(start).Microseconds()

	return &models.DecisionResult{
		Effect:          finalEffect,
		MatchedPolicies: allMatchedIDs,
		DecisionTime:    duration,
		RequestID:       reqID,
		Reason:          buildReason(finalEffect, projectEffect, tenantEffect, globalEffect),
	}, nil
}

func filterByLevel(matched []matchedPolicy, level models.PolicyLevel) []matchedPolicy {
	var result []matchedPolicy
	for _, m := range matched {
		if m.policy.Level == level {
			result = append(result, m)
		}
	}
	return result
}

func buildReason(final, proj, tenant, global models.Effect) string {
	parts := []string{}
	if proj != "not-applicable" {
		parts = append(parts, fmt.Sprintf("project:%s", proj))
	}
	if tenant != "not-applicable" {
		parts = append(parts, fmt.Sprintf("tenant:%s", tenant))
	}
	if global != "not-applicable" {
		parts = append(parts, fmt.Sprintf("global:%s", global))
	}
	if len(parts) == 0 {
		return "no policies matched"
	}
	return fmt.Sprintf("final=%s; %s", final, strings.Join(parts, "; "))
}

func (e *ABACEngine) DecideWithTrace(req *models.AccessRequest, algo models.CombiningAlgorithm) (*models.DecisionResult, map[string]interface{}, error) {
	start := time.Now()
	trace := make(map[string]interface{})

	resType, _ := req.Resource["type"].(string)
	globalSnap := e.GetGlobalSnapshot()
	tenantSnap := e.GetTenantSnapshot(req.TenantID)

	evalTrace := []map[string]interface{}{}

	var allMatched []matchedPolicy
	globalMatchedCount := 0
	tenantMatchedCount := 0

	if tenantSnap != nil {
		candidates := e.filterCandidates(tenantSnap, resType, req.Action)
		trace["tenant_candidates"] = len(candidates)
		for _, idx := range candidates {
			p := tenantSnap.Policies[idx]
			if p.Status != models.StatusEnabled {
				continue
			}
			matched := e.matchPolicy(&p, req)
			evalTrace = append(evalTrace, map[string]interface{}{
				"policy_id": p.ID,
				"level":     p.Level,
				"priority":  p.Priority,
				"effect":    p.Effect,
				"matched":   matched,
			})
			if matched {
				allMatched = append(allMatched, matchedPolicy{idx: idx, policy: p})
				if p.Level == models.LevelTenant {
					tenantMatchedCount++
				}
			}
		}
	}

	if globalSnap != nil {
		candidates := e.filterCandidates(globalSnap, resType, req.Action)
		trace["global_candidates"] = len(candidates)
		for _, idx := range candidates {
			p := globalSnap.Policies[idx]
			if p.Status != models.StatusEnabled {
				continue
			}
			matched := e.matchPolicy(&p, req)
			evalTrace = append(evalTrace, map[string]interface{}{
				"policy_id": p.ID,
				"level":     p.Level,
				"priority":  p.Priority,
				"effect":    p.Effect,
				"matched":   matched,
			})
			if matched {
				allMatched = append(allMatched, matchedPolicy{idx: idx, policy: p})
				globalMatchedCount++
			}
		}
	}

	trace["evaluated"] = evalTrace
	trace["global_matched"] = globalMatchedCount
	trace["tenant_matched"] = tenantMatchedCount

	result, err := e.Decide(req, algo)
	if err != nil {
		return nil, trace, err
	}
	result.DecisionTime = time.Since(start).Microseconds()
	trace["final_effect"] = result.Effect
	trace["matched_policies"] = result.MatchedPolicies
	return result, trace, nil
}
