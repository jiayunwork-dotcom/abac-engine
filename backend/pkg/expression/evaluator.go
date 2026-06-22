package expression

import (
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Operator string

const (
	OpEquals         Operator = "equals"
	OpNotEquals      Operator = "not_equals"
	OpContains       Operator = "contains"
	OpNotContains    Operator = "not_contains"
	OpRegexMatch     Operator = "regex_match"
	OpGreaterThan    Operator = "gt"
	OpGreaterOrEqual Operator = "gte"
	OpLessThan       Operator = "lt"
	OpLessOrEqual    Operator = "lte"
	OpIn             Operator = "in"
	OpNotIn          Operator = "not_in"
	OpIPInCIDR       Operator = "ip_in_cidr"
	OpTimeRange      Operator = "time_range"
	OpWeekdayRange   Operator = "weekday_range"
	OpIntersects     Operator = "intersects"
	OpExists         Operator = "exists"
)

type Evaluator struct{}

func NewEvaluator() *Evaluator {
	return &Evaluator{}
}

func (e *Evaluator) EvaluateCondition(attribute string, op string, value interface{}, attrs map[string]interface{}) (bool, error) {
	attrVal, exists := attrs[attribute]

	if op == string(OpExists) {
		wantExist, ok := value.(bool)
		if !ok {
			return false, fmt.Errorf("exists operator requires boolean value")
		}
		return exists == wantExist, nil
	}

	if !exists {
		return false, nil
	}

	operator := Operator(op)
	switch operator {
	case OpEquals:
		return e.equals(attrVal, value)
	case OpNotEquals:
		r, err := e.equals(attrVal, value)
		if err != nil {
			return false, err
		}
		return !r, nil
	case OpContains:
		return e.contains(attrVal, value)
	case OpNotContains:
		r, err := e.contains(attrVal, value)
		if err != nil {
			return false, err
		}
		return !r, nil
	case OpRegexMatch:
		return e.regexMatch(attrVal, value)
	case OpGreaterThan:
		return e.compare(attrVal, value, func(a, b float64) bool { return a > b })
	case OpGreaterOrEqual:
		return e.compare(attrVal, value, func(a, b float64) bool { return a >= b })
	case OpLessThan:
		return e.compare(attrVal, value, func(a, b float64) bool { return a < b })
	case OpLessOrEqual:
		return e.compare(attrVal, value, func(a, b float64) bool { return a <= b })
	case OpIn:
		return e.inSet(attrVal, value)
	case OpNotIn:
		r, err := e.inSet(attrVal, value)
		if err != nil {
			return false, err
		}
		return !r, nil
	case OpIPInCIDR:
		return e.ipInCIDR(attrVal, value)
	case OpTimeRange:
		return e.timeRange(attrVal, value)
	case OpWeekdayRange:
		return e.weekdayRange(attrVal, value)
	case OpIntersects:
		return e.intersects(attrVal, value)
	default:
		return false, fmt.Errorf("unknown operator: %s", op)
	}
}

func (e *Evaluator) equals(a, b interface{}) (bool, error) {
	as, aok := a.(string)
	bs, bok := b.(string)
	if aok && bok {
		return as == bs, nil
	}

	af, err1 := toFloat64(a)
	bf, err2 := toFloat64(b)
	if err1 == nil && err2 == nil {
		return af == bf, nil
	}

	ab, aok := a.(bool)
	bb, bok := b.(bool)
	if aok && bok {
		return ab == bb, nil
	}

	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b), nil
}

func (e *Evaluator) contains(a, b interface{}) (bool, error) {
	switch av := a.(type) {
	case string:
		bv, ok := b.(string)
		if !ok {
			return false, fmt.Errorf("contains on string requires string value")
		}
		return strings.Contains(av, bv), nil
	case []interface{}:
		for _, item := range av {
			if eq, _ := e.equals(item, b); eq {
				return true, nil
			}
		}
		return false, nil
	case []string:
		bv, ok := b.(string)
		if !ok {
			return false, nil
		}
		for _, item := range av {
			if item == bv {
				return true, nil
			}
		}
		return false, nil
	default:
		return false, fmt.Errorf("contains requires string or array attribute")
	}
}

func (e *Evaluator) regexMatch(a, b interface{}) (bool, error) {
	as, ok := a.(string)
	if !ok {
		return false, fmt.Errorf("regex_match requires string attribute")
	}
	bs, ok := b.(string)
	if !ok {
		return false, fmt.Errorf("regex_match requires string pattern")
	}
	re, err := regexp.Compile(bs)
	if err != nil {
		return false, fmt.Errorf("invalid regex: %v", err)
	}
	return re.MatchString(as), nil
}

func (e *Evaluator) compare(a, b interface{}, cmp func(float64, float64) bool) (bool, error) {
	af, err := toFloat64(a)
	if err != nil {
		return false, err
	}
	bf, err := toFloat64(b)
	if err != nil {
		return false, err
	}
	return cmp(af, bf), nil
}

