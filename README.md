# Dever 框架开发指南

本指南按当前 `dever/` 代码维护，面向业务项目中内嵌或引用 Dever 的开发者。重点是怎么启动、生成注册文件、写模块、接入 ORM、使用中间件、打包和提交。

## 1. 功能概览

| 目录 | 作用 |
| --- | --- |
| `cmd/` | 框架运行入口与生成器：启动 HTTP 服务，生成 `data/router.go`、`data/load/service.go`、`data/load/model.go`，执行迁移。 |
| `cmd/dever/` | 开发命令行：`run`、`build`、`init`、`routes`、`service`、`model`、`migrate`、`install`、`push`。 |
| `config/` | 读取 `config/setting.jsonc` 或 `config/setting.json`，提供日志、HTTP、数据库、Redis、observe、auth 等配置结构。 |
| `server/` | 统一 HTTP 抽象，封装 `server.Context`、请求参数、JSON 响应和 Fiber 适配。 |
| `middleware/` | 全局与路由级中间件注册，默认提供 Recover + Log。 |
| `load/` | Provider 与 Model 注册中心，读取生成后的 `data/load/*.go`。 |
| `orm/` | 泛型 Model、结构体 schema、自动迁移、CRUD、聚合、事务、乐观锁、schema 持久化。 |
| `lock/` | Redis 原子增减，支持上下限和 TTL。 |
| `log/` | 结构化访问日志和错误日志。 |
| `observe/` | 请求和 SQL 观测埋点，支持内置 provider 和扩展 provider。 |
| `auth/jwt/` | 可配置 JWT scheme、guard 和中间件。 |
| `util/` | JSONC、类型转换、模块源码解析等通用工具。 |

## 2. 快速启动

在业务项目根目录执行。若项目通过 `replace github.com/shemic/dever => ./dever` 使用本地框架，命令路径就是 `./dever/cmd/dever`。

```sh
go run ./dever/cmd/dever install
dever run
```

`install` 会把一个 `dever` 启动脚本写入用户 bin 目录，脚本始终执行当前项目内的 `dever/cmd/dever` 源码。后续日常开发优先使用 `dever run`，它会先执行 `init --skip-tidy`，并在 `module/*/{api,service,model}` 等敏感文件变化后重新生成注册文件再重启服务。

常用发布和提交命令：

```sh
dever build
dever push
```

`dever build` 默认打包当前项目根入口 `main.go`，目标为 `linux/amd64`、关闭 CGO，并使用 release 参数 `-trimpath -buildvcs=false -ldflags="-s -w -buildid="`。`dever push` 会读取 `git status`，把有变更的文件加入暂存区，执行 `git commit -m "edit"`，最后 `git push`；没有本地变更时直接 `git push`。

## 3. 配置

默认读取 `config/setting.jsonc`，不存在时回退 `config/setting.json`。数据库配置不使用 `connections` 包裹，多连接直接写在 `database` 下：

```json
{
  "log": {
    "level": "info",
    "output": "stdout",
    "successFile": "data/log/access.log",
    "errorFile": "data/log/error.log"
  },
  "observe": {
    "enabled": true,
    "provider": "builtin",
    "slowRequest": "500ms",
    "slowSQL": "200ms"
  },
  "http": {
    "host": "0.0.0.0",
    "port": 8080,
    "shutdownTimeout": "10s",
    "appName": "Dever",
    "enableTuning": true,
    "cors": {
      "enabled": true,
      "allowOrigins": ["*"]
    }
  },
  "database": {
    "create": true,
    "persist": true,
    "migrationLog": false,
    "default": {
      "driver": "mysql",
      "host": "127.0.0.1:3306",
      "user": "root",
      "pwd": "123456",
      "dbname": "demo",
      "maxOpenConns": 20,
      "maxIdleConns": 10,
      "connMaxLifetime": "300s"
    }
  },
  "redis": {
    "enable": false,
    "addr": "127.0.0.1:6379",
    "prefix": "demo:"
  }
}
```

多数据库示例：

