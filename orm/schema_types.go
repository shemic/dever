package orm

import "time"

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
}
