---
name: shemic-dever
description: This skill should be used when working on Dever Go projects; when editing model, service, provider, api, page JSON, front plugins, package/module components, config/front.json, Dever CLI behavior, generated route/load files, package/front admin pages, package/bot, package/user, or debugging Dever registration, permission, page, option, logo, install, build, run, and plugin errors.
version: 1.0.0
---

# shemic-dever

Use this skill to develop Dever projects without bypassing framework capabilities. Keep Dever code boring: inspect current project structure, reuse existing package/front behavior, write the smallest required model/page/service code, and avoid generated-file or compiled-asset edits.

## Read First

Load only the references needed for the current task:

- `references/workflow.md`: decision order and task routing.
- `references/framework.md`: Dever CLI, generated files, route/model/service registration, middleware.
- `references/development.md`: reuse, naming, responsibility boundaries, cleanup.
- `references/model.md`: model files, comments, options, relations, indexes.
- `references/front-page.md`: package/front page JSON rules, automatic inference, admin/site pages.
- `references/service-api.md`: when Provider, Service, and API are allowed.
- `references/files.md`: config templates, logo/favicon, AGENTS blocks, static files.
- `references/component.md`: package/module components, component skills, dever.json.
- `references/package-plugin.md`: package/module front plugin source and build outputs.
- `references/security.md`: permission, action, public route, secret, upload safety.
- `references/troubleshooting.md`: common Dever errors and first checks.
- `references/migration.md`: old project and old component upgrade guidance.

Legacy aliases exist for old links: `references/page.md` points to `front-page.md`, and `references/module.md` points to `service-api.md`.

## Task Routing

- Empty project, install, new site, admin/work/site setup: read `workflow.md`, `framework.md`, `files.md`, `component.md`.
- Backend model or table change: read `model.md`, then `front-page.md` if a page is involved.
- Admin page, CRUD, list/update/detail, category/list page, permission or option error: read `front-page.md`, `model.md`, `security.md`.
- Provider, Service, API, callback, login, register, workflow, external request, transaction: read `service-api.md`, `security.md`.
- Config, logo, favicon, AGENTS, static assets, public files: read `files.md`.
- package/front, package/bot, package/user, module/package component: read `component.md`; then read the component's own `skills/**/SKILL.md` if present.
- Front plugin or React node: read `package-plugin.md`; use page JSON unless a real custom interactive node is required.
- Dever CLI, generated files, import cycle, run/build/install issue: read `framework.md`, `troubleshooting.md`.

## Hard Rules

- Do not edit generated files: `data/router.go`, `data/load/model.go`, `data/load/service.go`, `data/table/*.json`.
- Do not edit compiled front assets such as `package/front/html/assets/index*.js` to change behavior, logo, text, or style.
- Application projects use Go module path `my`; do not rename it to project, domain, or directory names.
- Ordinary admin CRUD is `Model + package/front + page JSON`; do not write CRUD API or CRUD Service.
- Standard page paths infer model automatically; do not hardcode `_model`, `_use`, `<<NewXxxModel>>`, or `submit.use` in standard list/update/create/view/detail/info pages.
- Model comments, Options, and Relations are the source for labels, enum options, and relation options. Do not duplicate them across page JSON unless overriding display text intentionally.
- `status`, `sort`, and simple list-maintained fields use front standard list actions; do not add update-form services for them.
- Provider is only for real page/load hooks, validation, normalization, save lifecycle, or adaptation. Do not create empty passthrough Provider methods.
- Service is only for real business workflows: transactions, state transitions, external calls, async orchestration, cross-table rules. Do not create CRUD wrapper services.
- API is only for real HTTP capabilities: login, register, callbacks, external-facing endpoints, workflow actions, complex frontend interactions. Keep API thin.
- Config, logo, favicon, AGENTS snippets, page skeletons, and component skill skeletons come from `files/`; do not scatter heredoc templates or hardcoded assets in scripts.
- Before changing `package/<name>` or `module/<name>`, inspect whether `package/<name>/skills/**/SKILL.md` or `module/<name>/skills/**/SKILL.md` exists and follow it.
- Current local code is the source of truth. Reference order: current `package/front`, current `package/bot`, current `package/user`, current module/package examples, `backend/dever`; external demo is fallback only.
- If the user forbids build/test, do not run `npm run build`, `dever build`, `dever front build`, `go test`, or equivalent tests. Static text audits are allowed only when useful.

## Required Workflow

1. Inspect existing files with `rg` or `find` before designing code.
2. Identify whether the task can be solved by model metadata, page JSON, existing package/front action, Provider hook, Service, API, front plugin, config, or static template.
3. Choose the lowest-power mechanism that satisfies the requirement.
4. Keep changes close to the owning module/package.
5. Remove duplicate or temporary code after implementation.
6. Run static audit when allowed and useful:

```bash
bash skills/skills-dever/scripts/audit.sh <changed-file-or-dir>
```

7. In the final response, list changed files, affected model/page/service/api/component, generated files touched or not touched, and verification run or intentionally skipped.
