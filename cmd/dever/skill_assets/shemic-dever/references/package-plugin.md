# Package / Front Plugin 规则

用于维护可复用 Go 组件，或给 package/module 增加前端插件。普通业务开发不要改 `backend/package/*`，除非用户明确要求维护 package。

## 1. Go package 组件

真实代码放 package：

```txt
backend/package/<name>/
  <name>fs.go
  middleware/
  model/
  service/
  api/
  front/page/{page}/
```

应用只放 shim：

```go
// backend/module/<name>/main.go
package <name>

// dever:import my/package/<name>
```

规则：

- Model 按 `model.md`。
- Page JSON 按 `front-page.md`。
- Provider / Service / API 按 `service-api.md`。
- 组件元数据和组件 skill 按 `component.md`。
- 不手改 `data/router.go`、`data/load/*.go`、`data/table/*.json`。
- package 的 page JSON 放 `backend/package/<name>/front/page/{page}`，path 仍归 package。`{page}` 规则见 `front-page.md`。
- `go:embed front/page` 放在 package 自己的 `fs.go`。
- package 自带中间件放 `middleware/init.go`，提供 `Register()`；Dever 通过 module shim 自动发现并注册。
- package middleware 内部必须 `sync.Once`，只写组件自己的横切逻辑，不写项目私有路径规则。
- package 内部运行时缓存优先复用 `dever/cache`，并通过组件自己的统一失效入口管理。
- `package/front` 的页面、权限、option 等 runtime 缓存统一接入 `runtimecache.Register`；保存、删除、导入等写操作成功后主动失效。
- front 插件仍按页面实际 node 按需加载，不要因为缓存或预热改成全量加载插件。

## 2. 项目引入 package

项目里需要新增组件时，优先用命令：

```bash
dever package add bot
```

它会从 `https://github.com/dever-package/bot.git` 拉取到 `package/bot`，创建 `module/bot/main.go` shim，并刷新 routes/model/service 注册。换组件时把 `bot` 换成对应名称。

空项目只要用户要基于 site 建系统，按当前 site 基线同时引入 `front` 和 `bot`，不要只装 `front`：

```bash
dever package add --skip-init front
dever package add --skip-init bot
dever init --skip-tidy
```

`front` 提供站点运行时、页面、路由、上传、导入导出等通用能力；`bot` 提供当前 site 基线需要的 AI/agent 相关 package 能力。一次补多个 package 时先 `--skip-init`，最后只刷新一次生成文件。

可选项：

```bash
dever package add --project-root=backend bot
dever package add --repo-base=https://github.com/dever-package bot
dever package add --skip-init bot
```

更新已安装组件：

```bash
dever package update bot
```

默认更新规则：

- `package/bot` 必须是 git 仓库。
- 本地有未提交或未处理文件时直接报错。
- 更新动作是 `git pull --ff-only`，不做合并提交。
- `module/bot/main.go` 缺失时会补 shim；存在但不是目标 shim 会报错。
- 更新后默认刷新 routes/model/service 注册。

确认要丢弃本地 package 改动并重拉时，才使用：

```bash
dever package update --force bot
```

手动引入时使用同样结构：

```txt
backend/package/<name>/   # 确保 package 源码已存在，本地 package 直接放这里
backend/module/<name>/main.go
```

`module/<name>/main.go`：

```go
package <name>

// dever:import my/package/<name>
```

然后刷新生成文件：

```bash
dever init --skip-tidy
```

应用项目的 `go.mod` 固定是 `module my`，所以 package 模板、shim 和项目内 import 里的 `my/package/<name>` 要保持不变；不要替换成项目名、域名或目录名。

如果 package 来自独立 Go module，不要复制代码；在 `go.mod` 配好 `require/replace`，shim 的 import 写真实 Go import path。Dever 会通过 `go list` 解析真实源码目录。

package 自带前端插件会由 `package/front` 的站点服务发现；不要在每个组件里复制插件静态服务。

复杂 package 应该带自己的组件 skill：

```txt
package/<name>/skills/<name>/SKILL.md
```

维护该 package 前先读组件 skill。

## 3. Package 前端插件目录

复杂 React 节点不要塞进主 `front/src`，放 package 自己的 `front`：

```txt
backend/package/<name>/front/
  page/{page}/
  src/
    plugin.ts
    nodes/
    components/
  dist/
    placeholder.txt
```

module 也可用同样结构：

```txt
backend/module/<name>/front/
  page/{page}/
  src/plugin.ts
```

开发态有两种：

