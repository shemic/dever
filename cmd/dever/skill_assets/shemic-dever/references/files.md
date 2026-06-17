# Files, Config, And Assets

Use `skills/skills-dever/files` as the template and static resource source. Do not scatter large heredoc templates across scripts, and do not edit compiled assets for configuration or branding.

## Files Directory

Expected layout:

```txt
files/
  gitignore
  AGENTS.dever.md
  config/
    setting.jsonc.tmpl
    front.jsonc.tmpl
    front/assets/admin/images/logo.svg
    front/assets/admin/images/favicon.svg
    front/assets/work/images/logo.svg
    front/assets/work/images/favicon.svg
  go/
    main.go.tmpl
    middleware/readme.txt
    data/readme.txt
    package/readme.txt
  page/standard/
    list.json.tmpl
    update.json.tmpl
    detail.json.tmpl
  component/
    dever.json.tmpl
    skills/SKILL.md.tmpl
    skills/README.md.tmpl
```

Scripts may replace simple placeholders:

```txt
{{APP_NAME}}
{{PORT}}
{{MODULE_NAME}}
{{SITE_KEY}}
{{PAGE_NAME}}
{{RESOURCE_NAME}}
{{COMPONENT_NAME}}
{{COMPONENT_TITLE}}
```

Avoid complex template engines unless the current repo already uses one.

## Config

Generate config from templates:

- `config/setting.jsonc` from `files/config/setting.jsonc.tmpl`.
- `config/front.jsonc` from `files/config/front.jsonc.tmpl`.

If target config exists, do not overwrite by default. With `--force`, back up existing files first.

Rules:

- Do not put real secrets in templates.
- Use placeholders like `replace_me`.
- Logs default to files: `data/log/access.log`, `data/log/error.log`.
- Site config belongs in `config/front.json(c)`, not package/front source.
- Project runtime data belongs in `data/`, not `package/`.

## Logo And Favicon

Brand assets are site config assets:

```txt
config/front/assets/<site>/images/logo.svg
config/front/assets/<site>/images/favicon.svg
```

`config/front.json(c)` should reference them with relative paths:

```jsonc
{
  "sites": {
    "admin": {
      "assets": {
        "logo": "assets/images/logo.svg",
        "favicon": "assets/images/favicon.svg"
      }
    }
  }
}
```

Rules:

- `logo.svg` should normally be transparent background, suitable for sidebars and loading screens.
- `favicon.svg` may include its own background because it must stand alone in browser tabs.
- Do not change logo by editing `package/front/html/assets/index*.js`.
- Do not edit built files under `package/front/html` or plugin `front/dist`.
- Do not duplicate logo display code per site; use package/front site brand/runtime behavior.

## AGENTS Blocks

Use `files/AGENTS.dever.md` as a managed block for `AGENTS.md`, `CLAUDE.md`, `.codex/AGENTS.md`, and `.opencode/AGENTS.md`.

Do not overwrite whole files. Insert or replace only:

```md
<!-- dever-skill:start -->
...
<!-- dever-skill:end -->
```

The block must say that Dever tasks must read `shemic-dever` skill, and if skills are unavailable, manually read `skills/skills-dever/SKILL.md` and relevant references.

## Public And Uploaded Files

- Public static assets must live under the owning site/component path and be referenced through config/runtime.
- Uploaded files must go through package/front upload rules or a documented custom upload API.
- Validate size, extension, MIME where available, and storage target.
- Do not commit runtime uploads, logs, generated exports, or user data.