```json
{
  "database": {
    "create": true,
    "default": "main",
    "main": {"driver": "mysql", "host": "127.0.0.1:3306", "user": "root", "pwd": "123456", "dbname": "main"},
    "report": {"driver": "mysql", "host": "127.0.0.1:3306", "user": "root", "pwd": "123456", "dbname": "report"}
  }
}
```

配置要点：

- `database.create=true` 会在首次加载 Model 时启用自动建表和结构更新。
- `database.persist=true` 会把 schema 记录到 `data/table`，`dever migrate <database>` 会读取这些记录应用到目标数据库。
- `database.default` 可以是连接名，也可以直接是默认连接对象。
- `http.cors.enabled=true` 时，未配置 method/header 会使用框架默认值。
- `observe.enabled=true` 时，请求中间件和 ORM 会记录 trace/span、慢请求和慢 SQL。

## 4. 命令行

安装后使用 `dever ...`；未安装时使用 `go run ./dever/cmd/dever ...`。

| 命令 | 说明 |
| --- | --- |
| `dever run [--project-root=.] [--entry=main.go] [--interval=800ms] [--skip-init]` | 热重载运行项目。默认启动前执行 `init --skip-tidy`，监听 `config`、`data`、`dever`、`middleware`、`module`、`package` 等目录。 |
| `dever daemon start\|stop\|restart\|status\|logs [--project-root=.] [--name=default] [-- <command...>]` | 后台运行和管理命令。`start` 需要命令，`restart` 不带命令时复用上次命令；pid、元数据和日志写入 `tmp/dever/daemon/<name>.*`。 |
| `dever build [--project-root=.] [--output=] [-o=] [--os=linux] [--arch=amd64] [--cgo=false] [target]` | release 打包。`target` 可以为空、目录或 `main.go`；默认输出到项目根目录的 `server`，Windows 自动补 `.exe`。 |
| `dever init [--project-root=.] [--skip-tidy]` | 执行 `go mod tidy`，然后生成 routes、service、model 注册文件。 |
| `dever routes [--project-root=.]` | 只扫描 API 并生成 `data/router.go`。 |
| `dever service [--project-root=.]` | 只扫描 Provider 并生成 `data/load/service.go`。 |
| `dever model [--project-root=.]` | 只扫描 Model 构造函数并生成 `data/load/model.go`。 |
| `dever migrate [--project-root=.] <database>` | 将 `data/table` 中记录的 schema 应用到指定数据库。 |
| `dever install [--project-root=.] [--bin-dir=]` | 安装本项目绑定的 `dever` 启动脚本。 |
| `dever push [--project-root=.] [--message=edit] [-m edit]` | 默认对调用 `dever` 时所在目录执行 git 操作；输出 `git status --short`，`git add` 变更文件，`git commit -m <message>`，最后 `git push`。 |

日常开发只需要 `dever run`。显式执行 `routes/service/model/init` 主要用于排查生成问题，生成文件不要手改：

- `data/router.go`
- `data/load/service.go`
- `data/load/model.go`

## 5. 服务入口

业务项目入口通常很薄：

```go
package main

import (
    "log"

    "your/project/data"
    "github.com/shemic/dever/cmd"
)

func main() {
    if err := cmd.Run(data.RegisterRoutes); err != nil {
        log.Fatal(err)
    }
}
```

`cmd.Run` 会读取配置、配置 Redis lock、初始化日志、初始化 observe、创建 Fiber Server、注册路由，并在收到退出信号后按 `http.shutdownTimeout` 优雅关闭。

## 6. 模块开发

业务模块放在 `module/<name>`。当前生成器关注三个目录：

```text
module/<name>/
  api/
  model/
  service/
```

如果 `module/<name>/main.go` 写了 `// dever:import <import-path>`，生成器会通过 `go list` 找到真实源码目录，适合把可复用 package 挂到项目模块名下。

### 6.1 Model

`model` 目录中所有导出的普通函数都会进入 `data/load/model.go`。推荐一个资源一个构造函数，返回 `*orm.Model[T]`：

