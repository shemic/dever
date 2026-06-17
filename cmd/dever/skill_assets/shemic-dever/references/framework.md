# Dever 框架速查

## 项目入口

Dever 应用项目的 Go module 固定为 `my`。标准入口、业务代码 import、package shim 都按 `my/...` 写；不要把 `module my` 改成项目名、域名或目录名。

标准入口：

```go
package main

import (
    "log"

    "my/data"
    _ "my/data/load"

    dever "github.com/shemic/dever/cmd"
)

func main() {
    if err := dever.Run(data.RegisterRoutes); err != nil {
        log.Fatal(err)
    }
}
```

如果启用 `package/front` 静态站点，入口可在 `dever.Run` 里同时注册：

```go
dever.Run(func(s server.Server) {
    data.RegisterRoutes(s)
    frontsite.Register(s)
})
```

## 命令

- `dever install`：安装本地 `dever` 启动脚本。
- `dever run`：热重载；启动前会执行 `init --skip-tidy`，model/service/api 变更后会再次刷新生成文件。存在 package/module 前端源码插件时，会启动插件 dev server；默认端口从后端 `http.port + 10000` 派生，例如 `8085 -> 18085`、`8082 -> 18082`，避免多项目都抢 `5174`。`DEVER_FRONT_PLUGIN_DEV_PORT` 可显式覆盖。
- `dever init --skip-tidy`：生成 routes/model/service 注册文件。
- `dever routes`：只生成 `data/router.go`。
- `dever model`：只生成 `data/load/model.go`。
- `dever service`：只生成 `data/load/service.go`。
- `dever migrate default`：按 `data/table` 应用表结构。
- `cd front && pnpm run build:backend`：构建主 `front` 运行时，输出到 `backend/package/front/html`；不包含 module/package 插件源码。
- `dever front build`：构建所有 `backend/package/*/front` 与 `backend/module/*/front` 插件前端。
- `dever front build bot`：只构建 `bot` 前端插件，输出到对应 `front/dist`。
- `dever package add bot`：从 `github.com/dever-package/bot` 拉取 package，创建 `module/bot/main.go` shim，并刷新生成文件。
- `dever package update bot`：更新已安装 package；默认要求 git 工作区干净并执行 `git pull --ff-only`。
- `dever build [target]`：发布构建；默认构建当前项目，`target` 可传目录或 `main.go`；默认先构建前端插件，再构建 Go 二进制。用户禁止 build 时不要运行。
- `dever build --skip-front`：只构建 Go 二进制，跳过 package/module 前端插件。

本地 replace 项目用 `go run ./dever/cmd/dever <cmd>`；普通项目用安装后的 `dever`。

## 源码定位

Dever 框架源码主仓库在 GitHub：

