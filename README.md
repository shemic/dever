# Dever 框架开发指南

本指南面向第一次接触 Dever 的开发者，结合项目根目录下的示例代码，说明如何配置应用、划分模块，以及在模块内编写 API、Model、Service 并完成常见的增删改查。

## 环境准备与启动

- 安装 Go 1.25 及以上版本，并在项目根目录执行 `go mod download` 拉取依赖。
- 启动服务时直接运行：

```sh
go run .
```

主函数会调用 `data.RegisterRoutes`，加载自动生成的路由并启动 Fiber 服务器。终端可以通过 `Ctrl+C` 触发优雅停机。

## 在 config 目录编写配置

统一配置文件位于 `config/setting.json`，包含日志、HTTP 服务与数据库等模块化配置。下面示例节选了默认内容，并说明各字段含义：

```json
{
  "log": {
    "level": "info",            // 支持 debug/info/warn/error
    "encoding": "console",      // console 或 json
    "development": false        // 开发模式输出更详细的 caller
  },
  "http": {
    "host": "127.0.0.1",        // 监听地址，0.0.0.0 对外开放
    "port": 8081,               // 监听端口，0 表示随机端口
    "shutdownTimeout": "10s",   // 优雅停机等待时长
    "appName": "测试应用",        // 启动横幅显示名称
    "prefork": false,           // 是否启用多进程
    "enableTuning": true,       // 是否开启 Fiber 性能调优
    "serverHeader": "myproject" // HTTP Server 响应头
  },
  "database": {
    "create": true,             // 启动时自动根据模型建表/迁移
    "default": {
      "driver": "mysql",
      "host": "127.0.0.1:3310",
      "user": "root",
      "pwd": "123456",
      "dbname": "ydb",
      "maxOpenConns": 20,
      "maxIdleConns": 10,
      "connMaxLifetime": "300s",
      "connMaxIdleTime": "120s",
      "healthCheckPeriod": "5s"
    }
  }
}
```

建议做法：
- 不同环境可通过覆盖 `config/` 下的 JSON 文件实现差异化配置。
- 开发环境保留 `create=true` 以便自动迁移；生产环境通常关闭自动建表，但可保留 `persist`、`migrationLog` 等选项，将结构持久化后手动执行。

## 新建业务模块（以 user 为例）

模块统一放在 `module/<name>`，至少包含 `api/`、`model/`、`service/` 三层。示例结构：

```
module/
  user/
    api/
      test.go
    model/
      user.go
      profile.go
    service/
      test.go
```

### 编写 Model：描述表结构与默认数据

在 `module/user/model/user.go` 中使用 dorm 风格标签定义字段、索引与默认数据，然后通过 `orm.MustLoadModel` 注册模型：

```go
package model

import (
	"time"

	"github.com/shemic/dever/orm"
)

type User struct {
	ID        uint      `dorm:"primaryKey;autoIncrement"`
	Name      string    `dorm:"size:64;not null"`
	Email     string    `dorm:"size:128;not null"`
	Age       int       `dorm:"default:18"`
	Status    uint8     `dorm:"type:tinyint(1);default:1"`
	CreatedAt time.Time `dorm:"type:timestamp"`
	UpdatedAt time.Time `dorm:"type:timestamp"`
}

type UserIndex struct {
	Name   struct{} `index:"name"`
	Email  struct{} `unique:"email"`
}

var UserDefault = []map[string]any{
	{"name": "admin", "email": "admin@example.com", "age": 30, "status": 1},
}

func NewUserModel() *orm.Model {
	return orm.MustLoadModel("user", User{}, UserIndex{}, UserDefault, "id desc", "default")
}
```

- 结构体字段通过 `dorm` 标签声明主键、默认值、数据类型等约束。
- `UserIndex` 描述普通索引与唯一索引，字段值为空结构体即可。
- `UserDefault` 在迁移时写入初始数据。

### 编写 Service：组织业务逻辑并调用模型

服务层接收 `context.Context` 与业务参数，将 ORM 操作封装为接口。示例 `module/user/service/test.go`：

```go
package user

import (
	"context"

	"myproject/module/user/model"
)

func GetInfo(ctx context.Context, params map[string]any) any {
	userModel := model.NewUserModel()   // 模型仅初始化一次并缓存
	model.NewProfileModel()             // 若需要关联查询，提前加载其他模型

	filters := map[string]any{
		"main.id": params["id"],
	}
	list := userModel.Select(ctx, filters, map[string]any{
		"page":     1,
		"pageSize": 10,
		"field":    "main.id, main.name",
	})

	user := userModel.Find(ctx,
		map[string]any{"main.id": params["id"]},
		map[string]any{"field": "main.id, main.name"},
	)

	// // 增删改示例
	// userModel.Insert(ctx, map[string]any{"name": "demo"})
	// userModel.Update(ctx, map[string]any{"id": params["id"]}, map[string]any{"name": "demo"})
	// userModel.Delete(ctx, map[string]any{"id": params["id"]})

	return map[string]any{
		"list": list,
		"user": user,
	}
}
```

常用方法：
- `Select(ctx, filters, options)`：分页或多条件查询，`filters` 支持 `map`、`[]map` 组合 AND/OR。
- `Find(ctx, filters, options)`：返回单条数据。
- `Insert`、`Update`、`Delete`：增删改操作均接受 `map[string]any` 形式的参数。
- `options` 可设置 `field`、`order`、`page`、`pageSize`、`join` 等。

### 编写 API：绑定路由并连接上下游

API 层位于 `module/user/api/`，函数签名统一为 `(c *server.Context) error`。示例 `GetUser`、`PostUser`：

