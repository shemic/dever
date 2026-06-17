# 空项目多站点系统接入流程

适用场景：空目录、最小 Dever 项目、或刚初始化的项目需要搭任意基于 `front` site 的系统，包括管理后台、工作台、业务前台、运营系统、门户等。这里说的是空项目 site 基线，不是某个行业系统；只有用户明确给出业务资源时，才命名具体业务模块。

## 1. 参考源

优先参考当前项目源码和本 skill 模板：

1. `skills/skills-dever/files`
2. 当前 `package/front`
3. 当前 `package/bot`
4. 当前 `package/user`
5. 当前项目已有 `module/*` / `package/*`
6. 当前 `backend/dever`

外部 demo 只作为兜底参考，不作为首选来源。不要从 demo 复制业务模块、旧页面写法或旧配置结构。

## 2. 空项目最小骨架

Dever 应用项目的 Go module 固定使用 `my`。不要按项目名、公司域名或目录名改成其他 module path；`module/front`、`module/bot` 等组件 shim 依赖 `my/package/...`，改名会导致 package 组件不可用。

`go.mod` 第一行保持：

```go
module my
```

空项目先补项目骨架，再按 site 补系统和业务：

```txt
go.mod
main.go
config/setting.jsonc
config/front.jsonc
config/front/assets/admin/images/logo.svg
config/front/assets/admin/images/favicon.svg
config/front/assets/work/images/logo.svg
config/front/assets/work/images/favicon.svg
data/readme.txt
package/readme.txt
module/front/main.go         # package/front shim
module/bot/main.go           # package/bot shim
```

`data/router.go`、`data/load/*.go`、`data/table/*.json` 都由 Dever 生成，不手写、不手改。

## 3. 空项目默认 package

用户说“新建项目”、“空项目”、“搭系统”、“搭站点”、“搭后台”、“admin”、“work”、“使用 front 组件”时，site 系统基线同时引入 `front` 和 `bot`，不要只装 `front`：

```bash
dever package add --skip-init front
dever package add --skip-init bot
dever init --skip-tidy
```

如果只装单个 package，可以不加 `--skip-init`；一次补多个 package 时，先 `--skip-init`，最后只运行一次 `dever init --skip-tidy`。

安装后确认：

- `package/front`、`package/bot` 已存在。
- `module/front/main.go` 是 `// dever:import my/package/front` shim。
- `module/bot/main.go` 是 `// dever:import my/package/bot` shim。
- package 源和 shim 里的 `my/package/front`、`my/package/bot` 是固定路径，不要替换成项目名或当前目录名。

## 4. 配置顺序

空项目配置优先从模板生成：

```txt
skills/skills-dever/files/config/setting.jsonc.tmpl
skills/skills-dever/files/config/front.jsonc.tmpl
```

先配置 `config/setting.jsonc`：

- `log.output="file"`，不要用 `stdout`。
- `log.successFile="data/log/access.log"`。
- `log.errorFile="data/log/error.log"`。
- 常规访问日志和错误日志写到 `data/log`，不要刷到 `dever run` 屏幕。
- `http.port`、`http.appName` 使用项目值。
- `frontSite.enabled=true`。
- `frontSite.enabled=true` 是站点服务开关；站点细节放 `config/front.jsonc`。
- 数据库按用户指定环境配置；如果用户指定 PostgreSQL，直接设置 `driver=postgres`、目标 `dbname`、项目 `prefix`，不要先落 SQLite 再切。

最小日志配置：

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

再配置 `config/front.jsonc`：

- 每个系统对应一个 `sites.<siteKey>`，例如 `admin`、`work`、`portal`、`shop`。
- `sites.<siteKey>.api` 使用该系统的 API 分组；后台通常为 `front`，业务前台可按 demo 的 `work` 站点方式配置。
- `sites.<siteKey>.page` 决定物理页面目录，页面放到 `front/page/{page}/...`；`siteKey` 和 `page` 可以同名，也可以不同名。
- `sites.<siteKey>.access` 使用该系统需要的登录、RBAC 或公开访问模式。
- `sites.<siteKey>.assets.logo/favicon` 的相对路径映射到 `config/front/assets/{siteKey}/`；默认图标从 `skills/skills-dever/files/config/front/assets/<site>/images/` 复制。
- `public` 保留上传、站点信息、bot 回调/请求等 package 需要的公开路径。
- 菜单只放当前 site 的真实功能分组；bot 自带页面按 package 能力接入，不要复制页面实现。

一个后台 `admin` 加一个前台 `work` 的最小形态可直接使用 `files/config/front.jsonc.tmpl`，需要新增站点时再按同样结构扩展：

