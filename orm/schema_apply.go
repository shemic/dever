package orm

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

func (m *Model) ensureSchema(ctx context.Context) error {
	if m.schema == nil {
		return fmt.Errorf("orm: schema for table %s not registered", m.table)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	db, err := m.db()
	if err != nil {
		return err
	}
	driver := normalizeDriver(db.DriverName())
	statements, err := syncTableSchema(ctx, db, driver, m.table, m.schema)
	if err != nil {
		return err
	}
	if len(statements) > 0 {
		if err := recordMigrationStatements(m.dbName, m.table, statements); err != nil {
			return err
		}
	}
	return nil
}

func syncTableSchema(ctx context.Context, db *sqlx.DB, driver, table string, schema *tableSchema) ([]string, error) {
	exists, err := tableExists(ctx, db, driver, table)
	if err != nil {
		return nil, err
	}

	var statements []string
	if !exists {
		stmt, err := createTable(ctx, db, driver, table, schema.Columns)
		if err != nil {
			return nil, err
		}
		if stmt != "" {
			statements = append(statements, stmt)
		}
		seedStmt, err := applySeedData(ctx, db, table, schema.Seeds)
		if err != nil {
			return nil, err
		}
		statements = append(statements, seedStmt...)
	} else {
		renameStmt, err := reconcileColumnNames(ctx, db, driver, table, schema.Columns)
		if err != nil {
			return nil, err
		}
		statements = append(statements, renameStmt...)

		addStmt, err := addMissingColumns(ctx, db, driver, table, schema.Columns)
		if err != nil {
			return nil, err
		}
		statements = append(statements, addStmt...)

		dropStmt, err := dropObsoleteColumns(ctx, db, driver, table, schema.Columns)
		if err != nil {
			return nil, err
		}
		statements = append(statements, dropStmt...)
	}

	indexStmt, err := ensureIndexes(ctx, db, driver, table, schema.Indexes)
	if err != nil {
		return nil, err
	}
	statements = append(statements, indexStmt...)

	return uniqueStrings(statements), nil
}

func normalizeDriver(name string) string {
	switch strings.ToLower(name) {
	case "postgres", "postgresql", "pgx":
		return "postgres"
	case "mysql", "mariadb":
		return "mysql"
	case "sqlite", "sqlite3":
		return "sqlite"
	default:
		return strings.ToLower(name)
	}
}

func tableExists(ctx context.Context, db *sqlx.DB, driver, table string) (bool, error) {
	if err := ensureIdentifier(table); err != nil {
		return false, err
	}
	var query string
	switch driver {
	case "postgres":
		query = "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = current_schema() AND table_name = $1"
	case "mysql":
		query = "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?"
	case "sqlite":
		query = "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name = ?"
	default:
		query = "SELECT COUNT(*) FROM information_schema.tables WHERE table_name = ?"
	}
	query = db.Rebind(query)
	var count int
	if err := db.GetContext(ctx, &count, query, table); err != nil {
		return false, err
	}
	return count > 0, nil
}

func createTable(ctx context.Context, db *sqlx.DB, driver, table string, columns []columnDef) (string, error) {
	if err := ensureIdentifier(table); err != nil {
		return "", err
	}
	defs := make([]string, 0, len(columns))
	var pk string
	for _, col := range columns {
		columnType := sqlTypeForDriver(col.Type, driver, col.AutoIncrement)
		parts := []string{quoteIdentifier(driver, col.Name), columnType}

		switch driver {
		case "mysql":
			if col.AutoIncrement {
				parts = append(parts, "AUTO_INCREMENT")
			}
		case "sqlite":
			if col.Primary {
				parts = append(parts, "PRIMARY KEY")
				if col.AutoIncrement {
					parts = append(parts, "AUTOINCREMENT")
				}
			}
		}

		if col.NotNull && !(driver == "sqlite" && col.Primary) {
			parts = append(parts, "NOT NULL")
		}
		if col.DefaultValue != nil {
			parts = append(parts, "DEFAULT "+formatDefaultValue(*col.DefaultValue, col.DefaultIsRaw))
		}

		if col.Primary && driver != "sqlite" {
			pk = col.Name
		}
		defs = append(defs, strings.Join(parts, " "))
	}
	if pk != "" {
		defs = append(defs, fmt.Sprintf("PRIMARY KEY (%s)", quoteIdentifier(driver, pk)))
	}
	statement := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (%s)", quoteIdentifier(driver, table), strings.Join(defs, ", "))
	if _, err := db.ExecContext(ctx, statement); err != nil {
		return "", err
	}
	return statement, nil
}

func addMissingColumns(ctx context.Context, db *sqlx.DB, driver, table string, columns []columnDef) ([]string, error) {
	existing, err := loadExistingColumns(ctx, db, driver, table)
	if err != nil {
		return nil, err
	}
	var statements []string
	for _, column := range columns {
		if columnExists(existing, column.Name) {
			continue
		}
		definition := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", quoteIdentifier(driver, table), quoteIdentifier(driver, column.Name), sqlTypeForDriver(column.Type, driver, column.AutoIncrement))
		if column.NotNull {
			definition += " NOT NULL"
		}
		if column.DefaultValue != nil {
			definition += " DEFAULT " + formatDefaultValue(*column.DefaultValue, column.DefaultIsRaw)
		}
		if driver == "mysql" && column.AutoIncrement {
			definition += " AUTO_INCREMENT"
		}
		if _, err := db.ExecContext(ctx, definition); err != nil {
			return nil, err
		}
		statements = append(statements, definition)
	}
	return statements, nil
}

func dropObsoleteColumns(ctx context.Context, db *sqlx.DB, driver, table string, columns []columnDef) ([]string, error) {
	existing, err := loadExistingColumns(ctx, db, driver, table)
	if err != nil {
		return nil, err
	}
	desired := make(map[string]struct{}, len(columns)*2)
	for _, column := range columns {
		lower := strings.ToLower(column.Name)
		desired[lower] = struct{}{}
		desired[canonicalColumnKey(lower)] = struct{}{}
	}
	var statements []string
	for name := range existing {
		if _, ok := desired[name]; ok {
			continue
		}
		if _, ok := desired[canonicalColumnKey(name)]; ok {
			continue
		}
		colName := existing[name]
		stmt := buildDropColumnStatement(driver, table, colName)
		if stmt == "" {
			continue
		}
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return nil, err
		}
		statements = append(statements, stmt)
	}
	return statements, nil
}

