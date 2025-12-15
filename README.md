# Dever 框架开发指南

本指南面向第一次接触 Dever 的开发者。文档按照 `dever/` 目录的核心功能组织，帮助你理解配置、命令行、服务器适配、ORM、锁与模块开发流程。

## 1. 功能概览
| 目录 | 作用 |
| --- | --- |
| `cmd/` | 提供 `run`、`routes`、`init`、`migrate` 等命令，负责启动 Fiber 服务、自动生成路由、整理依赖。|
| `config/` | 解析 `config/setting.json`，支持多数据库连接、日志/HTTP/Redis 默认值。|
| `server/` | 抽象 HTTP Server，提供统一 `Context`、JSON 适配和 Fiber 实现 (`server/http`)。|
| `middleware/` | 全局与路由级中间件注册，默认组合 Recover + Log。|
| `load/` | 服务与模型注册中心，通过 `load.Service("<provider 名>")` 调用自动注册的 Provider，或用 `load.Model("user")` 缓存模型。|
| `orm/` | 结构体驱动的 ORM，内建自动迁移、CRUD、复杂过滤、事务、悲观/乐观锁。|
| `lock/` | Redis 原子扣减组件，提供 `Dec/Inc` 与 `WithFloor/WithCeiling/WithTTL` 配置。|
| `log/` | 访问与错误日志配置，`dlog.Configure` 统一初始化。|
| `util/` | Map 与字符串等轻量工具函数。|

## 2. 环境准备与快速启动
1. 安装 Go 1.25+，在项目根目录执行 `go mod download`。
2. 确认 `config/setting.json` 已配置数据库/Redis。默认结构参见第 3 节。
3. 启动服务：
   ```sh
   go run .
   ```
   主函数会调用 `dever/cmd.Run(data.RegisterRoutes)`，注册 `data/router.go` 中自动生成的路由，并装配日志、中间件、锁配置。
4. 停止服务可使用 `Ctrl+C`，框架会等待 `shutdownTimeout` 后优雅退出。

## 3. 配置（`dever/config`）
统一配置文件为 `config/setting.json`：

```json
{
  "log": {
    "level": "info",
    "encoding": "console",
    "development": false,
    "output": "file",
    "successFile": "data/log/access.log",
    "errorFile": "data/log/error.log"
  },
  "http": {
    "host": "0.0.0.0",
    "port": 8081,
    "shutdownTimeout": "10s",
    "appName": "Dever Demo",
    "enableTuning": true,
    "prefork": false,
    "serverHeader": "work"
  },
  "database": {
    "create": true,
    "default": {
      "driver": "mysql",
      "host": "127.0.0.1:3310",
      "user": "root",
      "pwd": "123456",
      "dbname": "ydb",
      "maxOpenConns": 20,
      "maxIdleConns": 10,
      "connMaxLifetime": "300s"
    }
  },
  "redis": {
    "enable": false,
    "addr": "127.0.0.1:6379",
    "prefix": "work:"
  }
}
```

- `log` 支持按访问/错误拆分输出，`output="stdout|stderr|file|off"`；若目录不存在将自动创建。
- `http` 配置会传入 Fiber，`enableTuning=false` 时使用最小化配置以保持 KISS。
- `database` 可写多个连接：`"connections": {"default": {...}, "reporting": {...}}`，`create=true` 打开自动迁移；`persist`、`migrationLog` 控制表结构持久化。
- `redis` 只在需要 `lock` 时启用，遵循 YAGNI；`prefix` 会自动拼接在所有键前。

## 4. 命令行工具（`dever/cmd`）
| 命令 | 说明 |
| --- | --- |
| `go run ./dever/cmd/dever run` | 启动框架示例，与 `go run .` 类似但仅运行脚手架。|
| `go run ./dever/cmd/dever routes` | 扫描 `module/*/api/*.go`，根据函数名生成 `data/router.go`。所有 API 修改后必须执行。|
| `go run ./dever/cmd/dever service` | 扫描 `module/*/service/*.go`，提取 `Provider*` 方法并写入 `data/load/service.go`。|
| `go run ./dever/cmd/dever init` | 合并 routes、service、model 等生成与依赖整理流程。|
| `go run ./dever/cmd/dever migrate` | 执行模型迁移（依赖于 ORM 注册的 schema）。|

`cmd/run.go` 会：
1. 调用 `config.Load` 读取配置；
2. 通过 `lock.Configure` 记录 Redis 设置；
3. `dlog.Configure` 初始化访问/错误日志；
4. 使用 `server/http` 创建 Fiber 应用，注册路由后启动；
5. 阻塞等待信号并优雅关闭。

## 5. 服务器与中间件
- `server.Server` 定义统一接口，当前实现位于 `server/http`，对 Fiber 进行封装。
- `server.Context` 暴露 `Input/BindJSON/JSON/Error`：
  ```go
  func GetUser(c *server.Context) error {
      id := c.Input("id", "is_number", "用户ID")
      return c.JSON(service.GetUser(c.Context(), map[string]any{"id": id}))
  }
  ```
- `middleware.Init()` 组合 Recover + Log，可在 `main.go` 或模块初始化中执行 `middleware.UseGlobal(middleware.Init())`。
- 自定义中间件：
  ```go
  middleware.UseRouteFunc("POST", "/user/test/add", func(ctx any) error {
      // 访问 *server.Context
      return nil
  })
  ```

## 6. 模块化开发流程
模块统一存放在 `module/<name>`，包含 `api/`、`model/`、`service/`、`world/` 等子目录。