```jsonc
{
  "public": [
    "/upload/*",
    "/site/info",
    "/bot/energon/request",
    "/bot/energon/demo"
  ],
  "sites": {
    "admin": {
      "name": "管理后台",
      "api": "front",
      "page": "admin",
      "access": {
        "mode": "rbac",
        "authProvider": "front"
      },
      "public": ["auth/login"],
      "auth": []
    },
    "work": {
      "name": "工作台",
      "api": "work",
      "page": "work",
      "access": {
        "mode": "login",
        "authProvider": "work"
      },
      "public": ["auth/login", "auth/register"],
      "auth": []
    }
  }
}
```

含义：

- `admin` 是后台站点，普通 CRUD 走 `Model + page JSON`，页面放 `front/page/admin`。
- `work` 是前台站点，页面放 `front/page/work`，登录注册和复杂业务动作可放 `module/work/api`。
- `siteKey` 决定访问路径和 runtime，例如 `/admin/runtime.js`、`/work/runtime.js`。
- `page` 只决定物理页面目录，不进入最终 route。

## 5. 多站点复用决策

`package/front/html` 和主 React runtime 是所有 `sites` 共享的。每个站点的差异来自 `config/front.json.sites.<siteKey>` 注入的 runtime：`siteKey`、`api`、`page`、`access`、资源和展示信息。

不要把“复用 admin 样式”和“复用 admin 账号权限”混为一件事：

- 复用 admin 样式：复用 `app-sidebar`、`app-topbar`、`app-outlet`、`app-login-form` 等内置节点，在新站点自己的 `main.json` / `login.json` 里组合。
- 独立账号：新站点使用独立 `api` 和 `access.authProvider`，登录接口放业务模块，例如 `module/work/api/auth.go` 暴露 `/work/auth/login`。
- 独立权限：如果只是登录后可访问，优先用 `access.mode: "login"`；业务权限放业务 Service 校验。
- 完整 RBAC：当前 `front` RBAC 绑定 `front` 账号、角色、权限模型。不要承诺纯配置即可给每个站点一套独立 RBAC；需要维护 `package/front` 做 provider/scope 级扩展，或新增业务自己的权限体系。

两种常见方案：

| 目标 | 推荐做法 |
| --- | --- |
| 后台型系统，想像 admin 一样的侧栏、顶栏、列表表单 | 新站点保留自己的 `api/page/access`，在自己的 page 目录复制/抽取 admin `main.json` 和 `login.json` 壳，业务页面继续用 Page JSON。 |
| 工作台、门户、画布、C 端业务界面 | `main.json` 做薄壳或自定义壳，普通页面用 Page JSON，复杂体验用 `module/<site>/front/src/plugin.ts` 注册业务节点。 |

不能只把新站点配置成 `page: "admin"` 就认为会复用 admin 系统页。`main/login` 的逻辑 route 前缀跟当前站点 `api` 走，例如 `api: "work"` 时系统页是 `work/main`、`work/login`；`page` 只决定这些页面从哪个物理目录读取。

如果复用 admin 顶栏，注意它可能包含账户资料入口。业务账号独立时，要么让业务登录返回兼容的 `user.id/name/account`，要么提供站点自己的 profile 页面/节点，不要让前台账号误用 `/front/account/profile` 这类管理员页面。

## 6. 入口注册

空项目安装 `front` 后，入口要注册站点服务：

```go
import (
	"log"

	"my/data"
	_ "my/data/load"
	frontsite "my/package/front/service/site"

	dever "github.com/shemic/dever/cmd"
	"github.com/shemic/dever/server"
)

func main() {
	if err := dever.Run(func(s server.Server) {
		data.RegisterRoutes(s)
		frontsite.Register(s)
	}); err != nil {
		log.Fatal(err)
	}
}
```

`bot` 通过 package/module shim、生成注册和页面/接口能力接入；没有 package 文档要求时，不在 `main.go` 里额外硬编码 bot 注册。

## 7. 后续业务资源

空项目 site 立住后，新增普通资源走标准路径：

1. `module/<biz>/model/<resource>.go`
2. `module/<biz>/front/page/{page}/<resource>/list.json`
3. `module/<biz>/front/page/{page}/<resource>/update.json`
4. `dever init --skip-tidy`

普通列表、录入、编辑、详情不要默认写 API/Service。只有状态流转、跨表保存、强校验、外部协议、异步任务、聚合查询等真实业务规则，才补 Service/API。

后台页面例子：

```txt
module/product/model/goods.go
module/product/front/page/admin/goods/list.json
module/product/front/page/admin/goods/update.json
```

前台页面例子：

```txt
module/work/front/page/work/main.json
module/work/front/page/work/home.json
module/work/front/src/plugin.ts              # 可选，复杂 React 节点才需要
module/work/front/src/nodes/home/home.tsx    # 可选
```

## 8. 交付检查

交付前至少静态确认：

```bash
rg -n "my/package/(front|bot)|module/front|module/bot|frontSite|sites" .
bash skills/skills-dever/scripts/audit.sh <改动文件或目录>
```

如果用户明确禁止 build/test，不运行 `dever build`、`dever front build`、`npm run build`、测试命令。若为了验证临时启动服务，结束前要关闭进程，除非用户明确要求保留。
