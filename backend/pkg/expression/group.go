package expression

import (
	"fmt"
	"strings"

	"abac-engine/internal/models"
)

type GroupEvaluator struct {
	condEval *Evaluator
}

func NewGroupEvaluator() *GroupEvaluator {
	return &GroupEvaluator{
		condEval: NewEvaluator(),
	}
}

func (g *GroupEvaluator) EvaluateGroup(group *models.ConditionGroup, attrs map[string]interface{}) (bool, error) {
	if group == nil {
		return true, nil
	}
	if len(group.Conditions) == 0 && len(group.Groups) == 0 {
		return true, nil
	}

	logic := strings.ToUpper(group.Logic)
	if logic == "" {
		logic = "AND"
	}
	if logic != "AND" && logic != "OR" {
		return false, fmt.Errorf("invalid logic: %s", group.Logic)
	}

	results := make([]bool, 0, len(group.Conditions)+len(group.Groups))

	for _, cond := range group.Conditions {
		r, err := g.condEval.EvaluateCondition(cond.Attribute, cond.Operator, cond.Value, attrs)
		if err != nil {
			return false, err
		}
		results = append(results, r)
	}

	for _, subGroup := range group.Groups {
		r, err := g.EvaluateGroup(&subGroup, attrs)
		if err != nil {
			return false, err
		}
		results = append(results, r)
	}

	if len(results) == 0 {
		return false, nil
	}

	if logic == "AND" {
		for _, r := range results {
			if !r {
				return false, nil
			}
		}
		return true, nil
	}

	for _, r := range results {
		if r {
			return true, nil
		}
	}
	return false, nil
}

func (g *GroupEvaluator) ValidateGroup(group *models.ConditionGroup, validAttrs []string) []error {
	if group == nil {
		return nil
	}

	var errs []error

	logic := strings.ToUpper(group.Logic)
	if logic != "" && logic != "AND" && logic != "OR" {
		errs = append(errs, fmt.Errorf("invalid logic operator: %s (must be AND/OR)", group.Logic))
	}

	attrSet := make(map[string]bool)
	for _, a := range validAttrs {
		attrSet[a] = true
	}

	for _, cond := range group.Conditions {
		if cond.Attribute == "" {
			errs = append(errs, fmt.Errorf("condition attribute cannot be empty"))
			continue
		}
		if len(validAttrs) > 0 && !attrSet[cond.Attribute] {
			errs = append(errs, fmt.Errorf("unknown attribute: %s (valid: %v)", cond.Attribute, validAttrs))
		}
		if _, err := validateOperator(cond.Operator); err != nil {
			errs = append(errs, fmt.Errorf("attribute %s: %v", cond.Attribute, err))
		}
	}

	for _, sg := range group.Groups {
		subErrs := g.ValidateGroup(&sg, validAttrs)
		errs = append(errs, subErrs...)
	}

	return errs
}

func validateOperator(op string) (Operator, error) {
	operators := []Operator{
		OpEquals, OpNotEquals, OpContains, OpNotContains, OpRegexMatch,
		OpGreaterThan, OpGreaterOrEqual, OpLessThan, OpLessOrEqual,
		OpIn, OpNotIn, OpIPInCIDR, OpTimeRange, OpWeekdayRange, OpIntersects, OpExists,
	}
	for _, o := range operators {
		if string(o) == op {
			return o, nil
		}
	}
	return "", fmt.Errorf("unknown operator: %s", op)
}

var (
	ValidSubjectAttributes = []string{
		"user_id", "username", "roles", "department", "department_id",
		"level", "title", "tags", "email_domain", "is_admin",
		"manager_id", "region", "tenure_days",
	}
	ValidResourceAttributes = []string{
		"id", "type", "name", "owner_id", "owner_dept",
		"sensitivity_level", "created_at", "updated_at",
		"project_id", "status", "tags", "size_bytes",
	}
	ValidActionAttributes = []string{
		"name", "category",
	}
	ValidEnvAttributes = []string{
		"timestamp", "client_ip", "user_agent", "device_type",
		"device_os", "browser", "country", "region", "is_mfa_authenticated",
	}
)

func IsGroupEmpty(g *models.ConditionGroup) bool {
	if g == nil {
		return true
	}
	if len(g.Conditions) == 0 && len(g.Groups) == 0 {
		return true
	}
	if len(g.Conditions) > 0 {
		for _, c := range g.Conditions {
			if c.Attribute != "" || c.Operator != "" {
				return false
			}
		}
	}
	if len(g.Groups) > 0 {
		for _, sg := range g.Groups {
			if !IsGroupEmpty(&sg) {
				return false
			}
		}
	}
	return true
}

func IsTargetEmpty(t models.Target) bool {
	return IsGroupEmpty(t.Subject) &&
		IsGroupEmpty(t.Resource) &&
		IsGroupEmpty(t.Action) &&
		IsGroupEmpty(t.Environment)
}
