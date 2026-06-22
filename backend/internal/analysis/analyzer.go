package analysis

import (
	"fmt"
	"strings"

	"abac-engine/internal/models"
	"abac-engine/pkg/expression"
)

type PolicyAnalyzer struct {
	groupEval *expression.GroupEvaluator
}

func NewPolicyAnalyzer() *PolicyAnalyzer {
	return &PolicyAnalyzer{
		groupEval: expression.NewGroupEvaluator(),
	}
}

func (a *PolicyAnalyzer) DetectConflicts(
	newPolicy *models.Policy,
	existingPolicies []models.Policy,
	algo models.CombiningAlgorithm,
) []models.PolicyConflict {
	var conflicts []models.PolicyConflict

	for _, existing := range existingPolicies {
		if existing.ID == newPolicy.ID {
			continue
		}
		if existing.Level != newPolicy.Level {
			continue
		}
		if existing.ProjectID != newPolicy.ProjectID {
			continue
		}

		overlap, dims, desc := a.checkTargetOverlap(newPolicy, &existing)
		if !overlap {
			continue
		}

		if newPolicy.Effect == existing.Effect {
			continue
		}

		winnerID, winnerReason := a.determineWinner(newPolicy, &existing, algo)

		conflicts = append(conflicts, models.PolicyConflict{
			PolicyID:       existing.ID,
			OverlapDims:    dims,
			OverlapDesc:    desc,
			WinnerPolicyID: winnerID,
			WinnerReason:   winnerReason,
		})
	}

	return conflicts
}

func (a *PolicyAnalyzer) BuildDependencyGraph(
	policies []models.Policy,
	algo models.CombiningAlgorithm,
) *models.DependencyGraph {
	graph := &models.DependencyGraph{
		Nodes: make([]models.GraphNode, 0, len(policies)),
		Edges: []models.GraphEdge{},
	}

	policyMap := make(map[string]*models.Policy)
	for i := range policies {
		p := &policies[i]
		graph.Nodes = append(graph.Nodes, models.GraphNode{
			ID:          p.ID,
			Description: p.Description,
			Effect:      string(p.Effect),
			Priority:    p.Priority,
			Level:       string(p.Level),
		})
		policyMap[p.ID] = p
	}

	for i := 0; i < len(policies); i++ {
		for j := i + 1; j < len(policies); j++ {
			p1 := &policies[i]
			p2 := &policies[j]

			if p1.Level != p2.Level || p1.ProjectID != p2.ProjectID {
				continue
			}

			overlap, _, desc := a.checkTargetOverlap(p1, p2)

			if overlap && p1.Effect != p2.Effect {
				winnerID, _ := a.determineWinner(p1, p2, algo)
				loserID := p1.ID
				if winnerID == p1.ID {
					loserID = p2.ID
				}
				graph.Edges = append(graph.Edges, models.GraphEdge{
					Source: loserID,
					Target: winnerID,
					Type:   models.EdgeTypeConflict,
					Desc:   fmt.Sprintf("冲突: %s", desc),
				})
			} else if overlap && p1.Effect == p2.Effect {
				isSubset1, subsetDesc1 := a.isTargetSubset(p1, p2)
				isSubset2, subsetDesc2 := a.isTargetSubset(p2, p1)

				if isSubset1 && p1.Priority > p2.Priority {
					graph.Edges = append(graph.Edges, models.GraphEdge{
						Source: p1.ID,
						Target: p2.ID,
						Type:   models.EdgeTypeOverride,
						Desc:   fmt.Sprintf("覆盖: %s的条件更具体且优先级更高，覆盖%s", p1.ID, p2.ID),
					})
				} else if isSubset2 && p2.Priority > p1.Priority {
					graph.Edges = append(graph.Edges, models.GraphEdge{
						Source: p2.ID,
						Target: p1.ID,
						Type:   models.EdgeTypeOverride,
						Desc:   fmt.Sprintf("覆盖: %s的条件更具体且优先级更高，覆盖%s", p2.ID, p1.ID),
					})
				}
				_ = subsetDesc1
				_ = subsetDesc2
			} else if !overlap {
				complement, compDesc := a.checkComplementary(p1, p2)
				if complement {
					graph.Edges = append(graph.Edges, models.GraphEdge{
						Source: p1.ID,
						Target: p2.ID,
						Type:   models.EdgeTypeComplement,
						Desc:   compDesc,
					})
				}
			}
		}
	}

	return graph
}