func buildDropColumnStatement(driver, table, column string) string {
	switch driver {
	case "postgres":
		return fmt.Sprintf("ALTER TABLE %s DROP COLUMN IF EXISTS %s", quoteIdentifier(driver, table), quoteIdentifier(driver, column))
	case "mysql":
		return fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", quoteIdentifier(driver, table), quoteIdentifier(driver, column))
	case "sqlite":
		return fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", quoteIdentifier(driver, table), quoteIdentifier(driver, column))
	default:
		return ""
	}
}

type indexState struct {
	Name    string
	Columns []string
	Unique  bool
}

func reconcileColumnNames(ctx context.Context, db *sqlx.DB, driver, table string, columns []columnDef) ([]string, error) {
	existing, err := loadExistingColumns(ctx, db, driver, table)
	if err != nil {
		return nil, err
	}
	var statements []string
	for _, column := range columns {
		actual, ok := findExistingColumn(existing, column.Name)
		if !ok {
			continue
		}
		if strings.EqualFold(actual, column.Name) {
			continue
		}
		stmt := buildRenameColumnStatement(driver, table, actual, column.Name)
		if stmt == "" {
			continue
		}
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return nil, err
		}
		statements = append(statements, stmt)
	}
	return statements, nil
}

func buildRenameColumnStatement(driver, table, oldName, newName string) string {
	if strings.EqualFold(oldName, newName) {
		return ""
	}
	switch driver {
	case "postgres", "sqlite":
		return fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s", quoteIdentifier(driver, table), quoteIdentifier(driver, oldName), quoteIdentifier(driver, newName))
	case "mysql":
		return "" // MySQL 8 以下不支持 RENAME COLUMN，保持兼容
	default:
		return ""
	}
}

func applySeedData(ctx context.Context, db *sqlx.DB, table string, seeds []map[string]any) ([]string, error) {
	if len(seeds) == 0 {
		return nil, nil
	}
	statements := make([]string, 0, len(seeds))
	for _, row := range seeds {
		if len(row) == 0 {
			continue
		}
		query, payload, err := buildInsertQuery(table, row)
		if err != nil {
			return nil, err
		}
		if _, err := db.NamedExecContext(ctx, query, payload); err != nil {
			return nil, err
		}
		statements = append(statements, query)
	}
	return statements, nil
}

