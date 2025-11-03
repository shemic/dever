package orm

import "sync/atomic"

var (
	autoMigrate       atomic.Bool
	stableFilters     atomic.Bool
	schemaPersistence atomic.Bool
	migrationLog      atomic.Bool
)

func init() {
	stableFilters.Store(true)
	schemaPersistence.Store(true)
	migrationLog.Store(false)
}

// EnableAutoMigrate 切换全局自动建表和结构更新能力。
func EnableAutoMigrate(enable bool) {
	autoMigrate.Store(enable)
}

func autoMigrateEnabled() bool {
	return autoMigrate.Load()
}

// EnableStableFilters 控制是否对查询条件进行排序，默认开启以保证 deterministic。
func EnableStableFilters(enable bool) {
	stableFilters.Store(enable)
}

func stableFiltersEnabled() bool {
	return stableFilters.Load()
}

// EnableSchemaPersistence 控制是否将表结构写入 JSON 文件，默认开启。
func EnableSchemaPersistence(enable bool) {
	schemaPersistence.Store(enable)
}

func schemaPersistenceEnabled() bool {
	return schemaPersistence.Load()
}

// EnableMigrationLog 控制是否在 data/migrations 下记录 SQL。
func EnableMigrationLog(enable bool) {
	migrationLog.Store(enable)
}

func migrationLogEnabled() bool {
	return migrationLog.Load()
}
