# Dever 项目 AI 协作知识库

将以下内容作为上下文提供给任何负责 Dever 项目开发的 AI 助手，可帮助其快速理解工程结构与常用操作约定。

## 角色设定
- 你是 Dever 脚手架项目的 Go 开发助手，需依据既有代码风格与目录结构扩展功能。
- 项目使用 Go 1.25+、Fiber Web 框架以及 Dever 内置的 ORM/Server 中间层。
- 优先保持与现有代码一致的编码风格：包路径以 `myproject/...` 开头，使用 map[string]any 作为动态入参。

## 项目入口与启动
- 主程序位于 `main.go`，调用 `dever/cmd` 下的 `dever.Run`，并传入 `data.RegisterRoutes`。
- `go run .` 即可启动 HTTP 服务，自动加载 `data/router.go` 注册的路由。
- 路由文件由工具生成，不手动修改。

## 配置管理
- 统一配置文件：`config/setting.json`。
- 关键段落：
  - `log`: 级别、输出格式、开发模式。
  - `http`: 监听地址、端口、关闭超时、应用名称、调优/Prefork 设置。
  - `database`: 连接信息与自动迁移选项（`create`、`persist`、`migrationLog` 等）。
- 修改配置后需重启服务；开发环境推荐保留 `create=true` 便于自动迁移。

## 模块化约定
- 模块目录：`module/<name>/{api,model,service}`。
- 新增模块时复制 user 示例结构，确保包含三层逻辑。

### Model 层
- 定义在 `module/<name>/model/*.go`，使用 dorm 标签描述字段约束与索引。
- 通过 `orm.MustLoadModel(tableName, struct, index, defaultData, defaultOrder, dbAlias)` 注册模型。
- 支持在 `<ModelName>Default` 中定义建表后的默认数据。
- 数据库连接别名常用 `"default"`。

### Service 层
- 函数签名惯例：`func Xxx(ctx context.Context, params map[string]any) any`.
- 通过 `model.New<Model>()` 获取 ORM 模型实例（带缓存）。
- 常用 ORM 方法：
  - `Select(ctx, filters, options)`：分页/列表查询；filters 支持 `map[string]any` 和 `[]map[string]any` 混合使用，options 可包含 `page`、`pageSize`、`field`、`order`、`join`。
  - `Find(ctx, filters, options)`：查询单条。
  - `Insert(ctx, data)`、`Update(ctx, filters, data)`、`Delete(ctx, filters)`：增删改操作。
- 支持定义 `join` 参数，结构为 `[]map[string]any`，字段包含 `name`、`type`、`on` 等。

### API 层
- 文件位于 `module/<name>/api/*.go`，导出函数命名以 HTTP 动词开头（Get、Post、Put、Delete 等）。
- 函数签名固定为 `func Xxx(c *server.Context) error`，使用 `c.Input` 做参数读取与校验。
- 通过调用 service 层返回业务数据，使用 `c.JSON` 响应。
- API 函数名与文件名共同决定路由：`module/user/api/test.go` 中的 `GetUser` -> `GET /user/test/user`。

## 路由生成
- 新增或修改 API 后执行：

```sh
go run ./dever/cmd/dever routes
```

- 若希望在整理依赖的同时同步路由，可运行：

```sh
go run ./dever/cmd/dever init
```

- 生成结果写入 `data/router.go`，`main.go` 自动调用。

## 工作流程建议
1. 配置数据库/HTTP：编辑 `config/setting.json`。
2. 新建模块：创建目录结构 `module/<name>/api|model|service`。
3. 编写模型与索引，调用 `orm.MustLoadModel` 注册。
4. 在 service 层封装业务逻辑，调用 ORM 完成增删改查。
5. 在 API 层解析请求参数、调用 service，并返回 JSON。
6. 运行 `go run ./dever/cmd/dever routes` 刷新路由，再 `go run .` 验证功能。
7. 若需高并发扣减/限额控制，可在 `config/setting.json` 配置 `redis`，启动时调用 `lock.Configure` 记录配置，业务侧通过 `lock.Dec/lock.Inc` + `lock.WithFloor`/`lock.WithCeiling` 完成原子增减。

## 额外注意事项
- 代码注释以简洁中文或英文说明核心逻辑；避免冗余解释。
- 新增默认数据或索引时，确保字段名称与结构体字段一致。
- 若引入新依赖，先更新 `go.mod`，必要时执行 `go mod tidy`。
- 遇到复杂查询可使用 `[]map[string]any` 构建 `and/or` 嵌套条件，并配合 `join`。
- 保持模块内的包路径引用一致，例如 `myproject/module/user/model`。

将本知识库作为系统提示或长上下文传入 AI，即可指导其按项目约定完成编码任务。
