package orm

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/jmoiron/sqlx"
)

// Model 文件导航（同包多文件按职责拆分）：
// - model_query.go: 查询/聚合与 SELECT 语句拼装
// - model_mutation.go: Insert/Update/Delete
// - model_where.go: WHERE 条件解析与参数构建
// - model_normalize.go: 字段名/结构归一化
// - model_convert.go: 值类型转换
// - model_util.go: 执行器与通用辅助函数

// modelCore 提供最小化的 CRUD 能力，并在初始化时支持基于结构体的自动建表/更新。
type modelCore struct {
	table        string
	primaryKey   string
	dbName       string
	driverName   string
	schema       *tableSchema
	dbOnce       sync.Once
	dbCached     *sqlx.DB
	dbErr        error
	defaultOrder string
	versionOnce  sync.Once
	hasVersion   bool
}

// Model 是泛型模型封装，Find/Select 返回结构体指针。
type Model[T any] struct {
	*modelCore
}

var (
	modelCacheMu sync.RWMutex
	modelCache   = map[string]*modelCore{}
)

// EnsureCachedSchemas 尝试为已加载的模型同步表结构，适用于模型早于数据库初始化加载的场景。
func EnsureCachedSchemas(ctx context.Context) error {
	if !autoMigrateEnabled() {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	modelCacheMu.RLock()
	cached := make([]*modelCore, 0, len(modelCache))
	for _, m := range modelCache {
		cached = append(cached, m)
	}
	modelCacheMu.RUnlock()

	for _, m := range cached {
		if m == nil || m.schema == nil {
			continue
		}
		if err := m.ensureSchema(ctx); err != nil {
			return err
		}
	}
	return nil
}

// newModelCore 创建模型，参数格式：
//   - table 必填，表示表名
//   - 可选参数：字符串表示数据库名称
func newModelCore(table string, args ...any) (*modelCore, error) {
	m := &modelCore{
		table:      table,
		primaryKey: "id",
		dbName:     currentDefaultDatabase(),
	}

	var (
		schemaModel  any
		indexModels  []any
		seedRows     []map[string]any
		defaultOrder string
		orderSet     bool
	)

	for _, arg := range args {
		switch v := arg.(type) {
		case string:
			trimmed := strings.TrimSpace(v)
			if trimmed == "" {
				continue
			}
			if schemaModel != nil && !orderSet && looksLikeOrder(trimmed) {
				defaultOrder = trimmed
				orderSet = true
				continue
			}
			m.dbName = trimmed
		case *string:
			if v != nil && strings.TrimSpace(*v) != "" {
				m.dbName = *v
			}
		case []map[string]any:
			if len(v) > 0 {
				seedRows = append(seedRows, cloneSeedRows(v)...)
			}
		case SeedData:
			if len(v.Rows) > 0 {
				seedRows = append(seedRows, cloneSeedRows(v.Rows)...)
			}
		case nil:
			continue
		default:
			if isStructLike(v) {
				if schemaModel == nil {
					schemaModel = v
				} else {
					indexModels = append(indexModels, v)
				}
				continue
			}
			return nil, fmt.Errorf("orm: unsupported argument type %T for NewModel", v)
		}
	}

	table = applyTablePrefix(table, m.dbName)
	m.table = table

	if schemaModel != nil {
		options := schemaOptions{
			indexes: indexModels,
			seeds:   seedRows,
		}
		if err := registerSchemaOnce(table, schemaModel, options); err != nil {
			return nil, err
		}
	}

	m.defaultOrder = strings.TrimSpace(defaultOrder)

	schema, ok := getRegisteredSchema(table)
	if !ok {
		return nil, fmt.Errorf("orm: table %s schema not registered, please call orm.RegisterModel", table)
	}
	m.schema = schema

	if autoMigrateEnabled() {
		if err := m.ensureSchema(context.Background()); err != nil {
			return nil, err
		}
	}

	return m, nil
}

// LoadModel 返回缓存的模型实例，若不存在则创建并缓存。
func LoadModel[T any](table string, args ...any) *Model[T] {
	if !hasSchemaType[T](args) {
		var schema T
		args = append([]any{schema}, args...)
	}
	core, err := loadModelCore(table, args...)
	if err != nil {
		panic(err)
	}
	return &Model[T]{modelCore: core}
}

func loadModelCore(table string, args ...any) (*modelCore, error) {
	if err := ensureDatabaseInitialized(); err != nil {
		return nil, err
	}

	key := modelCacheKey(table, args...)
	modelCacheMu.RLock()
	if cached, ok := modelCache[key]; ok {
		modelCacheMu.RUnlock()
		return cached, nil
	}
	modelCacheMu.RUnlock()

	created, err := newModelCore(table, args...)
	if err != nil {
		return nil, err
	}

	modelCacheMu.Lock()
	defer modelCacheMu.Unlock()
	if cached, ok := modelCache[key]; ok {
		return cached, nil
	}
	modelCache[key] = created
	return created, nil
}
