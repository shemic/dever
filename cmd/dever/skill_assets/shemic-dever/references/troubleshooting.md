# Troubleshooting

Start with the owning layer. Do not patch symptoms by loosening permissions or hardcoding model/action names.

## `model 未注册`

Check:

- model file is under `module/<name>/model` or `package/<name>/model`.
- file has exactly one zero-arg `NewXxxModel`.
- filename matches `NewXxxModel`.
- constructor does not panic.
- generated `data/load/model.go` was refreshed by `dever init`/`dever run`.

Do not hand-edit `data/load/model.go`.

## `暂无权限`

Check:

- logged-in account and site access mode.
- page path and menu/auth registration.
- action key inferred from page path.
- child dialog/table action context.
- custom API route auth/public config.
- component `dever.json` menu/auth entries if component metadata is used.

Do not fix with wildcard permission unless the user explicitly wants an unsafe admin-only shortcut.

## `option 无法推导模型`

Check:

- field path belongs to the current form/table model.
- model has Options or Relations for the field.
- embedded row/dialog has the correct model context.
- page does not accidentally reuse another model's category/search state.
- standard page is not hardcoding `_model/_use` incorrectly.

## Page Schema Null Errors

Check top-level objects:

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

## Import Cycle

Check package/service dependencies. Common cause: package/front service package imports a subpackage that imports package/front service again.

Fix by moving shared small helpers to a lower-level package or inverting dependency through narrow interfaces. Do not add another wrapper package that still imports both sides.

## Logo Black Background

Check:

- `config/front/assets/<site>/images/logo.svg` is transparent if used as sidebar/loading logo.
- `favicon.svg` can have background.
- `config/front.json(c).sites.<site>.assets.logo` points to the intended file.
- no compiled `package/front/html/assets/index*.js` was edited.
- front source is not wrapping the logo in a fixed dark icon container unless intentionally designed.

## Front Plugin Not Loaded

Check:

- `front/src/plugin.ts` exists under owning package/module.
- page JSON uses the registered node type.
- plugin dev server is running only when `dever run` starts it.
- built output is not manually edited.

## Generated Route Missing

Check:

- API method is exported and named `Get/Post/Put/DeleteXxx`.
- receiver type is exported.
- file is under `api`.
- `dever routes` or `dever run` refreshed generated routes.

Do not hand-edit `data/router.go`.
