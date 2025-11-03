package orm

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	schemaMu   sync.RWMutex
	registered = map[string]*tableSchema{}
)

var (
	schemaOnceMap sync.Map
)

// SeedData 用于描述默认插入的数据行。
type SeedData struct {
	Rows []map[string]any
}

// Seeds 构造 SeedData，便于调用侧传入默认数据。
func Seeds(rows ...map[string]any) SeedData {
	return SeedData{Rows: cloneSeedRows(rows)}
}

var schemaSearchDirs = []string{
	filepath.Join("data"),
	filepath.Join("data", "table"),
}

// schemaFileCandidates returns possible file paths for the given table schema.
func schemaFileCandidates(table string) []string {
	table = strings.TrimSpace(table)
	if table == "" {
		return nil
	}
	candidates := []string{}
	lower := strings.ToLower(table)
	candidates = append(candidates, filepath.Join("data", lower+".json"))
	if strings.HasSuffix(lower, "s") && len(lower) > 1 {
		candidates = append(candidates, filepath.Join("data", strings.TrimSuffix(lower, "s")+".json"))
	}
	candidates = append(candidates, filepath.Join("data", "table", lower+".json"))
	if parts := strings.Split(lower, "_"); len(parts) > 1 {
		suffix := parts[len(parts)-1]
		if suffix != "" {
			candidates = append(candidates, filepath.Join("data", "table", suffix+".json"))
		}
	}
	return uniquePaths(candidates)
}

func schemaPreferredPath(table string) string {
	for _, candidate := range schemaFileCandidates(table) {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	lower := strings.ToLower(strings.TrimSpace(table))
	if lower != "" {
		if path := filepath.Join("data", "table", lower+".json"); path != "" {
			return path
		}
	}
	if strings.HasSuffix(lower, "s") && len(lower) > 1 {
		return filepath.Join("data", strings.TrimSuffix(lower, "s")+".json")
	}
	if parts := strings.Split(lower, "_"); len(parts) > 1 {
		suffix := parts[len(parts)-1]
		if suffix != "" {
			return filepath.Join("data", "table", suffix+".json")
		}
	}
	return filepath.Join("data", lower+".json")
}

func uniquePaths(paths []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(paths))
	for _, path := range paths {
		cleaned := filepath.Clean(path)
		if cleaned == "." || cleaned == "" {
			continue
		}
		if _, ok := seen[cleaned]; ok {
			continue
		}
		seen[cleaned] = struct{}{}
		result = append(result, cleaned)
	}
	return result
}

func cloneSeedRows(rows []map[string]any) []map[string]any {
	if len(rows) == 0 {
		return nil
	}
	cloned := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		if row == nil {
			continue
		}
		copyRow := make(map[string]any, len(row))
		for k, v := range row {
			copyRow[k] = v
		}
		cloned = append(cloned, copyRow)
	}
	if len(cloned) == 0 {
		return nil
	}
	return cloned
}

// RegisterSchema 注册表结构并同步索引信息，可传入额外的索引结构用于描述复合索引。
func RegisterSchema(table string, model any, indexModels ...any) error {
	return registerSchemaWithOptions(table, model, indexModels, nil)
}

// RegisterSchemaWithSeeds 注册表结构并附带默认数据。
func RegisterSchemaWithSeeds(table string, model any, seeds []map[string]any, indexModels ...any) error {
	return registerSchemaWithOptions(table, model, indexModels, seeds)
}

func registerSchemaWithOptions(table string, model any, indexModels []any, seeds []map[string]any) error {
	table = strings.TrimSpace(table)
	if table == "" {
		return errors.New("orm: table name required for registration")
	}
	schema, err := buildSchema(table, model, indexModels, seeds)
	if err != nil {
		return err
	}

	schemaMu.Lock()
	registered[strings.ToLower(table)] = schema
	schemaMu.Unlock()

	if err := persistSchema(schema); err != nil {
		return fmt.Errorf("orm: persist schema for %s failed: %w", table, err)
	}
	return nil
}

func registerSchemaOnce(table string, model any, options schemaOptions) error {
	if model == nil {
		return nil
	}
	lower := strings.ToLower(strings.TrimSpace(table))
	if lower == "" {
		return errors.New("orm: table name required for registration")
	}
	onceIface, _ := schemaOnceMap.LoadOrStore(lower, &sync.Once{})
	once := onceIface.(*sync.Once)
	var onceErr error
	once.Do(func() {
		onceErr = registerSchemaWithOptions(table, model, options.indexes, options.seeds)
	})
	return onceErr
}

type schemaOptions struct {
	indexes []any
	seeds   []map[string]any
}