func (a *PolicyAnalyzer) checkTargetOverlap(p1, p2 *models.Policy) (bool, []string, string) {
	var overlapDims []string
	var descParts []string

	if !slicesOverlap(p1.ResourceTypes, p2.ResourceTypes) && len(p1.ResourceTypes) > 0 && len(p2.ResourceTypes) > 0 {
		return false, nil, ""
	}

	if !slicesOverlap(p1.Actions, p2.Actions) && len(p1.Actions) > 0 && len(p2.Actions) > 0 {
		return false, nil, ""
	}

	subjectOverlap := a.checkGroupOverlap(p1.Target.Subject, p2.Target.Subject)
	resourceOverlap := a.checkGroupOverlap(p1.Target.Resource, p2.Target.Resource)
	actionOverlap := a.checkGroupOverlap(p1.Target.Action, p2.Target.Action)
	envOverlap := a.checkGroupOverlap(p1.Target.Environment, p2.Target.Environment)

	if expression.IsGroupEmpty(p1.Target.Subject) || expression.IsGroupEmpty(p2.Target.Subject) || subjectOverlap {
		if !expression.IsGroupEmpty(p1.Target.Subject) && !expression.IsGroupEmpty(p2.Target.Subject) {
			overlapDims = append(overlapDims, "subject")
			descParts = append(descParts, "主体条件可能重叠")
		}
	} else {
		return false, nil, ""
	}

	if expression.IsGroupEmpty(p1.Target.Resource) || expression.IsGroupEmpty(p2.Target.Resource) || resourceOverlap {
		if !expression.IsGroupEmpty(p1.Target.Resource) && !expression.IsGroupEmpty(p2.Target.Resource) {
			overlapDims = append(overlapDims, "resource")
			descParts = append(descParts, "资源条件可能重叠")
		}
	} else {
		return false, nil, ""
	}

	if expression.IsGroupEmpty(p1.Target.Action) || expression.IsGroupEmpty(p2.Target.Action) || actionOverlap {
		if !expression.IsGroupEmpty(p1.Target.Action) && !expression.IsGroupEmpty(p2.Target.Action) {
			overlapDims = append(overlapDims, "action")
			descParts = append(descParts, "动作条件可能重叠")
		}
	} else {
		return false, nil, ""
	}

	if expression.IsGroupEmpty(p1.Target.Environment) || expression.IsGroupEmpty(p2.Target.Environment) || envOverlap {
		if !expression.IsGroupEmpty(p1.Target.Environment) && !expression.IsGroupEmpty(p2.Target.Environment) {
			overlapDims = append(overlapDims, "environment")
			descParts = append(descParts, "环境条件可能重叠")
		}
	} else {
		return false, nil, ""
	}

	if len(overlapDims) == 0 {
		if len(p1.ResourceTypes) > 0 || len(p2.ResourceTypes) > 0 {
			overlapDims = append(overlapDims, "resource_type")
			descParts = append(descParts, "资源类型重叠")
		}
		if len(p1.Actions) > 0 || len(p2.Actions) > 0 {
			overlapDims = append(overlapDims, "action")
			descParts = append(descParts, "操作重叠")
		}
	}

	if len(overlapDims) == 0 {
		return true, []string{"general"}, "策略约束较少，可能存在重叠"
	}

	return true, overlapDims, strings.Join(descParts, "; ")
}

func (a *PolicyAnalyzer) checkGroupOverlap(g1, g2 *models.ConditionGroup) bool {
	if expression.IsGroupEmpty(g1) || expression.IsGroupEmpty(g2) {
		return true
	}

	attrs1 := a.extractAttributes(g1)
	attrs2 := a.extractAttributes(g2)

	hasCommonAttr := false
	for attr := range attrs1 {
		if attrs2[attr] {
			hasCommonAttr = true
			break
		}
	}

	if !hasCommonAttr {
		return true
	}

	for attr := range attrs1 {
		if !attrs2[attr] {
			continue
		}
		conds1 := a.findConditionsByAttr(g1, attr)
		conds2 := a.findConditionsByAttr(g2, attr)
		if !a.conditionsMayOverlap(conds1, conds2) {
			return false
		}
	}

	return true
}

func (a *PolicyAnalyzer) extractAttributes(g *models.ConditionGroup) map[string]bool {
	attrs := make(map[string]bool)
	if g == nil {
		return attrs
	}
	for _, c := range g.Conditions {
		if c.Attribute != "" {
			attrs[c.Attribute] = true
		}
	}
	for _, sg := range g.Groups {
		subAttrs := a.extractAttributes(&sg)
		for k := range subAttrs {
			attrs[k] = true
		}
	}
	return attrs
}

func (a *PolicyAnalyzer) findConditionsByAttr(g *models.ConditionGroup, attr string) []models.Condition {
	var result []models.Condition
	if g == nil {
		return result
	}
	for _, c := range g.Conditions {
		if c.Attribute == attr {
			result = append(result, c)
		}
	}
	for _, sg := range g.Groups {
		result = append(result, a.findConditionsByAttr(&sg, attr)...)
	}
	return result
}