- 源码仓库：[shemic/dever](https://github.com/shemic/dever)
- Go 模块路径：`github.com/shemic/dever`。
- CLI 源码：`cmd/dever`，`dever run/init/routes/model/service/front/build/package` 都在这里。
- 应用运行入口库：`cmd`，业务入口通常 `import dever "github.com/shemic/dever/cmd"`。
- package/module front 插件编译器：`compiler/front`。

当前 `/data/project/shemic` 是特殊形态：本地放了 `backend/dever`，并用 `replace github.com/shemic/dever => ./dever` 指向本地源码。因此当前项目里还要知道：

- `backend/dever`：本地 Dever 框架源码。
- `backend/dever/cmd/dever`：当前项目正在使用的 CLI 源码。
- `backend/dever/cmd`：当前项目正在使用的应用运行入口库。
- `backend/dever/compiler/front`：当前项目正在使用的 front 插件编译器。
- `backend/package/front`：当前项目内置的站点运行时、后台页面、插件服务、上传、导入导出等通用 package。
- `backend/package/bot`：当前项目内置的 bot package。
- package 拉取来源：`https://github.com/dever-package/<name>.git`。
- `front`：主 front 运行时源码，构建产物输出到 `backend/package/front/html`。

如果 `go.mod` 有：

```go
replace github.com/shemic/dever => ./dever
```

应用使用的是本地 `./dever` 源码。当前服务器上的 `/usr/local/bin/dever` 也可能是进入 `backend/dever` 后执行 `go run ./cmd/dever "$@"`，遇到当前项目的 CLI 行为问题时先查 `backend/dever/cmd/dever`。

## 生成文件

永远不要手改：

- `data/router.go`
- `data/load/model.go`
- `data/load/service.go`
- `data/table/*.json`

改源文件后让命令刷新。

也不要手改编译产物来修前端问题：

- `package/front/html/assets/index*.js`
- `package/front/html/assets/*.css`
- `package/*/front/dist/*`
- `module/*/front/dist/*`

改源码、配置或 `config/front/assets`，再由正常构建/运行流程产生结果。

## module 与 package

Dever 扫描 `module/*`。如果 `module/<name>/main.go` 有：

```go
// dever:import my/package/bot
```

这个 module 是 package 引入 shim，真实源码来自 package。`my/package/...` 是应用项目固定 import 路径，不要替换成项目名。应用开发时不要复制 package 代码，也不要改 package 源码；只通过引入、配置、page JSON、Provider hook 等公开能力复用。只有明确维护 package 本身时，新增页面、model、service、api 才放到真实 package。

可复用 Go package 与 package 自带前端插件的结构和命令看 `references/package-plugin.md`。
package 前端插件静态服务走 `package/front/service/plugin`，不要在每个组件里复制 `service/frontplugin`。

## 路由生成

扫描 `api` 目录里的结构体方法：

```go
func (User) GetList(c *server.Context) error
func (User) PostCreate(c *server.Context) error
```

生成：

- `GET /<module>/user/list`
- `POST /<module>/user/create`

`module/main` 特殊：不带模块前缀。

API 只取参、调用 Service、返回：

```go
id := util.ToUint64(c.Input("id", "required", "ID"))
return c.JSON(result)
```

## Middleware 自动注册

`data/router.go` 由 Dever 生成，不手改。生成器会在注册路由前自动执行：

- `devermiddleware.UseDefault()`：框架默认 `Recover + Log`，每个项目都会有。
- `middleware/*.go`：项目根 middleware 可选；目录里有 `func Register()` 才导入调用。
- `module/package` 真实源码的 `middleware/*.go`：组件自带 middleware 可选；例如 `package/front/middleware/init.go` 会随 `module/front` shim 自动注册。

规则：

- 普通项目不需要写 `middleware/init.go`。
- 项目自定义 middleware 只放项目自己的横切逻辑，不复制 package 的鉴权、站点或插件逻辑。
- 项目自定义 `Register()` 不要再调用 `coremiddleware.Init()`；默认 `Recover + Log` 已由生成器统一挂载。
- package/module middleware 必须用 `Register()`，内部用 `sync.Once` 防止重复注册。
- package/module middleware 可在 `Register()` 阶段做一次性预热；请求期不要做目录扫描、全量同步或重初始化。
- 高频 middleware 状态检查用 `sync.Once`、atomic 或已有缓存快路径，避免每个请求抢锁。
- 生成器只在 `dever init/routes/run` 阶段扫描目录，请求期没有目录扫描开销。

## Load 注册

Model 只注册 `model` 目录里的零参数 `New*Model` 构造函数，注册名：

```txt
<module>[.<model子目录>].NewXxxModel
```

普通导出 helper、Options、Normalize、DefaultRuntimeConfig 之类函数不会作为 Model 注册。

Provider 扫描 `service` 目录里 `Provider` 开头的方法，注册名：

```txt
<module>[.<service子目录>].<Receiver>.ProviderXxx
```

Provider 签名：

```go
func (Hook) ProviderBeforeSaveUser(c *server.Context, params []any) any
```

调用时用生成注册名，不猜。

## 配置

主配置是 `config/setting.json` 或 `config/setting.jsonc`。常用块：

- `http`
- `log`
- `observe`
- `database`
- `redis`
- `auth`
- `frontSite`

日志默认写文件，不输出到 `dever run` 屏幕：

```jsonc
{
  "log": {
    "level": "info",
    "development": false,
    "enabled": true,
    "output": "file",
    "successFile": "data/log/access.log",
    "errorFile": "data/log/error.log"
  }
}
```

不要在业务代码里重复造配置系统。

空项目配置、front site 配置、logo/favicon、AGENTS 提示块从 `skills/skills-dever/files` 模板复制，规则见 `references/files.md`。
