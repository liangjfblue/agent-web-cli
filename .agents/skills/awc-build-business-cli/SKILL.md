---
name: awc-build-business-cli
description: Generate a business-specific CLI and companion skill that reuse a signed-in Chrome session through agent-web-cli. Use when the user asks to wrap a management console or business website's HTTP APIs as a CLI, create a CLI plus skill for the current logged-in site, or scaffold another awc-based business integration. Generate query or mutation commands according to the user's explicit scope, preserve browser credential secrecy, and verify login recovery and generated commands.
---

# Build an awc business CLI

Generate a small business CLI and its companion skill. Keep `awc` as the
browser-session boundary and put business semantics in the generated project.

## Guardrails

- Generate only the operations the user requests. Do not silently broaden a
  query-only request into create, update, delete, authorization, or trigger
  capabilities.
- Separate implementation from live execution. A requested mutation may be
  implemented, but do not execute it merely to discover or smoke-test an API.
  Confirm the exact target and expected impact before changing live data.
- Never print, log, persist, or place Cookie, token, JWT, `cookieHeader`, or
  password values in source, tests, fixtures, diagnostics, or skill content.
- Keep auth-profile files limited to cookie names, URLs, selectors, and timeout
  metadata. They must not contain cookie values.
- Call `awc` with an argument array, never through a shell command string.
- Do not commit, push, publish, or install globally unless the user requests it.

## Workflow

### 1. Establish scope

Identify the target site, desired CLI name, output directory, and requested
business operations. Treat the current signed-in browser tab as the target when
the user says "this site" or equivalent. If command scope is omitted, inspect
the requested page but ask before choosing a materially broader scope.

Inspect the destination repository and its local instructions before editing.
Preserve unrelated worktree changes.

### 2. Verify awc and login configuration

Run `awc sys:status`. If awc or the extension is unavailable, guide the user
through `awc sys:setup` before continuing.

Use an existing auth profile when one matches the target. If no profile exists,
use the bundled `awc-auth-config` skill when available, or follow its verified
cookie-difference workflow. Do not guess the authentication cookie.

Use `awc session:acquire <profile> --url <base-url> --json` as the generated
CLI's only credential boundary. Do not use raw `cookies:get` output in generated
CLI code.

### 3. Discover only the requested HTTP APIs

Read [references/discovery.md](references/discovery.md) before inspecting
network traffic. Prefer existing API documentation or source code. Otherwise,
capture requests produced by page operations within the requested scope. For a
mutation, prefer observing a user-performed operation or using disposable test
data in a test environment over changing live data for discovery.

For every proposed command, record method, URL, query parameters, auth mapping,
response envelope, pagination, application-level error fields, and login
rejection signals. Verify uncertain observations with a safe call that does not
introduce an unapproved side effect.

### 4. Scaffold the generated project

Follow the destination repository's existing language and CLI conventions when
adding to an established codebase. For a new standalone CLI, use the bundled
Node 18 scaffold after the login cookie has been identified. Verify `node` is
available before running it:

```text
node <this-skill>/scripts/scaffold.mjs \
  --name <project-slug> \
  --command <executable-name> \
  --base-url <business-base-url> \
  --login-url <login-url> \
  --cookie-url <cookie-origin-url> \
  --cookie-name <login-cookie-name> \
  --output <destination>
```

The scaffold intentionally contains no business resources or commands. It
provides only login handling, HTTP plumbing for common methods, output helpers,
tests, and a companion-skill skeleton.

### 5. Implement business commands

Read [references/cli-contract.md](references/cli-contract.md). Implement only
the commands requested by the user in `commands.js`. Update `printBusinessHelp`
and the generated tests at the same time. For mutations, validate identifiers
and required fields before sending the request and make the effect explicit in
help and output.

Keep endpoint paths, response mapping, status labels, pagination, and
application-level error rules in `commands.js`; do not move business-specific
knowledge into this builder skill or `awc`.

Adapt `buildAuthHeaders` only when the API does not accept the normal Cookie
header. Extract a named cookie in memory and never return or display it.

### 6. Complete the companion business skill

Read [references/business-skill-contract.md](references/business-skill-contract.md).
Replace every TODO in the generated `skill/SKILL.md`, document the exact command
map, and encode login recovery by exit code. Keep the business skill thin: it
should select commands, translate arguments, recover login, retry once, and
summarize results.

### 7. Verify end to end

Run all of the following that apply:

1. `node --check cli.js` and `node --check commands.js`.
2. `npm test` in the generated project.
3. Search generated files for template markers and credential-like values.
4. Run query commands against the live API; test mutations with mocks, a dry-run
   endpoint, or disposable test data unless the user explicitly authorizes the
   exact live change.
5. Test missing login, API-rejected credentials, interactive login, and one
   retry of the original command.
6. Validate the generated business skill with the available skill validator.

Report generated paths, supported commands, verification performed, and any
manual login or CAPTCHA step still required.
