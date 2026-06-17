# Front Page JSON

Use `package/front` page JSON for ordinary admin and simple site pages. Write React plugin nodes only when the page needs a genuinely custom interactive surface such as canvas, graph editor, rich workspace, or domain-specific live UI.

## Source Order

Check examples in this order:

1. Current `package/front/page` and `package/front/service/page`.
2. Current `package/bot/front/page`.
3. Current `package/user/front/page` if installed.
4. Current module/package pages in this project.
5. External demo only when the current project has no comparable example.

Do not use old project pages as the main pattern if current `package/front` already supports the feature.

## File Location And Path

Page route comes from file ownership and path:

```txt
package/front/page/admin/account/list.json       -> front/account/list
package/bot/front/page/admin/agent/list.json     -> bot/agent/list
module/work/front/page/work/home.json            -> work/home
```

The `{page}` directory under `front/page/{page}` is physical site isolation from `config/front.json.sites.*.page`; it is not automatically included in the route. Site system pages use `sites.*.api`:

- `api: "front"` reads `front/login` and `front/main`.
- `api: "work"` reads `work/login` and `work/main`.

When reusing admin shell for another site, create that site's own `main.json` and `login.json` instead of hardcoding `/front/auth/login`.

## Required Shape

Every page JSON keeps exactly these top-level objects, even when empty:

```json
{
  "page": {},
  "layout": {},
  "nodes": {},
  "data": {},
  "state": {},
  "action": {}
}
```

Missing `data`, `state`, or `action` commonly causes schema/runtime null errors.

## Model Inference

Standard page suffixes infer model from path:

- `list`
- `update`
- `create`
- `view`
- `detail`
- `info`

For standard pages, do not write:

- `data.table.list: "<<...NewXxxModel>>"`
- `data.form._model`
- `data.form._use`
- `action.submit.use`
- `<<NewXxxModel>>` in page JSON

If inference fails, fix model filename, `NewXxxModel`, ownership path, or page path. Do not hardcode the model as a shortcut.

Use explicit model only for non-standard pages such as `set`, `config`, fixed single-record pages, cross-resource embedded pages, or real custom flows.

## Model Metadata Is The Source

Put reusable metadata in model:

- `dorm:"comment:..."` for labels.
- `Options` for enums.
- `Relations` for relation selects.
- indexes and `Order` for list defaults.

Do not copy field labels/options into every page. Page JSON can override display text only when that page intentionally differs from the model meaning.

## Layout And Nodes

`layout` declares structure. `nodes` binds UI nodes to layout IDs. Do not invent node types; search `package/front` first.

Common nodes:

```txt
show-title, show-base, show-date, show-tag, show-table, show-button, show-page
form-input, form-textarea, form-number, form-switch, form-radio, form-checkbox, form-select, form-date
feedback-modal, feedback-drawer, feedback-confirm
nav-tab
app-site-brand, app-login-form, app-sidebar, app-topbar, app-outlet, app-assistant
```

Control selection:

- Fixed 2-4 item single enum: `form-radio`.
- Fixed small multi enum: `form-checkbox`.
- Many options, remote options, relation options, searchable options: `form-select`.
- Boolean or status toggle in list: `form-switch`.
- Sort/order inline list editing: `form-number` or existing list editor.

## List Pages

Standard list pages:

- Use `show-table` with remote data.
- Put search state under `data.search` or `data.table.searchFields`.
- Do not define `data.table.list`; runtime fetches it.
- Keep status/sort inline in the table when they are list-maintained fields.
- Do not add update forms only to flip status or sort.

Category-left + list-right pages must bind category model/data separately from the main table. The category selected ID may appear in search/query state, but the category list must not reuse another page's cached category state.

## Update/Create Pages

Standard update/create pages:

- Use inferred model and default submit.
- Do not hardcode `submit.use`.
- Do not include `status` or `sort` when those fields are list-maintained.
- Put only fields the user should edit.

Nested tables, child records, or embedded dialogs must declare their own page/action context. Do not loosen permissions to make child dialogs work.

## Action Rules

Use existing package/front actions for:

- list load/search/page
- update/create/delete
- status toggle
- sort edit
- option/relation loading
- import/export when supported

Do not hardcode `/front/route/action`; use current site runtime API prefix. Do not create new backend APIs for ordinary page actions.

When a page needs save-time normalization, validation, relation sync, or cross-table writes, add a Provider or Service according to `service-api.md`.

## Permission And Option Errors

For `暂无权限`, first check:

- current site access mode and auth state
- page `page.parent` and menu/auth registration
- action key inferred from path
- embedded dialog parent-child relation and action context
- public path only for pages that are genuinely public

For `option 无法推导模型`, first check:

- field path such as `form.service_id` maps to current form model
- model Options/Relations exist
- embedded row context has its own model
- action/dialog has correct page/action context

Do not fix these by granting wildcard permissions or writing broad direct-table actions.

## Front Plugin Boundary

Write `front/src/plugin.ts` only when page JSON cannot express the UI. Plugin source belongs to the owning package/module:

```txt
package/bot/front/src/plugin.ts
module/work/front/src/plugin.ts
```

Do not edit main `front/src` for a package/module feature unless maintaining the common front runtime itself.