func (a *PolicyAnalyzer) conditionsMayOverlap(conds1, conds2 []models.Condition) bool {
	if len(conds1) == 0 || len(conds2) == 0 {
		return true
	}

	for _, c1 := range conds1 {
		for _, c2 := range conds2 {
			if a.singleConditionMayOverlap(c1, c2) {
				return true
			}
		}
	}

	return false
}

func (a *PolicyAnalyzer) singleConditionMayOverlap(c1, c2 models.Condition) bool {
	switch c1.Operator {
	case "equals":
		switch c2.Operator {
		case "equals":
			return fmt.Sprintf("%v", c1.Value) == fmt.Sprintf("%v", c2.Value)
		case "not_equals":
			return fmt.Sprintf("%v", c1.Value) != fmt.Sprintf("%v", c2.Value)
		case "in":
			return a.valueInSlice(c1.Value, c2.Value)
		case "contains":
			s1, ok1 := c1.Value.(string)
			s2, ok2 := c2.Value.(string)
			return ok1 && ok2 && strings.Contains(s1, s2)
		default:
			return true
		}
	case "in":
		switch c2.Operator {
		case "equals":
			return a.valueInSlice(c2.Value, c1.Value)
		case "in":
			return a.slicesHaveIntersection(c1.Value, c2.Value)
		default:
			return true
		}
	case "gt", "gte", "lt", "lte":
		switch c2.Operator {
		case "gt", "gte", "lt", "lte":
			return a.rangeMayOverlap(c1, c2)
		default:
			return true
		}
	default:
		return true
	}
}

func (a *PolicyAnalyzer) rangeMayOverlap(c1, c2 models.Condition) bool {
	v1, err1 := toFloat(c1.Value)
	v2, err2 := toFloat(c2.Value)
	if err1 != nil || err2 != nil {
		return true
	}

	c1Lower, c1Upper := a.getRange(c1.Operator, v1)
	c2Lower, c2Upper := a.getRange(c2.Operator, v2)

	return c1Lower <= c2Upper && c2Lower <= c1Upper
}

func (a *PolicyAnalyzer) getRange(op string, v float64) (float64, float64) {
	const negInf = -1e18
	const posInf = 1e18
	switch op {
	case "gt":
		return v, posInf
	case "gte":
		return v, posInf
	case "lt":
		return negInf, v
	case "lte":
		return negInf, v
	default:
		return negInf, posInf
	}
}

func (a *PolicyAnalyzer) valueInSlice(val interface{}, slice interface{}) bool {
	switch s := slice.(type) {
	case []interface{}:
		for _, item := range s {
			if fmt.Sprintf("%v", item) == fmt.Sprintf("%v", val) {
				return true
			}
		}
	case []string:
		for _, item := range s {
			if item == fmt.Sprintf("%v", val) {
				return true
			}
		}
	}
	return false
}

func (a *PolicyAnalyzer) slicesHaveIntersection(s1, s2 interface{}) bool {
	set1 := a.toStringSet(s1)
	set2 := a.toStringSet(s2)
	for k := range set1 {
		if set2[k] {
			return true
		}
	}
	return false
}

func (a *PolicyAnalyzer) toStringSet(s interface{}) map[string]bool {
	set := make(map[string]bool)
	switch v := s.(type) {
	case []interface{}:
		for _, item := range v {
			set[fmt.Sprintf("%v", item)] = true
		}
	case []string:
		for _, item := range v {
			set[item] = true
		}
	}
	return set
}

func (a *PolicyAnalyzer) determineWinner(
	p1, p2 *models.Policy,
	algo models.CombiningAlgorithm,
) (string, string) {
	switch algo {
	case models.AlgoDenyOverride:
		if p1.Effect == models.EffectDeny {
			return p1.ID, "拒绝优先算法下，deny策略胜出"
		}
		return p2.ID, "拒绝优先算法下，deny策略胜出"

	case models.AlgoPermitOverride:
		if p1.Effect == models.EffectPermit {
			return p1.ID, "允许优先算法下，permit策略胜出"
		}
		return p2.ID, "允许优先算法下，permit策略胜出"

	case models.AlgoPriorityFirst:
		if p1.Priority > p2.Priority {
			return p1.ID, fmt.Sprintf("优先级优先算法下，%s优先级(%d)更高", p1.ID, p1.Priority)
		} else if p2.Priority > p1.Priority {
			return p2.ID, fmt.Sprintf("优先级优先算法下，%s优先级(%d)更高", p2.ID, p2.Priority)
		}
		return p1.ID, "优先级相同，按策略顺序前者胜出"

	default:
		if p1.Effect == models.EffectDeny {
			return p1.ID, "默认拒绝优先算法下，deny策略胜出"
		}
		return p2.ID, "默认拒绝优先算法下，deny策略胜出"
	}
}