1. 主 front 源码开发：`cd front && pnpm dev`，访问 5173。
2. 应用开发者模式：`dever run`，访问 8085；主 front 使用 `package/front/html`，插件源码由 Dever CLI 内置的 `compiler/front` 编译，8085 代理后按需加载。

两种模式都会扫描：

```txt
backend/package/*/front/src/plugin.ts
backend/module/*/front/src/plugin.ts
```

所以开发时改插件 `front/src` 不需要先打包插件。8085 模式不是让浏览器直接执行 TSX，而是由 `dever run` 启动 Dever CLI 的前端插件编译器，再按页面实际 node 通过 `{site}/plugins-src/{name}/runtime.js` 加载编译后的 ESM。开发者不需要也不应该依赖主 `front/src`。

多站点 front 只记一条：站点配置在 `config/front.json.sites`，页面目录是 `front/page/{page}`，业务前台优先放 `module/<name>`，可复用能力放 `package/<name>`。

## 4. 最小 front 插件流程

以 `work` 前台站点为例：

```txt
backend/module/work/front/
  page/work/home.json
  src/plugin.ts
  src/nodes/home/home-shell.tsx
```

`src/plugin.ts`：

```ts
import { defineFrontPlugin, lazyNode } from "@dever/front-plugin";

export default defineFrontPlugin({
  name: "work",
  nodes: {
    "work-home-shell": lazyNode(() =>
      import("./nodes/home/home-shell").then((mod) => ({
        default: mod.WorkHomeShell,
      })),
    ),
  },
});
```

`page/work/home.json` 引用插件节点：

```json
{
  "page": { "name": "工作台首页", "type": 1 },
  "layout": {
    "type": "container",
    "children": {
      "content": { "type": "container" }
    }
  },
  "nodes": {
    "content": [
      { "type": "work-home-shell" }
    ]
  },
  "data": {},
  "state": {},
  "action": {}
}
```

规则：

- `plugin.ts` 只注册节点，不请求接口、不读配置、不做全局副作用。
- 节点名要有 package/module 前缀，例如 `work-home-shell`，避免和 package/front 内置节点冲突。
- 组件只从 `@dever/front-plugin` 引入 SDK、类型、UI、request，不直接 import 主 `front/src`。
- 页面 JSON 引用了插件 node，运行时才加载对应插件；不要手写插件清单。
- 普通后台列表/表单不写插件；只有强交互、复杂布局、图形化编辑器、工作台 Shell 等场景才写。

## 5. 插件入口模板

`plugin.ts` 只注册能力，不做副作用。插件只能依赖公开 SDK，不要依赖主 `front/src` 的 `@/...` 路径：

```ts
import { defineFrontPlugin, lazyNode } from "@dever/front-plugin";

export default defineFrontPlugin({
  name: "bot",
  nodes: {
    "show-agent": lazyNode(() =>
      import("./nodes/show/agent").then((mod) => ({ default: mod.ShowAgent })),
    ),
  },
});
```

不要再写 `runtime.ts`；Dever CLI 前端插件编译器会按 `plugin.ts` 自动生成开发态和发布态注册入口。

`nodes` 和 `depends` 都从 `plugin.ts` 自动提取，用于运行时按需加载插件。不要在 `front.json` 里手写插件 node 清单；页面 JSON 引用了某个插件 node，主 front 才会加载对应插件。

节点组件使用 `@dever/front-plugin` 暴露的 SDK 和组件：

```ts
import type { NodeItemProps } from "@dever/front-plugin";
import { Button, request } from "@dever/front-plugin";
```

不要在插件里复制主 front 的 UI、请求、上传、agent runner、类型，也不要直接 import 主 `front/src`。旧插件里的 `@/...` 会由 compiler 兼容，但新代码必须用 `@dever/front-plugin`。

## 6. 前台站点和后台站点怎么分工

后台 `admin`：

- 配置在 `config/front.json.sites.admin`。
- `api` 通常是 `front`，`access.mode` 通常是 `rbac`。
- 页面放 `module/<biz>/front/page/admin/...`。
- 普通 CRUD 用 page JSON 和 model 元信息，不写 front 插件。

前台 `work` / `portal` / `shop`：

- 配置在 `config/front.json.sites.<siteKey>`。
- `api` 可以是业务分组，例如 `work`。
- 页面放 `module/<biz>/front/page/{page}/...`。
- 登录、注册、复杂动作可以写 `module/<biz>/api`。
- 需要完整 React 体验时，在同一个 module/package 下写 `front/src/plugin.ts`。

可复用业务能力放 `package/<name>`；项目私有业务放 `module/<name>`。应用开发不要为了改一个页面复制 package 源码。

## 7. 复用 admin 形式还是做自定义界面