func (e *Evaluator) inSet(a, b interface{}) (bool, error) {
	var set []interface{}
	switch bv := b.(type) {
	case []interface{}:
		set = bv
	case []string:
		set = make([]interface{}, len(bv))
		for i, s := range bv {
			set[i] = s
		}
	default:
		return false, fmt.Errorf("in operator requires array value")
	}
	for _, item := range set {
		if eq, _ := e.equals(a, item); eq {
			return true, nil
		}
	}
	return false, nil
}

func (e *Evaluator) ipInCIDR(a, b interface{}) (bool, error) {
	ipStr, ok := a.(string)
	if !ok {
		return false, fmt.Errorf("ip_in_cidr requires string IP attribute")
	}
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false, fmt.Errorf("invalid IP address: %s", ipStr)
	}

	switch bv := b.(type) {
	case string:
		_, cidr, err := net.ParseCIDR(bv)
		if err != nil {
			return false, fmt.Errorf("invalid CIDR: %v", err)
		}
		return cidr.Contains(ip), nil
	case []interface{}:
		for _, c := range bv {
			cs, ok := c.(string)
			if !ok {
				continue
			}
			_, cidr, err := net.ParseCIDR(cs)
			if err != nil {
				continue
			}
			if cidr.Contains(ip) {
				return true, nil
			}
		}
		return false, nil
	default:
		return false, fmt.Errorf("ip_in_cidr requires string or array CIDR value")
	}
}

func (e *Evaluator) timeRange(a, b interface{}) (bool, error) {
	var t time.Time
	switch av := a.(type) {
	case string:
		var err error
		t, err = time.Parse(time.RFC3339, av)
		if err != nil {
			t, err = time.Parse("2006-01-02 15:04:05", av)
			if err != nil {
				return false, fmt.Errorf("invalid time format: %s", av)
			}
		}
	case time.Time:
		t = av
	default:
		return false, fmt.Errorf("time_range requires time attribute")
	}

	bm, ok := b.(map[string]interface{})
	if !ok {
		return false, fmt.Errorf("time_range requires {start, end} map")
	}
	startStr, _ := bm["start"].(string)
	endStr, _ := bm["end"].(string)

	start, err := parseHHMM(startStr)
	if err != nil {
		return false, err
	}
	end, err := parseHHMM(endStr)
	if err != nil {
		return false, err
	}

	minutes := t.Hour()*60 + t.Minute()

	if start <= end {
		return minutes >= start && minutes <= end, nil
	}
	return minutes >= start || minutes <= end, nil
}

func (e *Evaluator) weekdayRange(a, b interface{}) (bool, error) {
	var t time.Time
	switch av := a.(type) {
	case string:
		var err error
		t, err = time.Parse(time.RFC3339, av)
		if err != nil {
			t, err = time.Parse("2006-01-02 15:04:05", av)
			if err != nil {
				return false, fmt.Errorf("invalid time format: %s", av)
			}
		}
	case time.Time:
		t = av
	default:
		return false, fmt.Errorf("weekday_range requires time attribute")
	}

	wd := int(t.Weekday())

	switch bv := b.(type) {
	case []interface{}:
		for _, d := range bv {
			df, err := toFloat64(d)
			if err != nil {
				continue
			}
			if int(df) == wd {
				return true, nil
			}
		}
		return false, nil
	case map[string]interface{}:
		startF, err := toFloat64(bv["start"])
		if err != nil {
			return false, err
		}
		endF, err := toFloat64(bv["end"])
		if err != nil {
			return false, err
		}
		start, end := int(startF), int(endF)
		if start <= end {
			return wd >= start && wd <= end, nil
		}
		return wd >= start || wd <= end, nil
	default:
		return false, fmt.Errorf("weekday_range requires array or {start,end}")
	}
}

func (e *Evaluator) intersects(a, b interface{}) (bool, error) {
	arrA := toStringSlice(a)
	arrB := toStringSlice(b)
	if arrA == nil || arrB == nil {
		return false, fmt.Errorf("intersects requires two arrays")
	}
	set := make(map[string]bool)
	for _, s := range arrA {
		set[s] = true
	}
	for _, s := range arrB {
		if set[s] {
			return true, nil
		}
	}
	return false, nil
}

func toFloat64(v interface{}) (float64, error) {
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
		return strconv.ParseFloat(val, 64)
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", v)
	}
}

func parseHHMM(s string) (int, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid time format, expect HH:MM: %s", s)
	}
	h, err := strconv.Atoi(parts[0])
	if err != nil || h < 0 || h > 23 {
		return 0, fmt.Errorf("invalid hour: %s", parts[0])
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil || m < 0 || m > 59 {
		return 0, fmt.Errorf("invalid minute: %s", parts[1])
	}
	return h*60 + m, nil
}

func toStringSlice(v interface{}) []string {
	switch val := v.(type) {
	case []string:
		return val
	case []interface{}:
		result := make([]string, len(val))
		for i, item := range val {
			result[i] = fmt.Sprintf("%v", item)
		}
		return result
	default:
		return nil
	}
}