```go
package model

import (
    "time"

    "github.com/shemic/dever/orm"
)

type User struct {
    ID        uint64    `dorm:"primaryKey;autoIncrement"`
    Name      string    `dorm:"size:64;not null"`
    Email     string    `dorm:"size:128;unique"`
    Status    int       `dorm:"default:1"`
    Version   int       `dorm:"default:1"`
    CreatedAt time.Time `dorm:"autoCreateTime"`
    UpdatedAt time.Time `dorm:"autoUpdateTime"`
}

type UserIndex struct {
    Email struct{} `unique:"email"`
    Status struct{} `index:"status"`
}

func NewUserModel() *orm.Model[User] {
    return orm.LoadModel[User]("用户", "user", orm.ModelConfig{
        Index:    UserIndex{},
        Order:    "id desc",
        Database: "default",
        Options: map[string]any{
            "status": []map[string]any{
                {"id": 1, "name": "启用"},
                {"id": 2, "name": "禁用"},
            },
        },
    })
}
```

说明：

- `LoadModel[T](name, table, config)` 会缓存模型实例。
- 首次加载 Model 时，ORM 会读取数据库配置并初始化连接。
- `ModelConfig.Index` / `Indexes` 描述索引，`Seeds` 描述建表初始数据，`Options` / `Relations` 可供后台页面和运行时元数据使用。
- 生成后的注册名形如 `user.NewUserModel`，可用 `load.Model("user.NewUserModel")` 获取。

### 6.2 Service 与 Provider

Service 承载业务编排，普通方法不要求固定签名；只有需要被 `load.Service` 动态调用的方法才写成 `ProviderXxx`。

```go
package service

import (
    "context"

    "your/project/module/user/model"
    "github.com/shemic/dever/server"
    "github.com/shemic/dever/util"
)

type UserService struct{}

func (UserService) Info(ctx context.Context, id uint64) *model.User {
    return model.NewUserModel().Find(ctx, map[string]any{"id": id})
}

func (s UserService) ProviderInfo(c *server.Context, params []any) any {
    var id uint64
    if len(params) > 0 {
        id, _ = util.ParseUint64(params[0])
    }
    return s.Info(c.Context(), id)
}
```

Provider 规则：

- 生成器只扫描 `service` 目录里的接收者方法。
- 接收者类型必须导出，方法名必须以 `Provider` 开头。
- 支持签名 `func(*server.Context, []any) any`。
- `ProviderInfo` 会注册为 `module.Type.Info`；上例在 `module/user` 中注册为 `user.UserService.Info`。
- 调用方式：`load.Service("user.UserService.Info", c, id)`。

### 6.3 API

API 层只负责参数读取、调用 Service、返回响应：

```go
package api

import (
    userservice "your/project/module/user/service"
    "github.com/shemic/dever/server"
    "github.com/shemic/dever/util"
)

type User struct{}

func (User) GetInfo(c *server.Context) error {
    id, _ := util.ParseUint64(c.Input("id", "required", "用户ID"))
    data := userservice.UserService{}.Info(c.Context(), id)
    return c.JSON(data)
}
```

路由规则：

- API 必须是结构体方法。
- 方法名前缀必须是 `Get`、`Post`、`Put`、`Delete`。
- `module/user/api/user.go` 中的 `func (User) GetInfo` 会生成 `GET /user/user/info`。
- 模块名为 `main` 时，路由省略模块前缀。

## 7. ORM 用法

查询：

```go
users := model.NewUserModel().Select(ctx,
    map[string]any{"status": 1},
    map[string]any{
        "field": "main.id, main.name, main.status",
        "order": "main.id desc",
        "page":  1,
        "pageSize": 20,
    },
)

user := model.NewUserModel().Find(ctx, map[string]any{"id": id})
```

返回值：

- `Select` 返回 `[]*T`，没有结果时返回空切片。
- `Find` 返回 `*T`，没有结果时返回 `nil`。
- `SelectMap` / `FindMap` 返回 map 结果，适合自定义字段或 join。
- `Count` / `Sum` 支持与 `Select` 类似的 `join`、`field` 选项。