func (a *PolicyAnalyzer) isTargetSubset(p1, p2 *models.Policy) (bool, string) {
	if !slicesSubset(p1.ResourceTypes, p2.ResourceTypes) && len(p2.ResourceTypes) > 0 {
		return false, ""
	}
	if !slicesSubset(p1.Actions, p2.Actions) && len(p2.Actions) > 0 {
		return false, ""
	}

	subjectSubset := a.isGroupSubset(p1.Target.Subject, p2.Target.Subject)
	resourceSubset := a.isGroupSubset(p1.Target.Resource, p2.Target.Resource)

	if !subjectSubset || !resourceSubset {
		return false, ""
	}

	return true, fmt.Sprintf("%s的条件是%s的子集", p1.ID, p2.ID)
}

func (a *PolicyAnalyzer) isGroupSubset(g1, g2 *models.ConditionGroup) bool {
	if expression.IsGroupEmpty(g2) {
		return true
	}
	if expression.IsGroupEmpty(g1) && !expression.IsGroupEmpty(g2) {
		return false
	}
	return a.checkGroupOverlap(g1, g2) && a.hasMoreConditions(g1, g2)
}

func (a *PolicyAnalyzer) hasMoreConditions(g1, g2 *models.ConditionGroup) bool {
	count1 := a.countConditions(g1)
	count2 := a.countConditions(g2)
	return count1 >= count2
}

func (a *PolicyAnalyzer) countConditions(g *models.ConditionGroup) int {
	if g == nil {
		return 0
	}
	count := len(g.Conditions)
	for _, sg := range g.Groups {
		count += a.countConditions(&sg)
	}
	return count
}

func (a *PolicyAnalyzer) checkComplementary(p1, p2 *models.Policy) (bool, string) {
	if len(p1.ResourceTypes) == 0 || len(p2.ResourceTypes) == 0 {
		return false, ""
	}

	commonRT := intersection(p1.ResourceTypes, p2.ResourceTypes)
	if len(commonRT) == 0 {
		return false, ""
	}

	if p1.Effect != p2.Effect {
		return false, ""
	}

	for _, rt := range commonRT {
		actions1 := a.getActionsForResource(p1, rt)
		actions2 := a.getActionsForResource(p2, rt)

		if len(actions1) > 0 && len(actions2) > 0 {
			union := union(actions1, actions2)
			if len(union) >= len(actions1)+len(actions2)/2 {
				return true, fmt.Sprintf("互补: 对资源类型%s共同覆盖%d种操作", rt, len(union))
			}
		}
	}

	return false, ""
}

func (a *PolicyAnalyzer) getActionsForResource(p *models.Policy, resourceType string) []string {
	if len(p.Actions) > 0 {
		return p.Actions
	}
	return []string{"*"}
}

func slicesOverlap(a, b []string) bool {
	if len(a) == 0 || len(b) == 0 {
		return true
	}
	set := make(map[string]bool)
	for _, s := range a {
		if s == "*" {
			return true
		}
		set[strings.ToLower(s)] = true
	}
	for _, s := range b {
		if s == "*" {
			return true
		}
		if set[strings.ToLower(s)] {
			return true
		}
	}
	return false
}

func slicesSubset(sub, super []string) bool {
	if len(super) == 0 {
		return true
	}
	if len(sub) == 0 {
		return false
	}
	set := make(map[string]bool)
	for _, s := range super {
		if s == "*" {
			return true
		}
		set[strings.ToLower(s)] = true
	}
	for _, s := range sub {
		if s == "*" {
			continue
		}
		if !set[strings.ToLower(s)] {
			return false
		}
	}
	return true
}

func intersection(a, b []string) []string {
	set := make(map[string]bool)
	for _, s := range a {
		set[strings.ToLower(s)] = true
	}
	var result []string
	seen := make(map[string]bool)
	for _, s := range b {
		sl := strings.ToLower(s)
		if set[sl] && !seen[sl] {
			result = append(result, s)
			seen[sl] = true
		}
	}
	return result
}

func union(a, b []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range a {
		sl := strings.ToLower(s)
		if !seen[sl] {
			result = append(result, s)
			seen[sl] = true
		}
	}
	for _, s := range b {
		sl := strings.ToLower(s)
		if !seen[sl] {
			result = append(result, s)
			seen[sl] = true
		}
	}
	return result
}

func toFloat(v interface{}) (float64, error) {
	switch val := v.(type) {
	case float64:
		return val, nil
	case float32:
		return float64(val), nil
	case int:
		return float64(val), nil
	case int32:
		return float64(val), nil
	case int64:
		return float64(val), nil
	case string:
		return 0, fmt.Errorf("cannot convert string to float")
	default:
		return 0, fmt.Errorf("unsupported type")
	}
}