func ensureIndexes(ctx context.Context, db *sqlx.DB, driver, table string, indexes []indexDef) ([]string, error) {
	existing, err := loadExistingIndexes(ctx, db, driver, table)
	if err != nil {
		return nil, err
	}

	desired := map[string]indexDef{}
	for _, index := range indexes {
		if len(index.Columns) == 0 {
			continue
		}
		indexName := normalizeIndexName(index.Name)
		if indexName == "" {
			indexName = defaultIndexName(table, index.Columns[0], index.Unique)
		} else if err := ensureIdentifier(indexName); err != nil {
			indexName = defaultIndexName(table, index.Columns[0], index.Unique)
		}
		normalizedCols := normalizeIndexColumns(index.Columns)
		if len(normalizedCols) == 0 {
			continue
		}
		desired[strings.ToLower(indexName)] = indexDef{
			Name:    indexName,
			Columns: normalizedCols,
			Unique:  index.Unique,
		}
	}

	var statements []string
	for key, info := range existing {
		if _, ok := desired[key]; ok {
			continue
		}
		drop := buildDropIndexStatement(driver, table, info.Name)
		if drop == "" {
			continue
		}
		if _, err := db.ExecContext(ctx, drop); err != nil {
			return nil, err
		}
		statements = append(statements, drop)
	}

	for key, target := range desired {
		existingInfo, ok := existing[key]
		if ok && sameIndex(existingInfo, target) {
			continue
		}
		if ok {
			drop := buildDropIndexStatement(driver, table, existingInfo.Name)
			if drop != "" {
				if _, err := db.ExecContext(ctx, drop); err != nil {
					return nil, err
				}
				statements = append(statements, drop)
			}
		}
		columnParts := make([]string, 0, len(target.Columns))
		for _, col := range target.Columns {
			columnParts = append(columnParts, quoteIdentifier(driver, col))
		}
		stmt := buildCreateIndexStatement(driver, target.Name, table, columnParts, target.Unique)
		if stmt == "" {
			continue
		}
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return nil, err
		}
		statements = append(statements, stmt)
	}

	return statements, nil
}

func normalizeIndexColumns(columns []string) []string {
	if len(columns) == 0 {
		return nil
	}
	result := make([]string, 0, len(columns))
	seen := map[string]struct{}{}
	for _, col := range columns {
		trimmed := strings.TrimSpace(col)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func sameIndex(existing indexState, desired indexDef) bool {
	if existing.Unique != desired.Unique {
		return false
	}
	if len(existing.Columns) != len(desired.Columns) {
		return false
	}
	for i := range existing.Columns {
		if !strings.EqualFold(existing.Columns[i], desired.Columns[i]) {
			return false
		}
	}
	return true
}

func buildCreateIndexStatement(driver, indexName, table string, columns []string, unique bool) string {
	columnList := strings.Join(columns, ", ")
	switch driver {
	case "postgres", "sqlite":
		if unique {
			return fmt.Sprintf("CREATE UNIQUE INDEX IF NOT EXISTS %s ON %s (%s)", quoteIdentifier(driver, indexName), quoteIdentifier(driver, table), columnList)
		}
		return fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s ON %s (%s)", quoteIdentifier(driver, indexName), quoteIdentifier(driver, table), columnList)
	case "mysql":
		if unique {
			return fmt.Sprintf("CREATE UNIQUE INDEX %s ON %s (%s)", quoteIdentifier(driver, indexName), quoteIdentifier(driver, table), columnList)
		}
		return fmt.Sprintf("CREATE INDEX %s ON %s (%s)", quoteIdentifier(driver, indexName), quoteIdentifier(driver, table), columnList)
	default:
		return ""
	}
}

func buildDropIndexStatement(driver, table, indexName string) string {
	if strings.TrimSpace(indexName) == "" {
		return ""
	}
	switch driver {
	case "postgres":
		return fmt.Sprintf("DROP INDEX IF EXISTS %s", quoteIdentifier(driver, indexName))
	case "mysql":
		return fmt.Sprintf("DROP INDEX %s ON %s", quoteIdentifier(driver, indexName), quoteIdentifier(driver, table))
	case "sqlite":
		return fmt.Sprintf("DROP INDEX IF EXISTS %s", quoteIdentifier(driver, indexName))
	default:
		return ""
	}
}

func loadExistingColumns(ctx context.Context, db *sqlx.DB, driver, table string) (map[string]string, error) {
	if err := ensureIdentifier(table); err != nil {
		return nil, err
	}
	result := map[string]string{}
	switch driver {
	case "postgres":
		query := "SELECT column_name FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = $1"
		rows, err := db.QueryxContext(ctx, query, table)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				return nil, err
			}
			result[strings.ToLower(name)] = name
		}
	case "mysql":
		query := "SELECT column_name FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = ?"
		rows, err := db.QueryxContext(ctx, query, table)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				return nil, err
			}
			result[strings.ToLower(name)] = name
		}
	case "sqlite":
		query := fmt.Sprintf("PRAGMA table_info(%s)", quoteIdentifier(driver, table))
		rows, err := db.QueryxContext(ctx, query)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var cid int
			var name, ctype string
			var notnull, pk int
			var dflt sql.NullString
			if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
				return nil, err
			}
			result[strings.ToLower(name)] = name
		}
	default:
		return nil, fmt.Errorf("orm: unsupported driver %s", driver)
	}
	return result, nil
}

