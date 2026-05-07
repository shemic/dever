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
	name         string
	table        string
	primaryKey   string
	dbName       string
	driverName   string
	schema       *tableSchema
	config       ModelConfig
	dbOnce       sync.Once
	dbCached     *sqlx.DB
	dbErr        error
	defaultOrder string
	quoteOnce    sync.Once
	quotedTable  string
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

// newModelCore 创建模型，name/table 是模型的业务名和表名，config 承载 ORM 与业务元信息。
func newModelCore(name, table string, config ModelConfig) (*modelCore, error) {
	name = strings.TrimSpace(name)
	config = normalizeModelConfig(config)
	m := &modelCore{
		name:       name,
		table:      table,
		primaryKey: "id",
		dbName:     currentDefaultDatabase(),
		config:     config,
	}
	if m.name == "" {
		m.name = table
	}
	if config.Database != "" {
		m.dbName = config.Database
	}

	schemaModel := config.schema
	indexModels := config.indexModels()
	seedRows := cloneSeedRows(config.Seeds)
	defaultOrder := strings.TrimSpace(config.Order)

	table = applyTablePrefix(table, m.dbName)
	m.table = table

	options := schemaOptions{
		indexes: indexModels,
		seeds:   seedRows,
	}
	if err := registerSchemaOnce(table, schemaModel, options); err != nil {
		return nil, err
	}

	m.defaultOrder = strings.TrimSpace(defaultOrder)

	schema, ok := getRegisteredSchema(table)
	if !ok {
		return nil, fmt.Errorf("orm: table %s schema not registered, please call orm.RegisterModel", table)
	}
	m.schema = schema
	m.config = m.config.withRuntimeMeta(m.name, m.table, m.dbName, m.defaultOrder, schema.labels())

	if autoMigrateEnabled() {
		if err := m.ensureSchema(context.Background()); err != nil {
			return nil, err
		}
	}

	return m, nil
}

// LoadModel 返回缓存的模型实例，若不存在则创建并缓存。
func LoadModel[T any](name, table string, config ModelConfig) *Model[T] {
	var schema T
	config = config.withSchema(schema)
	core, err := loadModelCore(name, table, config)
	if err != nil {
		panic(err)
	}
	return &Model[T]{modelCore: core}
}

func loadModelCore(name, table string, config ModelConfig) (*modelCore, error) {
	if err := ensureDatabaseInitialized(); err != nil {
		return nil, err
	}

	key := modelCacheKey(table, config)
	modelCacheMu.RLock()
	if cached, ok := modelCache[key]; ok {
		modelCacheMu.RUnlock()
		return cached, nil
	}
	modelCacheMu.RUnlock()

	created, err := newModelCore(name, table, config)
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
