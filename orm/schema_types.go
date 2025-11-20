package orm

import (
	"strings"
	"time"
	"unicode"
)

type columnDef struct {
	Name          string  `json:"name"`
	Type          string  `json:"type"`
	NotNull       bool    `json:"notNull"`
	Primary       bool    `json:"primary"`
	AutoIncrement bool    `json:"autoIncrement"`
	DefaultValue  *string `json:"defaultValue,omitempty"`
	DefaultIsRaw  bool    `json:"defaultIsRaw,omitempty"`
}

type indexDef struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
	Unique  bool     `json:"unique"`
	Type    string   `json:"type,omitempty"`
}

type tableSchema struct {
	Table     string           `json:"table"`
	Columns   []columnDef      `json:"columns"`
	Indexes   []indexDef       `json:"indexes,omitempty"`
	Seeds     []map[string]any `json:"seeds,omitempty"`
	UpdatedAt time.Time        `json:"updatedAt"`
	columnLookup map[string]string `json:"-"`
}

func (s *tableSchema) ensureLookup() {
	if s == nil || s.columnLookup != nil {
		return
	}
	lookup := make(map[string]string, len(s.Columns))
	for _, col := range s.Columns {
		lookup[normalizeColumnKey(col.Name)] = col.Name
	}
	s.columnLookup = lookup
}

func (s *tableSchema) resolveColumn(name string) (string, bool) {
	if s == nil {
		return "", false
	}
	s.ensureLookup()
	key := normalizeColumnKey(name)
	val, ok := s.columnLookup[key]
	return val, ok
}

func normalizeColumnKey(name string) string {
	if name == "" {
		return ""
	}
	var builder strings.Builder
	builder.Grow(len(name))
	for _, r := range name {
		if r == '_' {
			continue
		}
		builder.WriteRune(unicode.ToLower(r))
	}
	return builder.String()
}
