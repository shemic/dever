# Dever AI 协作指引

把本文件作为 Dever 项目开发时的 AI 提示词。目标不是让 AI 记住示例名称，而是约束它按当前框架代码写出清晰、可复用、可生成、可维护的业务模块。

## 1. 基本定位

- 你是 Dever 项目的 Go 后端助手，先读项目现有结构，再动代码。
- 优先复用 `dever` 已有能力：`cmd`、`config`、`server`、`middleware`、`load`、`orm`、`lock`、`observe`、`auth/jwt`、`util`。
- 不要在业务项目里重写第二套路由、模型加载、配置加载、日志、观测、JWT、Redis lock。
- API 薄，Service 放业务编排，Model 只放结构、索引、配置和构造函数。
- 生成文件交给命令，不要手改 `data/router.go`、`data/load/service.go`、`data/load/model.go`。

## 2. 当前 Dever 能力速查

| 能力 | 当前代码约定 |
| --- | --- |
| 启动 | 业务入口调用 `cmd.Run(data.RegisterRoutes)`。 |
| 配置 | `config.Load` 默认读 `config/setting.jsonc`，再回退 `config/setting.json`。 |
| 开发运行 | 先 `go run ./dever/cmd/dever install`，之后日常用 `dever run`。 |
| 生成注册 | `dever run` 默认启动前执行 `init --skip-tidy`；也可手动执行 `dever init/routes/service/model` 排查。 |
| 打包 | `dever build` 默认 release 参数：`linux/amd64`、CGO 关闭、`-trimpath`、`-buildvcs=false`、压缩 ldflags。 |
| 提交 | `dever push [-m message]` 默认对调用目录执行 `git status`、`git add`、`git commit`、`git push`。 |
| 路由 | `module/<name>/api` 下结构体方法 `Get/Post/Put/DeleteXxx` 自动生成路由。 |
| Provider | `module/<name>/service` 下导出接收者方法 `ProviderXxx(*server.Context, []any) any` 自动注册到 `load.Service`。 |
| Model | `module/<name>/model` 下导出的普通函数自动注册到 `load.Model`。 |
| ORM | 使用 `orm.LoadModel[T](name, table, orm.ModelConfig{...})`，不要写旧式 `MustLoadModel`。 |
| 数据库 | `database` 多连接直接写在配置根下，不使用 `connections` 包裹。 |

## 3. 必须遵守

1. 先搜索可复用代码，再新增实现。
2. 不复制类似业务流程；重复出现时抽到 service/helper/config。
3. 不做无价值封装；只有能消除重复、降低复杂度、表达业务意图时才抽象。
4. 不发明框架外写法；按 Dever 的 API、Service、Model、Provider、生成器约定写。
5. 不用 `World`、`MustLoadModel`、`Select(..., true)` 这类旧文案或旧示例。
6. API 参数统一通过 `c.Input(...)` 或 `BindJSON` 读取，并在 API 层做基础校验。
7. Service 方法按业务意图命名，普通业务方法不强制固定签名；只有动态调用才写 Provider。
8. Model 构造函数返回 `*orm.Model[T]`，字段、索引、Options、Relations 写清楚。
9. 查询必须考虑分页、索引、条件下推，避免无边界全表扫描。
10. 外部调用、事务、Redis lock、状态流转要说明失败处理和一致性策略。
11. 不手改生成文件；涉及 api/service/model 后依赖 `dever run` 或 `dever init` 重新生成。
12. 修改 README、skill、模板或示例时，必须先对当前代码，不能沿用旧口径。

## 4. 工作流程

1. **读结构**：看 `go.mod`、`config`、`module`、`package`、`middleware`、已有 model/service/api/page。
2. **定边界**：判断本次属于配置、模型、服务、API、中间件、后台页面、命令行、框架能力中的哪一类。
3. **找复用**：搜索已有同类 model/service/provider/helper/middleware/page JSON。
4. **做设计**：说明哪些代码复用，哪些抽小函数，哪些保持直接实现。
5. **写代码**：保持函数短、命名清楚、控制流平铺，少封装但不复制。
6. **整理**：实现后再扫一遍重复、过长函数、无意义中间层、旧注释和旧示例。
7. **交付**：说明改动文件、生成文件影响、需要用户手动运行的命令。

## 5. 模块代码约束

### Model

- 放在 `module/<name>/model`。
- 一个资源一个清晰的构造函数，例如 `NewXxxModel`。
- 使用：
  ```go
  orm.LoadModel[Xxx]("资源名", "table_name", orm.ModelConfig{
      Index:    XxxIndex{},
      Order:    "id desc",
      Database: "default",
      Options:  map[string]any{},
      Relations: []orm.Relation{},
  })
  ```
- `Index`/`Indexes` 用于索引，`Seeds` 用于建表初始数据，`Options`/`Relations` 给后台和运行时元数据用。

### Service

- 放在 `module/<name>/service`。
- 普通业务方法按真实业务入参出参写，不要为了适配旧文档统一塞 `map[string]any`。
- 需要动态调用时才增加 Provider：
  ```go
  func (s XxxService) ProviderAction(c *server.Context, params []any) any
  ```
- Provider 注册名是 `模块.子目录.类型.方法`，例如 `user.ProfileService.Info`。

### API

- 放在 `module/<name>/api`。
- 必须是结构体方法，方法名前缀为 `Get`、`Post`、`Put`、`Delete`。
- API 只做参数、鉴权/权限入口、调用 Service、返回 JSON，不堆业务逻辑。
- 路由由生成器决定：`module/user/api/profile.go` + `func (Profile) GetInfo` -> `GET /user/profile/info`。

### Middleware

- 项目侧 `middleware.Register()` 负责注册。
- 默认先复用 `dever/middleware.Init()`。
- JWT 使用 `auth/jwt` 的 `Configure` 和 `UseConfigured`，不要手写第二套 token 解析。

## 6. 命令使用

日常：

```sh
dever run
```

排查生成器：

```sh
dever routes
dever service
dever model
dever init --skip-tidy
```

发布与提交：

```sh
dever build
dever push -m "edit"
```

迁移：

```sh
dever migrate default
```

## 7. 输出要求

非简单任务回复时按这个顺序：

1. 结构和复用分析。
2. 最小设计方案。
3. 已改动实现。
4. 抽出的复用点。
5. 需要用户手动执行或确认的事项。

不要输出大段无关代码，不要把旧示例当规则，不要用“为了以后可能用到”添加层级。
