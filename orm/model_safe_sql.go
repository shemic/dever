package orm

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/shemic/dever/util"
)

// SQLRaw marks an intentional raw SQL fragment. It should only be used with
// constant, trusted strings; request values must stay in parameterized filters.
type SQLRaw string

func RawSQL(value string) SQLRaw {
	return SQLRaw(strings.TrimSpace(value))
}

func safeSelectFields(raw any, fallback string) string {
	switch value := raw.(type) {
	case nil:
		return fallback
	case SQLRaw:
		return rawSQLString(value, fallback)
	case string:
		fields := safeFieldList(value)
		if fields != "" {
			return fields
		}
	default:
		fields := safeFieldList(util.ToStringTrimmed(value))
		if fields != "" {
			return fields
		}
	}
	return fallback
}

func safeOrderClause(raw any, fallback string) string {
	switch value := raw.(type) {
	case nil:
		return fallback
	case SQLRaw:
		return rawSQLString(value, fallback)
	case string:
		order := safeOrderList(value)
		if order != "" || strings.TrimSpace(value) == "" {
			return order
		}
	default:
		order := safeOrderList(util.ToStringTrimmed(value))
		if order != "" {
			return order
		}
	}
	return fallback
}

func safeAggregateExpression(raw any, fallback string) string {
	switch value := raw.(type) {
	case nil:
		return fallback
	case SQLRaw:
		return rawSQLString(value, fallback)
	case string:
		if expr := safeGeneratedAggregate(value); expr != "" {
			return expr
		}
		expr := safeAggregateIdentifier(value)
		if expr != "" {
			return expr
		}
	default:
		expr := safeAggregateIdentifier(util.ToStringTrimmed(value))
		if expr != "" {
			return expr
		}
	}
	return fallback
}

func safeLimitClause(raw any) string {
	switch value := raw.(type) {
	case nil:
		return ""
	case SQLRaw:
		return rawSQLString(value, "")
	case string:
		return safeLimitText(value)
	default:
		limit, ok := util.ParseInt64(value)
		if !ok || limit <= 0 {
			return ""
		}
		return strconv.FormatInt(limit, 10)
	}
}

func safeISValue(value any) string {
	if value == nil {
		return "NULL"
	}
	switch v := value.(type) {
	case bool:
		if v {
			return "TRUE"
		}
		return "FALSE"
	}
	switch strings.ToUpper(strings.TrimSpace(util.ToString(value))) {
	case "NULL":
		return "NULL"
	case "TRUE":
		return "TRUE"
	case "FALSE":
		return "FALSE"
	case "UNKNOWN":
		return "UNKNOWN"
	default:
		return ""
	}
}

func safeFieldList(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return ""
		}
		if part == "main.*" {
			result = append(result, part)
			continue
		}
		if err := ensureQualifiedIdentifier(part); err != nil {
			return ""
		}
		result = append(result, part)
	}
	return strings.Join(result, ", ")
}

func safeOrderList(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		clause := safeOrderItem(part)
		if clause == "" {
			return ""
		}
		result = append(result, clause)
	}
	return strings.Join(result, ", ")
}

func safeOrderItem(value string) string {
	fields := strings.Fields(strings.TrimSpace(value))
	if len(fields) == 0 || len(fields) > 2 {
		return ""
	}
	field := fields[0]
	if err := ensureQualifiedIdentifier(field); err != nil {
		return ""
	}
	if len(fields) == 1 {
		return field
	}
	direction := strings.ToUpper(strings.TrimSpace(fields[1]))
	switch direction {
	case "ASC", "DESC":
		return field + " " + strings.ToLower(direction)
	default:
		return ""
	}
}

func safeAggregateIdentifier(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if value == "*" {
		return "*"
	}
	if err := ensureQualifiedIdentifier(value); err != nil {
		return ""
	}
	return value
}

func safeGeneratedAggregate(value string) string {
	compact := strings.Join(strings.Fields(strings.TrimSpace(value)), "")
	normalized := strings.ToUpper(compact)
	switch {
	case normalized == "COUNT(*)":
		return "COUNT(*)"
	case strings.HasPrefix(normalized, "SUM(") && strings.HasSuffix(normalized, ")"):
		inner := strings.TrimSuffix(compact[4:], ")")
		if field := safeAggregateIdentifier(inner); field != "" {
			return "SUM(" + field + ")"
		}
	}
	return ""
}

func safeLimitText(value string) string {
	limit, ok := util.ParseInt64(strings.TrimSpace(value))
	if !ok || limit <= 0 {
		return ""
	}
	return strconv.FormatInt(limit, 10)
}

func rawSQLString(value SQLRaw, fallback string) string {
	text := strings.TrimSpace(string(value))
	if text == "" {
		return fallback
	}
	return text
}

func safeJoinType(raw string) string {
	joinType := strings.ToUpper(strings.Join(strings.Fields(strings.TrimSpace(raw)), " "))
	if joinType == "" {
		return "LEFT JOIN"
	}
	if !strings.Contains(joinType, "JOIN") {
		joinType += " JOIN"
	}
	switch joinType {
	case "JOIN", "INNER JOIN", "LEFT JOIN", "LEFT OUTER JOIN", "RIGHT JOIN", "RIGHT OUTER JOIN", "FULL JOIN", "FULL OUTER JOIN", "CROSS JOIN":
		return joinType
	default:
		return ""
	}
}

func safeJoinOnClause(raw any) string {
	switch value := raw.(type) {
	case SQLRaw:
		return rawSQLString(value, "")
	case string:
		return safeJoinComparisonList(value)
	default:
		return safeJoinComparisonList(util.ToStringTrimmed(value))
	}
}

func safeJoinComparisonList(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	parts := splitJoinComparisons(value)
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		clause := safeJoinComparison(part)
		if clause == "" {
			return ""
		}
		result = append(result, clause)
	}
	return strings.Join(result, " AND ")
}

func splitJoinComparisons(value string) []string {
	fields := strings.Fields(value)
	parts := make([]string, 0, 1)
	var current []string
	for _, field := range fields {
		if strings.EqualFold(field, "AND") {
			if len(current) > 0 {
				parts = append(parts, strings.Join(current, " "))
				current = nil
			}
			continue
		}
		current = append(current, field)
	}
	if len(current) > 0 {
		parts = append(parts, strings.Join(current, " "))
	}
	return parts
}

func safeJoinComparison(value string) string {
	fields := strings.Fields(strings.TrimSpace(value))
	if len(fields) != 3 || fields[1] != "=" {
		return ""
	}
	if err := ensureQualifiedIdentifier(fields[0]); err != nil {
		return ""
	}
	if err := ensureQualifiedIdentifier(fields[2]); err != nil {
		return ""
	}
	return fmt.Sprintf("%s = %s", fields[0], fields[2])
}