### 6.1 Model（`dever/orm`）
```go
package model

type User struct {
    ID    uint   `dorm:"primaryKey;autoIncrement"`
    Name  string `dorm:"size:64;not null"`
    Email string `dorm:"size:128;unique"`
}

type UserIndex struct {
    Email struct{} `unique:"email"`
}

var UserDefault = []map[string]any{
    {"name": "admin", "email": "admin@example.com"},
}

func NewUserModel() *orm.Model {
    return orm.MustLoadModel("user", User{}, UserIndex{}, UserDefault, "id desc", "default")
}
```
- `MustLoadModel` 会缓存模型，必要时自动建表；
- `UserDefault` 仅在首次建表时写入；
- `orm.EnsureCachedSchemas` 可在初始化阶段同步所有模型结构。

### 6.2 Service 与 Provider 注册
```go
package service

func GetInfo(ctx context.Context, params map[string]any) any {
    userModel := model.NewUserModel()
    filters := map[string]any{"main.id": params["id"]}
    return userModel.Find(ctx, filters, map[string]any{"field": "main.id, main.name"})
}
```
- Service 签名惯例：`func Xxx(ctx context.Context, params map[string]any) any/error`；
- 可通过 `orm.Transaction`、`Select(..., true)`（悲观锁）、`Update(..., true)`（乐观锁）处理复杂场景；
- 若需要将 Service 暴露给 World/其他模块，定义结构体方法并以 `Provider` 为前缀，例如 `func (User) ProviderGet(ctx *server.Context, params []any) any`。执行 `go run ./dever/cmd/dever service` 后，方法会写入 `data/load/service.go` 并注册为 `module.subpath.Struct.Method`（上例即 `user.User.Get`），随后可通过 `load.Service("user.User.Get", ctx, ... )` 或 World 的 `provider` 字段引用。

### 6.3 API
```go
package api

func GetUser(c *server.Context) error {
    id := c.Input("id", "is_number", "用户ID")
    result := service.GetInfo(c.Context(), map[string]any{"id": id})
    return c.JSON(map[string]any{"data": result})
}
```
- API 函数名与文件名决定路由：`module/user/api/test.go` + `GetUser` → `GET /user/test/get_user`；
- 修改或新增 API 后运行 `go run ./dever/cmd/dever routes` 更新 `data/router.go`。

### 6.4 World JSON
1. 在 `module/<name>/world/<path>.json` 维护 `page/layout/data/flows`；
2. `data` 的 `type` 可为 `service/model/static`，`use` 填 Provider 字符串（如 `user.User.Get`，与 `load.Service` 一致）；
3. 使用 `/world/main/get`、`/world/main/run` + `locator` 调试。

## 7. 数据访问与事务
- **查询**：`Select` 返回切片、`Find` 返回单条；`options` 支持 `page`、`pageSize`、`field`、`order`、`join`；
- **增删改**：`Insert` / `Update` / `Delete` 入参均为 `map[string]any`，方便接受 `c.Input` 解析后的数据；
- **事务**：
  ```go
  err := orm.Transaction(ctx, func(txCtx context.Context) error {
      userModel := model.NewUserModel()
      user := userModel.Find(txCtx, map[string]any{"id": id}, nil)
      if len(user) == 0 {
          return fmt.Errorf("用户不存在")
      }
      _, err := userModel.Update(txCtx, map[string]any{"id": id}, map[string]any{"status": 1})
      return err
  })
  ```
- **锁策略**：
  - 乐观锁：`userModel.Update(ctx, filters, data, true)`，冲突时返回 `orm.ErrVersionConflict`；
  - 悲观锁：`userModel.Select(ctx, filters, options, true)` 自动追加 `FOR UPDATE`。

## 8. Redis 原子扣减（`dever/lock`）
```go
remaining, err := lock.Dec(ctx, "quota:user:1", 1, lock.WithFloor(0))
if errors.Is(err, lock.ErrInsufficient) {
    return c.Error("额度不足")
}

defer func(success *bool) {
    if !*success {
        _, _ = lock.Inc(ctx, "quota:user:1", 1)
    }
}(&success)
```
- `lock.Configure` 在 `cmd.Run` 启动时调用；
- `WithCeiling` 用于充值或防止超发；
- `WithTTL` 支持在操作成功后设置过期时间（毫秒精度）。

## 9. 日志与监控
- `dlog.Configure(cfg.Log)` 会启动访问日志（Access）和错误日志（Error），输出路径由 `successFile`、`errorFile` 决定；
- `middleware.Log` 自动记录 `method/path/duration/err`；若需要接入自定义监控，可在中间件中调用统计组件。

## 10. 验证与发布
- **路由**：新增 API 后必须重新生成路由并确认 `data/router.go` 已更新。
- **测试**：`go test ./module/<name>/...`、`go test ./dever/...`（如实现了对应测试）。
- **运行**：`go run .` 验证 HTTP 服务及配置加载；
- **配置/迁移**：若修改 `config/setting.json` 或模型，需要同步说明部署环境的差异、数据库迁移方式、是否要求 `go run ./dever/cmd/dever migrate`。
- **风险提示**：
  - 未限制分页/排序时应记录潜在性能风险；
  - 缺少鉴权时需在 PR 描述或 README 中标注；
  - Redis/数据库连接信息属敏感数据，避免硬编码。

掌握以上模块化功能，即可在 `module/*` 中快速实现业务逻辑，并通过 `dever` 脚手架完成配置、启动、验证与发布。
