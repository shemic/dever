---
name: dever-framework-dev
description: Use when modifying the Dever framework itself under backend/dever, especially for config, server, middleware, load, orm, observe, log, auth, and CLI behavior.
---

# dever-framework-dev

## Overview

这个 skill 只用于：

- **开发 Dever 框架自身**

适用范围：

- `backend/dever/**`

不负责：

- `backend/module/**` 业务开发
- 项目层 page/config/middleware 业务逻辑

## When to Use

出现以下情况时使用：

- 修改 `backend/dever/config`
- 修改 `backend/dever/server`
- 修改 `backend/dever/middleware`
- 修改 `backend/dever/load`
- 修改 `backend/dever/orm`
- 修改 `backend/dever/observe`
- 修改 `backend/dever/log`
- 修改 `backend/dever/auth`
- 修改 `backend/dever/cmd/dever`

## Core Principle

框架层的目标是：

- 提供可复用能力
- 不写死业务规则
- 让业务层只保留薄装配

所以每次改框架都先判断：

1. 这是通用能力还是项目规则？
2. 这段逻辑是否真的应该上升到框架？
3. 是否会让业务层减少重复代码？
4. 是否会引入平行实现或额外复杂度？

## Mandatory Rules

1. 只改 `backend/dever/**` 时，不要为了习惯去跑：
   - `go run ./dever/cmd/dever init --skip-tidy`

2. 框架层优先最小改动，不为未来预埋无用层。

3. 能放在：
   - `util`
   - `observe`
   - `auth/jwt`
   - `middleware`
   - `orm`
   - `server`
   这些稳定落点的，不要塞到业务层。

4. 新框架能力必须优先考虑：
   - 配置入口
   - 默认值
   - 可插拔边界
   - 业务层如何薄装配

5. 不要把项目特有规则写进框架默认行为。

## Framework Boundary

### 应该进框架的

- 通用转换
- 通用 JWT 能力
- 结构化日志
- 统一观测
- ORM 公共能力
- 中间件执行链
- `dever run/install/init` 这类 CLI 能力
- CORS / HTTP / server 行为

### 不该进框架的

- 项目特有的 public routes
- dev 用户直通
- 项目自己的业务字段约定
- 业务层页面协议细节
- 模块专属 service/provider 逻辑

## Preferred Workflow

1. 先做静态审查  
   先定位是 bug、边界问题、还是重复代码。

2. 再定最小抽象  
   只在真正能减少业务层重复时上升到框架。

3. 再改框架  
   优先让业务层接入更薄，而不是再新增一层复杂包装。

4. 最后检查是否需要顺手收薄业务层  
   如果框架能力已经具备，但项目层还保留旧实现，就顺手收掉那层重复代码。

## Current Conventions

### Config

- 配置走 `backend/dever/config`
- `setting.jsonc` 支持 JSONC
- 新配置项要补默认值
- 新配置项要考虑旧配置兼容

### Log

- 统一走结构化 JSON
- 优先用：
  - `dlog.AccessFields(...)`
  - `dlog.ErrorFields(...)`

### Observe

- 自观测统一在 `backend/dever/observe`
- request/db 观测优先挂框架主链
- 外部 provider 走注册式扩展

### JWT

- JWT 多 scheme 能力统一在 `backend/dever/auth/jwt`
- 单 JWT 保持兼容 `config.auth.jwtSecret`
- 多 JWT 走 `config.auth.jwt.schemes + guards`

### ORM

- 优先修热路径、重复转换、统一执行器入口
- 不引入不必要第三方映射库

### CLI

- `dever run` 关注热重载稳定性和监听边界
- `dever build` 关注 release 打包默认值、目标推导和输出命名一致性
- `dever install` 关注脚本可用性和当前源码联动

### Current Build Convention

- 发布打包优先走 `dever build`
- 无参数默认打包项目根目录 `main.go`，输出 `server`
- `dever build cmd/workflow-worker` 自动打包 `cmd/workflow-worker/main.go`，输出 `workflow-worker`
- 默认 release 参数：
  - `CGO_ENABLED=0`
  - `GOOS=linux`
  - `GOARCH=amd64`
  - `-trimpath`
  - `-buildvcs=false`
  - `-ldflags="-s -w -buildid="`

## Reuse Before Writing

框架内部也先复用现有能力：

- `backend/dever/util`
- `backend/dever/observe`
- `backend/dever/auth/jwt`
- `backend/dever/log`
- `backend/dever/middleware`
- `backend/dever/orm`
- `backend/dever/server`

不要在框架内部再造第二套：

- 类型转换
- 结构化日志
- trace/span 上下文
- JWT token 读取
- DB observe 包装

## Common Review Checklist

改完框架后至少检查：

1. 有没有把业务规则写死进框架
2. 有没有新增平行实现
3. 配置是否有默认值和兼容路径
4. 业务层是否能因此删掉重复代码
5. 是否影响：
   - `config`
   - `server`
   - `middleware`
   - `observe`
   - `orm`
   - `load`
   - `cmd/dever`

## Common Mistakes

1. 把项目规则误抽成框架能力
2. 只改框架，不顺手收薄业务层旧逻辑
3. 新增配置但没有默认值
4. 框架层留死代码或半生效路径
5. 为了“灵活”引入过重抽象

## Done Criteria

完成前至少确认：

1. 这次改动确实属于 `backend/dever/**`
2. 改动减少了重复或修掉了确定性 bug
3. 没把业务规则写死进框架
4. 新能力有清晰落点
5. 如有必要，业务层已收薄