```go
package user

import (
	userservice "myproject/module/user/service"

	"github.com/shemic/dever/server"
)

func GetUser(c *server.Context) error {
	id := c.Input("id", "is_number", "用户ID") // 自动做参数校验
	result := userservice.GetInfo(c.Context(), map[string]any{"id": id})
	return c.JSON(result)
}

func PostUser(c *server.Context) error {
	return c.JSON(map[string]any{"msg": "add"})
}
```

- `c.Input` 会依据校验规则读取参数并返回字符串或原始值。
- API 层负责解析输入、调用服务层、统一返回 JSON。
- 文件与函数名决定路由路径：`module/user/api/test.go` 中的 `GetUser` 对应 `GET /user/test/user`（详见下一节的路由生成规则）。

## 路由自动生成与更新

- Dever 会扫描 `module/*/api/*.go` 下导出的函数，通过命名规范生成 RESTful 路由，并写入 `data/router.go`。
- 在新增或修改 API 后执行：

```sh
go run ./dever/cmd/dever routes
```

- 若希望在整理依赖的同时刷新路由，可使用：

```sh
go run ./dever/cmd/dever init
```

`main.go` 会调用 `data.RegisterRoutes`，因此只要重新生成路由并启动服务，新的 API 即可生效。

## Redis 原子扣减组件

- 在 `config/setting.json` 配置 `redis` 段，设置连接地址、认证信息、连接池参数及统一前缀。
- 启动阶段调用 `lock.Configure` 即可记录配置，真正的 Redis 连接会在首次 `lock.Dec/lock.Inc` 触发时建立，业务层直接使用 `github.com/shemic/dever/lock` 包：

```go
balance, err := lock.Dec(ctx, "merchant:123:balance", 100, lock.WithFloor(0))
if errors.Is(err, lock.ErrInsufficient) {
    // 余额不足，给出提示或降级处理
}

// 下单失败后恢复额度
_, _ = lock.Inc(ctx, "merchant:123:balance", 100)
```

- `lock.Dec`/`lock.Inc` 使用 Lua 脚本保证 Redis 内部原子性，支持 `WithFloor`、`WithCeiling`、`WithTTL` 等选项，适合余额扣减、库存变更、活动预约等高并发场景。

## 事务、锁与一致性策略

- **事务**：使用 `orm.Transaction` 包裹需要原子性的业务逻辑；函数会自动判断上下文中是否已有事务并复用，出现 panic 将回滚：

  ```go
  err := orm.Transaction(ctx, func(txCtx context.Context) error {
      userModel := model.NewUserModel()
      profile := userModel.Find(txCtx, map[string]any{"id": uid}, nil)
      if len(profile) == 0 {
          return fmt.Errorf("用户不存在")
      }
      userModel.Update(txCtx,
          map[string]any{"id": uid},
          map[string]any{"status": 1},
      )
      return nil
  })
  ```

- **乐观锁**：为表结构增加 `version` 字段后，调用 `Update` 时将第四个参数设为 `true`，框架会自动比较版本并自增；冲突时抛出 `orm.ErrVersionConflict`：

  ```go
  if err := orm.Transaction(ctx, func(txCtx context.Context) error {
      _, err := userModel.Update(txCtx,
          map[string]any{"id": uid, "version": version},
          map[string]any{"status": 2},
          true,
      )
      return err
  }); err != nil {
      if orm.IsVersionConflict(err) {
          // 根据业务需要重试或提示用户
      }
  }
  ```

- **悲观锁**：`Model.Select` 的第四个可选参数为 `true` 时会在查询末尾追加 `FOR UPDATE`，配合事务使用即可锁定读取的行：

  ```go
  err := orm.Transaction(ctx, func(txCtx context.Context) error {
      userModel := model.NewUserModel()
      records := userModel.Select(txCtx,
          map[string]any{"main.id": uid},
          map[string]any{"page": 1, "pageSize": 1},
          true, // 开启悲观锁
      )
      if len(records) == 0 {
          return fmt.Errorf("用户不存在")
      }
      // 在同一事务中继续写操作
      userModel.Update(txCtx, map[string]any{"id": uid}, map[string]any{"status": 2})
      return nil
  })
  ```

  若需更灵活的 SQL，可仍然通过 `orm.Tx(ctx)` 获取底层 `*sqlx.Tx` 手写语句。悲观锁应尽量缩短持有时间；跨进程/跨服务场景推荐使用 Redis 分布式锁。

- **Redis 原子扣减与补偿**：`lock.Dec`/`lock.Inc` 提供原子增减操作，可结合 `defer` 实现失败补偿：

  ```go
  quota, err := lock.Dec(ctx, "coupon:123", 1, lock.WithFloor(0))
  if err != nil {
      return err
  }
  success := false
  defer func() {
      if !success {
          _, _ = lock.Inc(ctx, "coupon:123", 1)
      }
  }()
  // 执行业务逻辑，成功后将 success 置为 true
  ```

  可使用 `lock.WithCeiling` 设置上限、`lock.WithTTL` 控制过期时间；若需与数据库保持一致，可结合事务或异步补偿机制。

## 常见开发流程回顾

1. 在 `config/setting.json` 更新数据库、HTTP、日志等配置，并按环境区分。
2. 创建 `module/<name>` 目录，先编写 `model` 声明表结构，再根据业务编写 `service` 与 `api`。
3. 运行 `go run ./dever/cmd/dever routes` 更新 `data/router.go`，然后 `go run .` 启动服务验证。
4. 使用 ORM 的 `Select/Find/Insert/Update/Delete` 完成增删改查，必要时结合 `join`、复杂过滤条件满足检索需求。

至此，便可利用 Dever 快速构建模块化的 HTTP 服务；后续只需沿用上述约定，即可持续扩展新的业务模块。
