# Migration

Use this guide to upgrade older Dever projects/components to the current skill rules without breaking existing behavior.

## Project Upgrade

1. Install or copy `skills/skills-dever`.
2. Add the managed Dever block to `AGENTS.md` / `CLAUDE.md` / tool-specific agent files.
3. Add `files/` templates if missing.
4. Keep existing pages and services working; do not mass-delete code.
5. On the next related edit, remove only the duplicate service/API/page config for that feature.
6. Run static audit when allowed.

## Page JSON Upgrade

When touching a page:

- add missing top-level `page/layout/nodes/data/state/action`.
- remove standard-page `_model/_use/submit.use` if inference works.
- move duplicated labels/options to model comments/Options/Relations.
- move `status/sort` out of update forms when package/front list action supports it.

Do not rewrite every page at once unless the user asks for a migration.

## Service/API Upgrade

When touching service/API:

- delete empty passthrough Providers.
- delete CRUD wrapper Services only after confirming page/front handles the behavior.
- keep real business Services.
- make API thin if it currently contains business logic.

## Component Upgrade

For complex components:

- add `package/<name>/skills/<name>/SKILL.md`.
- move component menu/auth/site metadata toward `dever.json`.
- keep old project-level front config until component install/update supports migration.
- document manual upgrade steps for projects still using old config style.

## Asset Upgrade

- move logo/favicon to `config/front/assets/<site>/images`.
- make sidebar/loading logo transparent.
- leave favicon with background if desired.
- stop editing compiled `package/front/html` assets.
