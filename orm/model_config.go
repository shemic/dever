package orm

import "strings"

const (
	FieldTypeHidden   = "hidden"
	FieldTypePassword = "password"
)

type ModelConfig struct {
	Name     string
	Table    string
	Labels   map[string]string
	Index    any
	Indexes  []any
	Order    string
	Database string
	Seeds    []map[string]any

	Options   map[string]any
	Relations []Relation
	Fields    map[string]FieldConfig

	schema any
}

type FieldConfig struct {
	Type string
}

type Relation struct {
	Kind             string
	Name             string
	Field            string
	Through          string
	Option           string
	Mode             string
	OptionKeys       []string
	RowKey           string
	OwnerField       string
	TargetField      string
	OptionValueField string
	OptionLabelField string
	EmptyValue       any
	Order            string
	ThroughOrder     string
	OptionOrder      string
}

func (c ModelConfig) clone() ModelConfig {
	result := c
	result.Labels = cloneStringMap(c.Labels)
	result.Seeds = cloneMapSlice(c.Seeds)
	result.Options = cloneAnyMap(c.Options)
	result.Relations = cloneRelations(c.Relations)
	result.Fields = cloneFieldConfigs(c.Fields)
	if len(c.Indexes) > 0 {
		result.Indexes = append([]any(nil), c.Indexes...)
	}
	return result
}

func normalizeModelConfig(config ModelConfig) ModelConfig {
	result := config.clone()
	result.Name = strings.TrimSpace(result.Name)
	result.Table = strings.TrimSpace(result.Table)
	result.Order = strings.TrimSpace(result.Order)
	result.Database = strings.TrimSpace(result.Database)

	if len(result.Fields) > 0 {
		fields := make(map[string]FieldConfig, len(result.Fields))
		for field, config := range result.Fields {
			field = strings.TrimSpace(field)
			if field == "" {
				continue
			}
			config.Type = strings.ToLower(strings.TrimSpace(config.Type))
			if config.Type == "" {
				continue
			}
			fields[field] = config
		}
		result.Fields = fields
	}

	return result
}

func (c ModelConfig) withSchema(schema any) ModelConfig {
	c.schema = schema
	return c
}

func (c ModelConfig) indexModels() []any {
	result := make([]any, 0, len(c.Indexes)+1)
	if isStructLike(c.Index) {
		result = append(result, c.Index)
	}
	for _, index := range c.Indexes {
		if isStructLike(index) {
			result = append(result, index)
		}
	}
	return result
}

func (c ModelConfig) withRuntimeMeta(name, table, database, order string, labels map[string]string) ModelConfig {
	result := c.clone()
	result.Name = strings.TrimSpace(name)
	result.Table = strings.TrimSpace(table)
	result.Database = strings.TrimSpace(database)
	result.Order = strings.TrimSpace(order)
	result.Labels = cloneStringMap(labels)
	return result
}

func cloneStringMap(source map[string]string) map[string]string {
	if len(source) == 0 {
		return nil
	}
	result := make(map[string]string, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func cloneAnyMap(source map[string]any) map[string]any {
	if len(source) == 0 {
		return nil
	}
	result := make(map[string]any, len(source))
	for key, value := range source {
		result[key] = cloneConfigValue(value)
	}
	return result
}

func cloneMapSlice(source []map[string]any) []map[string]any {
	if len(source) == 0 {
		return nil
	}
	result := make([]map[string]any, 0, len(source))
	for _, item := range source {
		result = append(result, cloneAnyMap(item))
	}
	return result
}

func cloneRelations(source []Relation) []Relation {
	if len(source) == 0 {
		return nil
	}
	result := make([]Relation, len(source))
	for index, relation := range source {
		result[index] = relation
		if len(relation.OptionKeys) > 0 {
			result[index].OptionKeys = append([]string(nil), relation.OptionKeys...)
		}
	}
	return result
}

func cloneFieldConfigs(source map[string]FieldConfig) map[string]FieldConfig {
	if len(source) == 0 {
		return nil
	}
	result := make(map[string]FieldConfig, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func cloneConfigValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneAnyMap(typed)
	case []map[string]any:
		return cloneMapSlice(typed)
	case []any:
		result := make([]any, 0, len(typed))
		for _, item := range typed {
			result = append(result, cloneConfigValue(item))
		}
		return result
	default:
		return value
	}
}