其他站点不是从零写整套前端，也不是自动复用 admin 的 `main.json`。它们共用主 front runtime、Page JSON 渲染器、请求封装、插件按需加载；站点壳由该站点的 `main.json` 决定。

复用 admin 形式：

- 适合后台型系统、运营系统、内部工作台。
- `config/front.json.sites.<siteKey>` 使用自己的 `api/page/access/authProvider`。
- 在该站点 page 目录放自己的 `main.json` / `login.json`，结构参考 `package/front/page/admin/main.json` 和 `login.json`。
- `main.json` 组合 `app-sidebar`、`app-topbar`、`app-outlet`、按需 `app-assistant`。
- `login.json` 可复用 `app-site-brand`、`app-login-form`；登录接口由当前站点 `api` 决定。
- 独立账号优先写业务模块 `api/auth.go` 和 `service/auth.go`，返回兼容的 `token`、`user.id`、`user.name`、`user.account`。
- 如果顶部账户资料、角色切换、权限管理需要独立，提供站点自己的节点或页面；不要让业务站点误用管理员 `/front/account/profile`。

做一套其他界面：

- 适合门户、C 端、画布、项目空间、强交互工作台。
- `main.json` 可以只是薄壳，例如全屏 container + `app-outlet`，或组合自定义顶部导航。
- 简单内容页继续用 Page JSON 和内置节点，不要上来就写插件。
- 复杂体验才写 `front/src/plugin.ts` 和业务节点，例如 `work-login-page`、`work-home-shell`、`work-space-page`。
- 插件节点只承载 UI 和交互，登录、注册、保存、业务动作放 module API/Service。

权限边界：

- `access.mode: "login"` 会绕过 `front` RBAC，菜单来自当前站点可扫描到的 page/auth 记录，适合独立业务账号。
- `access.mode: "rbac"` 走 `package/front` 的账号、角色、权限模型。除非正在维护 `package/front`，不要承诺多站点各自一套完整 RBAC 只靠配置即可完成。

## 8. React 依赖规则

插件前端不能自己打包一份 React。

- 主 front 源码开发态：主 `front` 的 Vite alias / dedupe 提供同一份 React。
- 8085 源码插件开发态：Dever CLI 前端插件编译器编译插件，后端代理会把 Vite 的 React 依赖映射到主 front 暴露的 `window.React`。
- 发布态：插件构建必须 external `react`，由主 front 暴露 `window.React`。
- 不要在插件 `package.json` 里单独升级 React。

如果出现 hook 报错，先查是否打了两份 React。

## 9. 构建命令

主 front 开发：

```bash
cd front
pnpm dev
```

应用开发者模式：

```bash
dever run
```

`dever run` 检测到 `package/*/front/src/plugin.ts` 或 `module/*/front/src/plugin.ts` 后，会自动安装/复用 Dever CLI 前端插件编译器依赖并启动插件源码编译服务，后端站点仍访问：

```txt
http://host:8085/admin/
http://host:8085/work/
```

临时关闭插件源码模式：

```bash
DEVER_FRONT_PLUGIN_DEV=0 dever run
```

发布前构建所有插件前端：

```bash
dever front build
```

只构建某个插件：

```bash
dever front build bot
```

主 `front` 运行时单独构建，输出到 `backend/package/front/html`。它只包含基础框架和基础组件，不把 `backend/package/*/front/src`、`backend/module/*/front/src` 编进主包：

```bash
cd front
pnpm run build:backend
```

完整发布。`target` 可选，能传目录或 `main.go`；不传就构建当前项目：

```bash
dever build [target]
```

`dever build` 默认先执行前端插件构建，再 Go build。只想构建 Go：

```bash
dever build --skip-front
```

用户说不要 build/test 时，不运行这些命令。

## 10. 前端产物服务与二进制

插件构建产物输出到自己的 `front/dist`。后端站点发布态会自动发现 `backend/package/*/front/dist/manifest.json` 与 `backend/module/*/front/dist/manifest.json`；`dever run` 开发态优先发现源码插件，并按后端 `http.port + 10000` 派生插件 dev server 端口，例如 `8085 -> 18085`、`8082 -> 18082`。运行时会先根据页面 schema 的 node 类型判断需要哪些插件，再加载对应插件入口。page JSON 放 `front/page`。package 需要进二进制时用 `go:embed` 带进产物：

```go
//go:embed front/page
var PageFS embed.FS

//go:embed front/dist
var FrontFS embed.FS
```

复杂 React 节点放 package/module 自己的 `front/src/plugin.ts`，通过 `lazyNode` 按需加载；页面 JSON 没引用对应 node 时，不应加载对应业务 chunk。
