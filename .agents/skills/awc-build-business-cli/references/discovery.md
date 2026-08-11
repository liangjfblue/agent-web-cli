# Safe HTTP interface discovery

Use this reference only while identifying APIs for a requested business scope.

## Evidence order

Prefer evidence in this order:

1. Existing API documentation or the target application's source code.
2. Browser network records from page load and explicitly read-only UI actions.
3. A replayed read-only request using the generated CLI's session boundary.

Do not infer an endpoint solely from its URL or button label.

## Safe browser actions

Use page load, search/filter, pagination, sorting, tab changes, and detail views
freely when they are in scope. For create, edit, delete, export, authorize,
approve, trigger, retry, publish, send, or synchronize operations, prefer API
documentation, source code, an existing captured request, or an operation the
user performs deliberately.

Do not execute a mutation solely to learn its HTTP shape. If live execution is
necessary, identify the exact environment, target record, payload, and expected
side effect, then obtain the user's explicit approval for that concrete change.

When the UI does not expose enough evidence, stop and state what remains
unknown. Do not probe guessed endpoints on a live system, especially with a
mutation method.

## Record per command

Capture the following without recording credential values:

```text
CLI command:
HTTP method and URL:
Path/query parameters:
Auth header shape:
Response envelope:
Pagination fields:
Success condition:
Login rejection condition:
Other error condition:
Human output fields:
```

Distinguish HTTP authentication failures from application-level failures such
as `{ "code": 401 }` returned with HTTP 200. A cookie's existence proves only
that Chrome has it; a successful business API call proves that it is accepted.

## Scope review

Before implementing, compare the observed endpoints with the user's requested
scope. Exclude unrelated calls such as analytics, feature flags, notifications,
and navigation metadata unless a requested command requires them.
