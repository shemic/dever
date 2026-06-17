# Security And Permission

Keep front convenience features behind explicit server-side permission checks. Do not trade security for fewer page JSON lines.

## Permission Defaults

- API routes require authentication unless explicitly configured public.
- Page actions inherit the current site access mode.
- Public pages and routes need a documented reason.
- Embedded dialogs and child tables must keep the parent action context; do not grant wildcard access to make them work.

## Generic Actions

Generic page actions may be convenient, but they must not update arbitrary tables or fields from client-provided input.

Required safeguards:

- action key resolved from page/model registry
- permission check against current site/user/role
- allowed model/action/field list from server metadata
- server-side field filtering
- no direct client-controlled table name
- no direct client-controlled SQL fragments
- audit/logging for sensitive mutations

Auto-inferred actions follow the same rules. Empty action config does not mean no permission check.

## Secrets

- Do not put real JWT secrets, API keys, database passwords, or provider tokens in templates or source.
- Use `replace_me` placeholders in files templates.
- Mask secrets in logs and error messages.
- Never return provider credentials to page runtime.

## Uploads

- Validate file size.
- Validate extension and MIME where available.
- Store outside source directories.
- Do not allow path traversal.
- Do not expose uploaded private files through public static routes.

## External Endpoints

For public callbacks/webhooks:

- validate signature or token when provider supports it.
- make processing idempotent.
- keep retry-safe state transitions.
- log request IDs, not secrets.

## CORS

Development may use permissive CORS. Production config should restrict origins and credentials. Do not hardcode broad CORS in package code.