// RegisterModel 兼容旧接口，仅注册基础结构体信息。
func RegisterModel(table string, model any) error {
	return RegisterSchema(table, model)
}

// MustRegisterModel is RegisterModel but panics on error.
func MustRegisterModel(table string, model any) {
	if err := RegisterModel(table, model); err != nil {
		panic(err)
	}
}

func getRegisteredSchema(table string) (*tableSchema, bool) {
	lower := strings.ToLower(strings.TrimSpace(table))
	if lower == "" {
		return nil, false
	}
	schemaMu.RLock()
	schema, ok := registered[lower]
	schemaMu.RUnlock()
	if ok {
		return schema, true
	}
	loaded, err := loadSchemaForTable(table)
	if err != nil || loaded == nil {
		return nil, false
	}
	schemaMu.Lock()
	registered[lower] = loaded
	schemaMu.Unlock()
	return loaded, true
}

func buildSchema(table string, model any, indexModels []any, seeds []map[string]any) (*tableSchema, error) {
	t := reflect.TypeOf(model)
	if t == nil {
		return nil, fmt.Errorf("orm: model for %s must not be nil", table)
	}
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("orm: model for %s must be struct, got %s", table, t.Kind())
	}

	columns := make([]columnDef, 0, t.NumField())
	indexMap := map[string]*indexDef{}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" {
			continue
		}
		col, indexes, skip, err := parseField(table, field)
		if err != nil {
			return nil, err
		}
		if skip {
			continue
		}
		columns = append(columns, col)
		for _, idx := range indexes {
			existing, ok := indexMap[idx.Name]
			if ok {
				// merge columns for same index name
				existing.Columns = uniqueAppend(existing.Columns, idx.Columns...)
				if idx.Unique {
					existing.Unique = true
				}
				continue
			}
			index := idx
			indexMap[idx.Name] = &index
		}
	}

	// fallback primary key if none defined but we have id column
	hasPrimary := false
	for _, col := range columns {
		if col.Primary {
			hasPrimary = true
			break
		}
	}
	if !hasPrimary {
		for i, col := range columns {
			if strings.EqualFold(col.Name, "id") {
				columns[i].Primary = true
				if strings.Contains(strings.ToUpper(col.Type), "INT") {
					columns[i].AutoIncrement = true
				}
				break
			}
		}
	}

	extraIndexes, err := parseIndexModels(table, columns, indexModels...)
	if err != nil {
		return nil, err
	}
	for _, idx := range extraIndexes {
		existing, ok := indexMap[idx.Name]
		if ok {
			existing.Columns = uniqueAppend(existing.Columns, idx.Columns...)
			existing.Unique = existing.Unique || idx.Unique
			continue
		}
		copyIdx := idx
		indexMap[idx.Name] = &copyIdx
	}

	indexes := make([]indexDef, 0, len(indexMap))
	for _, idx := range indexMap {
		indexes = append(indexes, *idx)
	}

	return &tableSchema{
		Table:     table,
		Columns:   columns,
		Indexes:   indexes,
		Seeds:     cloneSeedRows(seeds),
		UpdatedAt: time.Now(),
	}, nil
}

func parseIndexModels(table string, columns []columnDef, indexModels ...any) ([]indexDef, error) {
	if len(indexModels) == 0 {
		return nil, nil
	}
	colMap := make(map[string]string, len(columns))
	for _, col := range columns {
		colMap[strings.ToLower(col.Name)] = col.Name
	}

	var indexes []indexDef
	for _, model := range indexModels {
		if model == nil {
			continue
		}
		typeVal := reflect.TypeOf(model)
		if typeVal.Kind() == reflect.Pointer {
			typeVal = typeVal.Elem()
		}
		if typeVal.Kind() != reflect.Struct {
			return nil, fmt.Errorf("orm: index model for %s must be struct, got %s", table, typeVal.Kind())
		}
		for i := 0; i < typeVal.NumField(); i++ {
			field := typeVal.Field(i)
			if field.PkgPath != "" { // unexported
				continue
			}
			nameHint := normalizeIndexName(field.Name)
			if nameHint == "" {
				continue
			}
			if cols := parseIndexColumns(field.Tag.Get("index"), colMap); len(cols) > 0 {
				indexes = append(indexes, indexDef{
					Name:    buildCustomIndexName(table, field, false),
					Columns: cols,
				})
			}
			if cols := parseIndexColumns(field.Tag.Get("unique"), colMap); len(cols) > 0 {
				indexes = append(indexes, indexDef{
					Name:    buildCustomIndexName(table, field, true),
					Columns: cols,
					Unique:  true,
				})
			}
		}
	}
	return indexes, nil
}

