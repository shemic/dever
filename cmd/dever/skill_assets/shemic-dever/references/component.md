# Components, Packages, Modules, And Skills

In Dever, `module` and `package` are both components from an architecture perspective. `package` is reusable/distributable; `module` is project-local or a shim that imports a package.

## Ownership

- Application business code belongs in `module/<name>`.
- Reusable component code belongs in `package/<name>`.
- If `module/<name>/main.go` only contains `// dever:import my/package/<name>`, real source belongs to `package/<name>`.
- Do not copy package code into module to customize behavior. Use config, page JSON, Provider hooks, exposed Service/API, or component extension points.

## Component Skill

Complex components must carry their own skill:

```txt
package/bot/skills/bot/SKILL.md
package/user/skills/user/SKILL.md
package/<name>/skills/<name>/SKILL.md
```

Before editing a component, check for `skills/**/SKILL.md` in that component and read it. If no component skill exists, follow `shemic-dever` plus local examples.

Simple CRUD-only components may skip component skill. Components with custom service/API, multiple models, permissions, front plugins, external integrations, or unusual lifecycle should have one.

Declare bundled skills in component `dever.json` with paths relative to the component root:

```json
{
  "skills": [
    "skills/bot/SKILL.md"
  ]
}
```

Run `dever skill doctor` after adding or moving component skills. It validates active components only, so unused packages do not block a project.

## dever.json

Component metadata should travel with the component rather than requiring every project to rewrite `front.json`.

`dever.json` may describe:

- name/title/version
- dependencies
- auth/menu entries
- sites/pages
- public paths
- static assets
- install/update hooks when necessary
- bundled skills

Keep `dever.json` declarative. Do not put business logic in it.

## Dependencies

When installing a component:

- install missing dependencies first when safe.
- report dependency conflicts clearly.
- avoid removing shared dependencies while another component still needs them.

When uninstalling a component:

- check reverse dependencies.
- remove only component-owned menu/auth/page/static entries.
- leave user data removal as an explicit destructive operation.

## Front Assets And Plugins

Component front source belongs to:

```txt
package/<name>/front/src/plugin.ts
module/<name>/front/src/plugin.ts
```

Page JSON belongs to:

```txt
package/<name>/front/page/<page>/...
module/<name>/front/page/<page>/...
```

Do not edit global `front/src` for a component feature. Do not edit compiled plugin output in `front/dist`.

## Component Skill Skeleton

Use `scripts/component-skill.sh` or `files/component/skills/SKILL.md.tmpl` to create a component skill. It should include:

- component purpose
- source of truth files
- core models
- existing pages
- allowed Service/API cases
- forbidden shortcuts
- front plugin rules
- common errors
- migration notes
