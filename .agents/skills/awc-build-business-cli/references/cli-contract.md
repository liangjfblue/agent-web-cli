# Generated CLI contract

## Required commands

Every generated CLI must expose:

```text
<command> login [--refresh]
<command> help
```

Add only business commands observed and requested for the target system. Do not
generate placeholder `user`, `role`, or other resource commands. Commands may
query or mutate data according to the user's explicit scope.

## Session boundary

Acquire browser state with:

```text
awc session:acquire <profile> --url <base-url> --json
```

Use the returned `data.cookieHeader` only in memory. On exit `10`, print an
actionable interactive login command. When the business API rejects credentials
that still exist, instruct the user to run login with `--refresh`.

Interactive login must capture awc stdout so JSON credentials are never shown.
Allow at least 315 seconds in the CLI for a 300-second browser login window.

## Exit codes

Use these stable codes:

| Code | Meaning |
| --- | --- |
| `0` | Success |
| `1` | Business API or unexpected failure |
| `2` | Invalid command or arguments |
| `10` | Login required or browser credentials rejected |
| `11` | Interactive login timed out |
| `20` | Native host unavailable |
| `21` | Multiple browser profiles require selection |
| `22` | Selected browser profile is unavailable |
| `30` | Extension operation failed |

Preserve awc infrastructure codes when possible. Never collapse login-required
into a generic business error.

## Output

- Send data to stdout and errors to stderr.
- Support `--json` for every business query.
- Keep human output compact and stable enough for terminal use.
- Do not include raw request headers or the awc session document in output.
- Bound pagination and validate identifiers before sending a request.

## HTTP methods and mutation safety

Support `GET`, `HEAD`, `POST`, `PUT`, `PATCH`, and `DELETE`. Encode JSON request
bodies in the shared request helper and set `Content-Type` only when a body is
present. Accept empty `HEAD` and `204 No Content` responses. Reject unknown
methods before acquiring credentials.

For mutation commands, validate identifiers and required fields locally, state
the target and result clearly, and add focused tests for request method, path,
and payload. Do not add interactive confirmation prompts inside a CLI intended
for agent automation unless the business system requires them; control live
execution at the skill/tool layer instead.