func parseIndexColumns(tagValue string, columnMap map[string]string) []string {
	raw := strings.TrimSpace(tagValue)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	columns := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, part := range parts {
		candidate := strings.TrimSpace(part)
		if candidate == "" {
			continue
		}
		resolved, ok := resolveIndexColumn(candidate, columnMap)
		if !ok {
			continue
		}
		if _, exists := seen[strings.ToLower(resolved)]; exists {
			continue
		}
		seen[strings.ToLower(resolved)] = struct{}{}
		columns = append(columns, resolved)
	}
	return columns
}

func resolveIndexColumn(raw string, columnMap map[string]string) (string, bool) {
	key := strings.ToLower(strings.TrimSpace(raw))
	if key == "" {
		return "", false
	}
	if val, ok := columnMap[key]; ok {
		return val, true
	}
	snake := strings.ToLower(toSnake(raw))
	if val, ok := columnMap[snake]; ok {
		return val, true
	}
	return "", false
}

func buildCustomIndexName(table string, field reflect.StructField, unique bool) string {
	override := strings.TrimSpace(field.Tag.Get("name"))
	if override != "" {
		if candidate := normalizeIndexName(override); candidate != "" {
			return candidate
		}
	}
	base := normalizeIndexName(field.Name)
	if base == "" {
		base = toSnake(field.Name)
	}
	if base == "" {
		base = "idx"
	}
	prefix := "idx"
	if unique {
		prefix = "uidx"
	}
	return fmt.Sprintf("%s_%s_%s", prefix, toSnake(table), base)
}

func parseField(table string, field reflect.StructField) (columnDef, []indexDef, bool, error) {
	var empty columnDef
	dormTag := field.Tag.Get("dorm")
	columnName := ""
	if dbTag := field.Tag.Get("db"); dbTag != "" && dbTag != "-" {
		columnName = dbTag
	}

	tagOptions := parseDormTag(dormTag)
	if tagExists(tagOptions, "-") {
		return empty, nil, true, nil
	}
	if colName := firstNonEmpty(tagOptions["column"]); colName != "" {
		columnName = colName
	}
	if columnName == "" {
		columnName = toSnake(field.Name)
	}
	if err := ensureIdentifier(columnName); err != nil {
		return empty, nil, false, err
	}

	sqlType, nullable, err := inferSQLType(field, tagOptions)
	if err != nil {
		return empty, nil, false, err
	}

	col := columnDef{
		Name:          columnName,
		Type:          sqlType,
		NotNull:       !nullable,
		AutoIncrement: tagExists(tagOptions, "autoincrement"),
		Primary:       tagExists(tagOptions, "primarykey"),
	}

	if tagExists(tagOptions, "not null") || tagExists(tagOptions, "notnull") {
		col.NotNull = true
	}
	if tagExists(tagOptions, "null") {
		col.NotNull = false
	}

	if defaultVal := firstNonEmpty(tagOptions["default"]); defaultVal != "" {
		defaultStr, raw := normalizeDefaultValue(defaultVal)
		col.DefaultValue = &defaultStr
		col.DefaultIsRaw = raw
	}

	indexes := collectIndexes(table, columnName, tagOptions)
	return col, indexes, false, nil
}

func inferSQLType(field reflect.StructField, options map[string][]string) (string, bool, error) {
	if custom := firstNonEmpty(options["type"]); custom != "" {
		return strings.ToUpper(custom), false, nil
	}

	ft := field.Type
	nullable := false
	if ft.Kind() == reflect.Pointer {
		nullable = true
		ft = ft.Elem()
	}

	switch ft {
	case reflect.TypeOf(time.Time{}):
		return "TIMESTAMP", nullable, nil
	}

	switch ft.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return "BIGINT", nullable, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "BIGINT", nullable, nil
	case reflect.String:
		if size := firstNonEmpty(options["size"]); size != "" {
			return fmt.Sprintf("VARCHAR(%s)", size), nullable, nil
		}
		return "VARCHAR(255)", nullable, nil
	case reflect.Float32, reflect.Float64:
		return "DOUBLE", nullable, nil
	case reflect.Bool:
		return "BOOLEAN", nullable, nil
	case reflect.Struct:
		return "", false, fmt.Errorf("orm: unsupported embedded struct %s for schema", field.Name)
	default:
		return "", false, fmt.Errorf("orm: unsupported field type %s for schema", ft.String())
	}
}

func parseDormTag(tag string) map[string][]string {
	options := map[string][]string{}
	if tag == "" {
		return options
	}
	parts := strings.Split(tag, ";")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		pair := strings.SplitN(part, ":", 2)
		key := strings.TrimSpace(strings.ToLower(pair[0]))
		value := ""
		if len(pair) == 2 {
			value = strings.TrimSpace(pair[1])
		}
		options[key] = append(options[key], value)
	}
	return options
}

