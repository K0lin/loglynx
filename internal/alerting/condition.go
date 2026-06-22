package alerting

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Condition is one predicate applied to the http_requests table.
type Condition struct {
	Field    string `json:"field"`    // e.g. "path", "status_code", "client_ip"
	Operator string `json:"operator"` // eq, neq, contains, not_contains, starts_with, ends_with, gte, lte, gt, lt, in
	Value    string `json:"value"`
}

// allowedFields maps user-visible field names to SQL column names.
// Only fields in this map can appear in conditions (prevents SQL injection).
var allowedFields = map[string]string{
	"method":           "method",
	"path":             "path",
	"host":             "host",
	"status_code":      "status_code",
	"response_time_ms": "response_time_ms",
	"client_ip":        "client_ip",
	"geo_country":      "geo_country",
	"backend_name":     "backend_name",
	"user_agent":       "user_agent",
	"source_name":      "source_name",
	"device_type":      "device_type",
	"router_name":      "router_name",
	"request_scheme":   "request_scheme",
}

// FieldLabels provides human-readable names for the UI.
var FieldLabels = map[string]string{
	"method":           "HTTP Method",
	"path":             "Request Path",
	"host":             "Host",
	"status_code":      "Status Code",
	"response_time_ms": "Response Time (ms)",
	"client_ip":        "Client IP",
	"geo_country":      "Country (ISO code)",
	"backend_name":     "Backend Name",
	"user_agent":       "User Agent",
	"source_name":      "Log Source",
	"device_type":      "Device Type",
	"router_name":      "Router Name",
	"request_scheme":   "Scheme (http/https)",
}

func ParseConditions(raw string) ([]Condition, error) {
	if raw == "" || raw == "null" {
		return nil, nil
	}
	var conds []Condition
	if err := json.Unmarshal([]byte(raw), &conds); err != nil {
		return nil, fmt.Errorf("parse conditions: %w", err)
	}
	return conds, nil
}

// BuildWhereClause converts conditions into a safe parameterised SQL fragment.
// Returns ("1=1", nil, nil) when conditions is empty (matches all rows).
func BuildWhereClause(conditions []Condition) (string, []interface{}, error) {
	if len(conditions) == 0 {
		return "1=1", nil, nil
	}

	clauses := make([]string, 0, len(conditions))
	args := make([]interface{}, 0)

	for _, c := range conditions {
		col, ok := allowedFields[c.Field]
		if !ok {
			return "", nil, fmt.Errorf("unknown field: %q", c.Field)
		}

		switch c.Operator {
		case "eq":
			clauses = append(clauses, col+" = ?")
			args = append(args, c.Value)

		case "neq":
			clauses = append(clauses, col+" != ?")
			args = append(args, c.Value)

		case "contains":
			clauses = append(clauses, col+` LIKE ? ESCAPE '\'`)
			args = append(args, "%"+escapeLike(c.Value)+"%")

		case "not_contains":
			clauses = append(clauses, col+` NOT LIKE ? ESCAPE '\'`)
			args = append(args, "%"+escapeLike(c.Value)+"%")

		case "starts_with":
			clauses = append(clauses, col+` LIKE ? ESCAPE '\'`)
			args = append(args, escapeLike(c.Value)+"%")

		case "ends_with":
			clauses = append(clauses, col+` LIKE ? ESCAPE '\'`)
			args = append(args, "%"+escapeLike(c.Value))

		case "gte":
			clauses = append(clauses, col+" >= ?")
			args = append(args, c.Value)

		case "lte":
			clauses = append(clauses, col+" <= ?")
			args = append(args, c.Value)

		case "gt":
			clauses = append(clauses, col+" > ?")
			args = append(args, c.Value)

		case "lt":
			clauses = append(clauses, col+" < ?")
			args = append(args, c.Value)

		case "in":
			vals := strings.Split(c.Value, ",")
			placeholders := make([]string, 0, len(vals))
			for _, v := range vals {
				v = strings.TrimSpace(v)
				if v != "" {
					placeholders = append(placeholders, "?")
					args = append(args, v)
				}
			}
			if len(placeholders) == 0 {
				continue
			}
			clauses = append(clauses, col+" IN ("+strings.Join(placeholders, ",")+")")

		default:
			return "", nil, fmt.Errorf("unknown operator: %q", c.Operator)
		}
	}

	if len(clauses) == 0 {
		return "1=1", nil, nil
	}
	return strings.Join(clauses, " AND "), args, nil
}

// GroupByColumn returns the SQL column expression for the given group_by value.
func GroupByColumn(groupBy string) string {
	switch groupBy {
	case "client_ip":
		return "client_ip"
	case "geo_country":
		return "geo_country"
	case "host":
		return "host"
	case "backend_name":
		return "backend_name"
	case "source_name":
		return "source_name"
	default:
		return "'global'"
	}
}

func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}
