package orm

import (
	"strings"
	"sync"
	"time"
	"unicode"
)

type columnDef struct {
	Name          string  `json:"name"`
	Type          string  `json:"type"`
	Comment       string  `json:"comment,omitempty"`
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
	Table           string               `json:"table"`
	Columns         []columnDef          `json:"columns"`
	Indexes         []indexDef           `json:"indexes,omitempty"`
	Seeds           []map[string]any     `json:"seeds,omitempty"`
	UpdatedAt       time.Time            `json:"updatedAt"`
	columnLookup    map[string]string    `json:"-"`
	columnDefLookup map[string]columnDef `json:"-"`
	aliasMu         sync.RWMutex         `json:"-"`
	aliasDefLookup  map[string]columnDef `json:"-"`
	aliasKeyLookup  map[string]string    `json:"-"`
	aliasMiss       map[string]struct{}  `json:"-"`
	labelLookup     map[string]string    `json:"-"`
}

func (s *tableSchema) ensureLookup() {
	if s == nil || s.columnLookup != nil {
		return
	}
	lookup := make(map[string]string, len(s.Columns))
	defLookup := make(map[string]columnDef, len(s.Columns))
	labelLookup := make(map[string]string, len(s.Columns))
	for _, col := range s.Columns {
		key := normalizeColumnKey(col.Name)
		lookup[key] = col.Name
		defLookup[key] = col
		if strings.TrimSpace(col.Comment) != "" {
			labelLookup[key] = strings.TrimSpace(col.Comment)
		}
	}
	s.columnLookup = lookup
	s.columnDefLookup = defLookup
	s.labelLookup = labelLookup
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

func (s *tableSchema) resolveColumnDef(name string) (columnDef, bool) {
	if s == nil {
		return columnDef{}, false
	}
	s.ensureLookup()
	key := normalizeColumnKey(name)
	val, ok := s.columnDefLookup[key]
	return val, ok
}

func (s *tableSchema) resolveLabel(name string) (string, bool) {
	if s == nil {
		return "", false
	}
	s.ensureLookup()
	key := normalizeColumnKey(name)
	if label, ok := s.labelLookup[key]; ok {
		return label, true
	}
	if strings.Contains(name, ".") {
		if idx := strings.LastIndex(name, "."); idx != -1 && idx+1 < len(name) {
			key = normalizeColumnKey(name[idx+1:])
			if label, ok := s.labelLookup[key]; ok {
				return label, true
			}
		}
	}
	return "", false
}

func (s *tableSchema) labels() map[string]string {
	if s == nil {
		return nil
	}
	s.ensureLookup()
	if len(s.labelLookup) == 0 {
		return nil
	}
	result := make(map[string]string, len(s.labelLookup))
	for key, value := range s.labelLookup {
		result[key] = value
	}
	return result
}

func (s *tableSchema) resolveColumnDefWithAlias(name string) (columnDef, string, bool) {
	if s == nil || name == "" {
		return columnDef{}, "", false
	}
	s.aliasMu.RLock()
	if s.aliasDefLookup != nil {
		if col, ok := s.aliasDefLookup[name]; ok {
			key := name
			if s.aliasKeyLookup != nil {
				if mapped, ok := s.aliasKeyLookup[name]; ok {
					key = mapped
				}
			}
			s.aliasMu.RUnlock()
			return col, key, true
		}
		if s.aliasMiss != nil {
			if _, ok := s.aliasMiss[name]; ok {
				s.aliasMu.RUnlock()
				return columnDef{}, "", false
			}
		}
	}
	s.aliasMu.RUnlock()

	col, ok := s.resolveColumnDef(name)
	lookupKey := name
	if !ok && strings.Contains(name, ".") {
		if idx := strings.LastIndex(name, "."); idx != -1 && idx+1 < len(name) {
			lookupKey = name[idx+1:]
			col, ok = s.resolveColumnDef(lookupKey)
		}
	}

	s.aliasMu.Lock()
	if s.aliasDefLookup == nil {
		s.aliasDefLookup = map[string]columnDef{}
		s.aliasKeyLookup = map[string]string{}
		s.aliasMiss = map[string]struct{}{}
	}
	if ok {
		s.aliasDefLookup[name] = col
		s.aliasKeyLookup[name] = lookupKey
	} else {
		s.aliasMiss[name] = struct{}{}
	}
	s.aliasMu.Unlock()

	if ok {
		return col, lookupKey, true
	}
	return columnDef{}, "", false
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
