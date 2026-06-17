# Provider, Service, And API

Use the lowest-power backend mechanism that satisfies the requirement. Most admin work needs no custom backend beyond model metadata and page JSON.

## Decision Table

| Requirement | Use | Do not use |
| --- | --- | --- |
| ordinary list/create/update/delete/detail | Model + page JSON | custom API/Service |
| labels/options/relations | Model comments/Options/Relations | duplicated page labels/options |
| default field value | model default or page default | Service |
| simple save normalization | ProviderBeforeSave | API |
| save-time validation tied to one model | ProviderBeforeSave | page-only checks |
| after-save relation sync/count/cache invalidation | ProviderAfterSave or focused Service | API-only logic |
| state transition with business rules | Service | direct status update |
| cross-table transaction | Service | page JSON action |
| external HTTP/provider call | Service | API with inline business logic |
| callback/webhook/login/register | API + Service | page JSON |
| async job/workflow/run action | Service, optional API trigger | CRUD wrapper |
| complex custom front interaction | API + Service when needed | generic table action |

## Provider Rules

Provider adapts Dever/page runtime to a model lifecycle or option/data hook.

Allowed:

- `ProviderBeforeSaveXxx` for validation, normalization, derived fields.
- `ProviderAfterSaveXxx` for relation sync, count update, cache invalidation.
- option/list providers when the data cannot come from model Options/Relations.
- small adapters that call a real Service method.

Forbidden:

- empty passthrough Provider returning input unchanged.
- Provider only because a script generated one.
- Provider containing long business workflows, HTTP clients, or transaction orchestration.
- Provider names guessed by hand without checking generated registration rules.

## Service Rules

Service owns business behavior. It must have a business verb and a real invariant.

Good method names:

```txt
Publish
Archive
RunNow
CreateVersion
AssignRole
SyncRelation
RotateToken
ImportRows
ExecuteWorkflow
```

Bad method names for ordinary CRUD wrappers:

```txt
Save
List
Create
Update
Delete
GetInfo
HandleData
Process
```

Service should:

- accept `context.Context` or explicit request context as project convention requires.
- take typed/clear parameters where possible.
- own transaction boundaries for multi-table changes.
- return explicit errors.
- apply timeouts for external calls.
- log meaningful failures without leaking secrets.
- be reusable from Provider/API/jobs.

Service should not:

- parse HTTP request directly.
- know page JSON layout details.
- wrap a single ORM call without adding business value.
- become a dumping ground for unrelated helpers.

## API Rules

API is the HTTP adapter. Keep it thin:

1. Read and validate request parameters.
2. Call Service.
3. Shape response.

Allowed APIs:

- login/register/logout/refresh token
- public callbacks/webhooks
- third-party endpoints
- workflow/action trigger endpoints
- custom front plugin endpoints
- file upload/download endpoints not covered by package/front

Forbidden APIs:

- CRUD API for pages already handled by package/front.
- API with inline transaction/business logic.
- API that bypasses permission checks to mutate arbitrary table/field.
- broad "execute action" endpoints without action registry and permission checks.

## Status, Sort, And Inline Updates

`status`, `sort`, and similar list-maintained fields use package/front standard list actions when possible. Do not add Provider/Service/API just to toggle a status or update sort.

Use Service for status only when the status represents a real business state transition with preconditions, side effects, audit, or locking.

## External Calls

For HTTP/LLM/storage/provider calls:

- set timeout.
- keep credentials in config or secret storage, not source.
- mask secrets in logs.
- make retries explicit and bounded.
- make idempotency explicit for callbacks and async jobs.

## Permission Boundary

All custom API and action-like service entrypoints must be reachable only through an authenticated and authorized route unless explicitly public. Public endpoints need a documented reason in code/page/config.