func columnExists(existing map[string]string, name string) bool {
	if existing == nil {
		return false
	}
	targetLower := strings.ToLower(name)
	targetCanonical := canonicalColumnKey(targetLower)
	for key := range existing {
		if key == targetLower {
			return true
		}
		if canonicalColumnKey(key) == targetCanonical {
			return true
		}
	}
	return false
}

func findExistingColumn(existing map[string]string, name string) (string, bool) {
	if existing == nil {
		return "", false
	}
	targetLower := strings.ToLower(name)
	if actual, ok := existing[targetLower]; ok {
		return actual, true
	}
	targetCanonical := canonicalColumnKey(targetLower)
	for key, actual := range existing {
		if canonicalColumnKey(key) == targetCanonical {
			return actual, true
		}
	}
	return "", false
}

func canonicalColumnKey(name string) string {
	return strings.ReplaceAll(strings.ToLower(name), "_", "")
}

func loadExistingIndexes(ctx context.Context, db *sqlx.DB, driver, table string) (map[string]indexState, error) {
	result := map[string]indexState{}
	switch driver {
	case "postgres":
		query := "SELECT indexname, indexdef FROM pg_indexes WHERE schemaname = current_schema() AND tablename = $1"
		rows, err := db.QueryxContext(ctx, query, table)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var name, def string
			if err := rows.Scan(&name, &def); err != nil {
				return nil, err
			}
			if strings.EqualFold(name, "primary") || strings.HasSuffix(strings.ToLower(name), "_pkey") {
				continue
			}
			cols, unique := parsePostgresIndexDef(def)
			key := strings.ToLower(name)
			result[key] = indexState{Name: name, Columns: cols, Unique: unique}
		}
	case "mysql":
		query := "SELECT index_name, non_unique, column_name, seq_in_index FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = ? ORDER BY index_name, seq_in_index"
		rows, err := db.QueryxContext(ctx, query, table)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var name, column string
			var nonUnique int
			var seq int
			if err := rows.Scan(&name, &nonUnique, &column, &seq); err != nil {
				return nil, err
			}
			if strings.EqualFold(name, "primary") || strings.TrimSpace(name) == "" {
				continue
			}
			key := strings.ToLower(name)
			state := result[key]
			state.Name = name
			state.Unique = nonUnique == 0
			state.Columns = append(state.Columns, column)
			result[key] = state
		}
	case "sqlite":
		query := fmt.Sprintf("PRAGMA index_list(%s)", quoteIdentifier(driver, table))
		rows, err := db.QueryxContext(ctx, query)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		type indexMeta struct {
			name   string
			unique bool
		}
		meta := map[string]indexMeta{}
		for rows.Next() {
			var seq, unique int
			var name, origin string
			var partial int
			if err := rows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
				return nil, err
			}
			if origin == "pk" {
				continue
			}
			meta[name] = indexMeta{name: name, unique: unique == 1}
		}
		for name, info := range meta {
			cols, err := loadSQLiteIndexColumns(ctx, db, name)
			if err != nil {
				return nil, err
			}
			key := strings.ToLower(name)
			result[key] = indexState{Name: name, Columns: cols, Unique: info.unique}
		}
	default:
		return nil, fmt.Errorf("orm: unsupported driver %s", driver)
	}
	return result, nil
}