func firstNonEmpty(values []string) string {
	for _, val := range values {
		if strings.TrimSpace(val) != "" {
			return strings.TrimSpace(val)
		}
	}
	return ""
}

func tagExists(options map[string][]string, key string) bool {
	_, ok := options[strings.ToLower(key)]
	return ok
}

func collectIndexes(table, column string, options map[string][]string) []indexDef {
	var result []indexDef
	if vals, ok := options["index"]; ok {
		for _, val := range vals {
			name := defaultIndexName(table, column, false)
			if trimmed := strings.TrimSpace(val); trimmed != "" {
				parts := strings.SplitN(trimmed, ",", 2)
				name = normalizeIndexName(parts[0])
			}
			result = append(result, indexDef{
				Name:    name,
				Columns: []string{column},
			})
		}
	}
	if vals, ok := options["uniqueindex"]; ok {
		for _, val := range vals {
			name := defaultIndexName(table, column, true)
			if trimmed := strings.TrimSpace(val); trimmed != "" {
				parts := strings.SplitN(trimmed, ",", 2)
				name = normalizeIndexName(parts[0])
			}
			result = append(result, indexDef{
				Name:    name,
				Columns: []string{column},
				Unique:  true,
			})
		}
	}
	if tagExists(options, "unique") {
		result = append(result, indexDef{
			Name:    defaultIndexName(table, column, true),
			Columns: []string{column},
			Unique:  true,
		})
	}
	return result
}

func defaultIndexName(table, column string, unique bool) string {
	prefix := "idx"
	if unique {
		prefix = "uidx"
	}
	return fmt.Sprintf("%s_%s_%s", prefix, toSnake(table), column)
}

func normalizeIndexName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ReplaceAll(name, "-", "_")
	return name
}

func uniqueAppend(dst []string, items ...string) []string {
	exists := map[string]struct{}{}
	for _, d := range dst {
		exists[d] = struct{}{}
	}
	for _, item := range items {
		if _, ok := exists[item]; ok {
			continue
		}
		dst = append(dst, item)
		exists[item] = struct{}{}
	}
	return dst
}

func normalizeDefaultValue(val string) (string, bool) {
	val = strings.TrimSpace(val)
	if val == "" {
		return val, false
	}
	switch strings.ToUpper(val) {
	case "CURRENT_TIMESTAMP", "CURRENT_DATE", "CURRENT_TIME", "NOW()":
		return strings.ToUpper(val), true
	}
	if _, err := strconv.ParseFloat(val, 64); err == nil {
		return val, true
	}
	if strings.EqualFold(val, "true") || strings.EqualFold(val, "false") {
		return strings.ToLower(val), true
	}
	return val, false
}

func persistSchema(schema *tableSchema) error {
	if schema == nil {
		return nil
	}
	if !schemaPersistenceEnabled() {
		return nil
	}
	path := schemaPreferredPath(schema.Table)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return err
	}
	existing, err := os.ReadFile(path)
	if err == nil {
		// skip write if identical to avoid churn
		if strings.TrimSpace(string(existing)) == strings.TrimSpace(string(data)) {
			return nil
		}
	}
	return os.WriteFile(path, data, 0o644)
}

func loadSchemaForTable(table string) (*tableSchema, error) {
	candidates := schemaFileCandidates(table)
	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		var schema tableSchema
		if err := json.Unmarshal(data, &schema); err != nil {
			return nil, err
		}
		if strings.TrimSpace(schema.Table) == "" {
			schema.Table = table
		}
		return &schema, nil
	}
	return nil, os.ErrNotExist
}

func listRecordedSchemas() ([]*tableSchema, error) {
	var schemas []*tableSchema
	seen := map[string]struct{}{}
	for _, dir := range schemaSearchDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			path := filepath.Join(dir, entry.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, err
			}
			var schema tableSchema
			if err := json.Unmarshal(data, &schema); err != nil {
				return nil, err
			}
			tableName := strings.ToLower(strings.TrimSpace(schema.Table))
			if tableName == "" {
				tableName = strings.TrimSuffix(strings.ToLower(entry.Name()), ".json")
			}
			if _, ok := seen[tableName]; ok {
				continue
			}
			schema.Table = strings.TrimSpace(schema.Table)
			seen[tableName] = struct{}{}
			schemaCopy := schema
			schemas = append(schemas, &schemaCopy)
		}
	}
	sort.Slice(schemas, func(i, j int) bool {
		return schemas[i].Table < schemas[j].Table
	})
	return schemas, nil
}
