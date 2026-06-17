# 开发规范

这份规范适用于 Dever 应用项目里的所有业务代码：`module/*`、项目自有 `cmd/*`、middleware、worker、hook、脚本和 page JSON。

`backend/dever` 和 `backend/package/*` 是框架与可复用 package。应用项目开发时只参考、引入和复用它们，不修改它们的源码。只有用户明确要求维护 Dever 框架或某个 package 本身时，才把这些目录当作可编辑目标。

## 1. 先复用，再新增

写代码前先搜索：

- 同类 model / service / provider / api / page
- `dever/orm`
- `dever/load`
- `dever/server`
- `dever/util`
- `dever/log`
- `dever/observe`
- `package/front` 已有页面运行时能力

能复用现有实现就不新建平行实现。这里的“复用/扩展”是指使用框架公开能力、配置、hook、Provider、page JSON、model 元信息或在应用层组合；不是修改 `backend/dever` 或 `backend/package/*` 源码。

## 2. 控制重复

- 同一流程出现第二次，先考虑抽函数、配置或 service 方法。
- 同一流程出现第三次，必须抽公共路径。
- 同一结构只是字段不同，优先用参数或配置。
- 同一状态/类型分支反复出现，优先用映射或策略表。

不要为了少几行代码抽空层。抽象必须降低重复、复杂度或未来修改面。

## 3. 职责分清

- Model：表结构、字段注释、索引、Options、Relations。
- Service：业务规则、事务、状态流转、跨表编排。
- Provider：给 `Dever.Load` / page JSON 调用的适配层。
- API：取参、调用 Service、返回。
- Page JSON：后台页面结构、绑定和简单动作。
- Config：配置，不放业务数据。

不要把业务逻辑写进 API，不要把复杂业务塞进 page JSON，不要把页面文案和 option 重复写散。

## 4. 简单清晰

- 函数只做一件事。
- 优先早返回，减少深层嵌套。
- 文件有明确主题，不做垃圾桶。
- 不创建无意义 `BaseService`、`Manager`、`Helper`、单实现 interface。
- 不写只调用一次且不提升可读性的 wrapper。

## 5. 命名

- 名字表达业务意图。
- 不用 `data/item/thing/manager/util/helper` 做业务名。
- 文件名靠目录提供上下文，不重复父目录语义。
- Dever 协议名不能随意缩短：
  - `NewXxxModel`
  - `ProviderXxx`
  - `Get/Post/Put/DeleteXxx`

## 6. 后端边界

- 普通后台 CRUD 不写 API。
- API 必须薄。
- Service 方法用 `context.Context + 明确参数`。
- 外部调用要有超时、错误返回和日志。
- 状态流转、唯一创建、计数器要考虑事务、唯一索引、锁或幂等。
- 列表要考虑分页、索引、字段选择，避免 N+1。

## 7. Page JSON 边界

- 普通 CRUD 用 `Model + package/front + page JSON`。
- 复杂校验、规范化、跨表保存写 Service/Provider hook。
- 不发明节点、action、meta。
- 不复制整页复杂 JSON；按当前需求生成最小页面。
- 标准 list/update/create/view/detail/info 页优先自动推导 model。
- 能从 Model comment、Options、Relations 推导的标签和选项，不在 JSON 重复写。
- `status/sort` 这类列表维护字段优先使用 package/front 标准列表 action。

## 8. Provider / Service / API 边界

- Provider 只做模型生命周期、选项、page/load 适配；禁止空 passthrough Provider。
- Service 只做真实业务流程；禁止普通 CRUD wrapper。
- API 只做 HTTP 适配；禁止为了后台普通页面新增 CRUD API。
- API 里不写事务、外部调用和复杂业务，调用 Service 完成。
- 具体决策表见 `service-api.md`。

## 9. 文件和资源边界

- 配置、logo、favicon、AGENTS block、page 骨架从 `files/` 模板生成。
- 不修改 `package/front/html/assets/index*.js`、`front/dist` 等编译产物。
- 站点 logo 放 `config/front/assets/<site>/images/logo.svg`，默认透明背景。
- favicon 可带背景。

## 10. 缓存与性能边界

- 优先复用 `dever/cache`、现有 runtime cache 或已存在的 service 缓存，不在业务里造平行缓存。
- 适合缓存：配置、权限元数据、page/schema 解析结果、小型 option 结果。
- 不适合缓存：大列表、大分页、实时业务数据、带复杂筛选的用户数据。
- 写操作成功后必须主动失效相关缓存，不能只依赖 TTL 等待刷新。
- 缓存值是 `map` / `slice` 时，返回给请求前要 clone，避免请求间互相污染。
- 高频请求路径不要做目录扫描、全量同步、重复初始化；这类工作放启动预热、生成期或带失效机制的缓存里。

## 11. 清理检查

实现后检查并删除：

- 未使用函数、变量、import、文件。
- 临时日志、实验代码、废弃 TODO。
- 重复查询、重复校验、重复转换。
- 旧分支、旧配置、无意义 wrapper。

留下的代码必须满足至少一个条件：

- 当前功能真实使用。
- 消除了实际重复。
- 表达了清晰业务概念。
- 降低了复杂度或未来修改面。

## 12. 最终自查

交付前确认：

- 是否复用了现有实现？
- 是否没有重复逻辑？
- 是否职责清楚？
- 是否没有手改生成文件？
- 是否没有默认生成 CRUD API/Service？
- 是否没有把复杂业务塞进 JSON？
- 是否没有留下临时代码？