写入：

```go
id := model.NewUserModel().Insert(ctx, map[string]any{
    "name": "admin",
    "email": "admin@example.com",
})

rows := model.NewUserModel().Update(ctx,
    map[string]any{"id": id, "version": version},
    map[string]any{"status": 2},
    true,
)

deleted := model.NewUserModel().Delete(ctx, map[string]any{"id": id})
```

说明：

- `Insert` 返回自增主键，驱动不支持时返回 `0`。
- `Update` / `Delete` 返回影响行数。
- `Update(..., true)` 使用乐观锁，要求模型存在 `version` 字段；冲突时触发 `orm.ErrVersionConflict`。
- 公共 `Select` API 当前不暴露 `FOR UPDATE` 参数；需要悲观锁时优先用事务和业务约束保证一致性，或在 ORM 层补明确的公共方法后再使用。

事务：

```go
err := orm.Transaction(ctx, func(txCtx context.Context) error {
    userModel := model.NewUserModel()
    user := userModel.Find(txCtx, map[string]any{"id": id})
    if user == nil {
        return fmt.Errorf("用户不存在")
    }

    userModel.Update(txCtx, map[string]any{"id": id}, map[string]any{"status": 1})
    return nil
})
```

## 8. 中间件、JWT 与 observe

默认生成的 `data/router.go` 会调用项目侧 `middleware.Register()`，项目可以在这里统一注册全局或路由中间件：

```go
package middleware

import (
    deverjwt "github.com/shemic/dever/auth/jwt"
    devermiddleware "github.com/shemic/dever/middleware"
)

func Register() {
    jwtGuard := deverjwt.UseConfigured(deverjwt.Options{})

    devermiddleware.UseGlobal(devermiddleware.Init())
    devermiddleware.UseRouteFunc("POST", "/user/user/info", jwtGuard)
}
```

要点：

- `middleware.Init()` 是 Recover + Log。
- `UseGlobal` / `UseRoute` 接收完整中间件链；`UseGlobalFunc` / `UseRouteFunc` 适合只依赖上下文的简单逻辑。
- JWT 配置由 `auth/jwt.Configure(config.Auth)` 建立运行时 scheme；项目可在启动或中间件注册阶段按需调用。
- observe 由 `cmd.Run` 根据配置自动初始化；请求日志中会带 `trace_id`、`span_id`。

## 9. Redis 原子扣减

```go
remaining, err := lock.Dec(ctx, "quota:user:1", 1, lock.WithFloor(0))
if errors.Is(err, lock.ErrInsufficient) {
    return c.Error("额度不足")
}

_, _ = remaining, err
```

`lock.Configure` 已由 `cmd.Run` 根据 `redis` 配置调用。常用选项：

- `WithFloor`：扣减下限，防止扣成负数。
- `WithCeiling`：增加上限，防止超过额度。
- `WithTTL`：操作成功后设置过期时间。

## 10. 发布与提交

常规流程：

```sh
dever run
dever daemon start --name run -- dever run
dever build
dever push
```

`dever build` 可指定目标：

```sh
dever build
dever build cmd/worker
dever build cmd/worker/main.go -o data/bin/worker --os=linux --arch=amd64
```

`dever push` 默认操作当前 shell 所在目录，即使 `dever` 启动脚本内部会切到框架源码目录运行，也不会改变 git 目标目录。可指定提交信息：

```sh
dever push -m "edit"
dever push --message "update user module"
```

提交前建议人工确认：

- `data/router.go` 是否因 API 变化更新。
- `data/load/service.go` 是否因 Provider 变化更新。
- `data/load/model.go` 是否因 Model 构造函数变化更新。
- `data/table` 是否因模型结构变化更新，并确认是否需要执行 `dever migrate <database>`。

核心原则：业务代码写在 `module/*`，生成文件交给 `dever run` 或 `dever init`，发布用 `dever build`，提交用 `dever push`。