func parsePostgresIndexDef(definition string) ([]string, bool) {
	upper := strings.ToUpper(definition)
	unique := strings.Contains(upper, "UNIQUE")
	start := strings.Index(definition, "(")
	end := strings.LastIndex(definition, ")")
	if start == -1 || end == -1 || end <= start {
		return nil, unique
	}
	segment := definition[start+1 : end]
	parts := strings.Split(segment, ",")
	columns := make([]string, 0, len(parts))
	for _, part := range parts {
		piece := strings.TrimSpace(part)
		if piece == "" {
			continue
		}
		fields := strings.Fields(piece)
		if len(fields) == 0 {
			continue
		}
		name := strings.Trim(fields[0], `"`)
		columns = append(columns, name)
	}
	return columns, unique
}

func loadSQLiteIndexColumns(ctx context.Context, db *sqlx.DB, indexName string) ([]string, error) {
	query := fmt.Sprintf("PRAGMA index_info(%s)", quoteIdentifier("sqlite", indexName))
	rows, err := db.QueryxContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns := []string{}
	for rows.Next() {
		var seqno, cid int
		var name string
		if err := rows.Scan(&seqno, &cid, &name); err != nil {
			return nil, err
		}
		columns = append(columns, name)
	}
	return columns, nil
}

func sqlTypeForDriver(baseType, driver string, autoIncrement bool) string {
	base := strings.ToUpper(baseType)
	switch driver {
	case "postgres":
		if autoIncrement {
			return "BIGINT GENERATED BY DEFAULT AS IDENTITY"
		}
		switch base {
		case "TIMESTAMP":
			return "TIMESTAMPTZ"
		case "BOOLEAN":
			return "BOOLEAN"
		default:
			return base
		}
	case "mysql":
		switch base {
		case "BOOLEAN":
			return "TINYINT(1)"
		case "TIMESTAMP":
			return "TIMESTAMP"
		default:
			return base
		}
	case "sqlite":
		switch base {
		case "TIMESTAMP":
			return "DATETIME"
		case "BOOLEAN":
			return "INTEGER"
		case "DOUBLE":
			return "REAL"
		case "VARCHAR(255)":
			return "TEXT"
		case "BIGINT":
			return "INTEGER"
		default:
			return base
		}
	default:
		return base
	}
}

func formatDefaultValue(value string, raw bool) string {
	if raw {
		return value
	}
	escaped := strings.ReplaceAll(value, "'", "''")
	return "'" + escaped + "'"
}

func recordMigrationStatements(dbName, table string, statements []string) error {
	if len(statements) == 0 {
		return nil
	}
	if !migrationLogEnabled() {
		return nil
	}
	dir := filepath.Join("data", "migrations", dbName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	filename := fmt.Sprintf("%s_%s.sql", time.Now().Format("20060102150405"), table)
	path := filepath.Join(dir, filename)

	content := strings.Builder{}
	content.WriteString("-- Generated by dever ORM auto-migrate\n")
	content.WriteString(fmt.Sprintf("-- Table: %s, Database: %s\n\n", table, dbName))
	for i, stmt := range statements {
		content.WriteString(stmt)
		if !strings.HasSuffix(stmt, ";") {
			content.WriteString(";")
		}
		if i < len(statements)-1 {
			content.WriteString("\n")
		}
	}
	content.WriteString("\n")
	return os.WriteFile(path, []byte(content.String()), 0o644)
}

func quoteIdentifier(driver, ident string) string {
	switch driver {
	case "postgres":
		return `"` + strings.ReplaceAll(ident, `"`, `""`) + `"`
	case "mysql":
		return "`" + strings.ReplaceAll(ident, "`", "``") + "`"
	case "sqlite":
		return `"` + strings.ReplaceAll(ident, `"`, `""`) + `"`
	default:
		return ident
	}
}

func uniqueStrings(items []string) []string {
	if len(items) == 0 {
		return items
	}
	seen := make(map[string]struct{}, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}

// ApplyRecordedSchemas 读取 data/table 下记录的表结构并应用到指定数据库。
func ApplyRecordedSchemas(ctx context.Context, dbName string) ([]string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	db, err := Get(dbName)
	if err != nil {
		return nil, err
	}
	driver := normalizeDriver(db.DriverName())
	schemas, err := listRecordedSchemas()
	if err != nil {
		return nil, err
	}
	if len(schemas) == 0 {
		return nil, nil
	}
	var statements []string
	for _, schema := range schemas {
		if schema == nil {
			continue
		}
		ops, err := syncTableSchema(ctx, db, driver, schema.Table, schema)
		if err != nil {
			return nil, err
		}
		statements = append(statements, ops...)
	}
	return uniqueStrings(statements), nil
}
