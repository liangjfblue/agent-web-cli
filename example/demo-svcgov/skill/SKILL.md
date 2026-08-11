---
name: demo-svcgov
description: "[demo] Query the demo service-governance platform through the demo-svcgov business CLI. Use when the user asks about service health, logs, database tables, table previews, or browser-login recovery for the demo service-governance system."
---

# demo-svcgov skill

Use the `demo-svcgov` CLI for service-governance data. The CLI acquires a
versioned browser session through `awc session:acquire`; do not call
`cookies:get` or parse awc's human-facing output yourself.

## Prerequisites

- Install the CLI: `cd example/demo-svcgov/cli && npm link`.
- Run the server: `cd example/demo-svcgov/server && npm install && node server.js`.
- Configure auth profile `svcgov` for `http://127.0.0.1:3001/login`.

## Command map

```text
service health overview -> demo-svcgov services
all logs                -> demo-svcgov logs
logs for one service    -> demo-svcgov logs --service <name>
logs by level           -> demo-svcgov logs --level <info|warn|error>
database table list     -> demo-svcgov tables
preview one table       -> demo-svcgov table <name>
check credentials       -> demo-svcgov status
interactive login       -> demo-svcgov login
replace rejected login  -> demo-svcgov login --refresh
```

## Login recovery

Run the requested business command first. Do not preflight with cookie
existence: a stale cookie can exist while the API rejects it.

- Exit `0`: answer from command output.
- Exit `2`: correct the command arguments; do not change login state.
- Exit `10` with `login required`: run `demo-svcgov login`.
- Exit `10` with `browser credentials were rejected`: run
  `demo-svcgov login --refresh`.
- Exit `11`: report that interactive login timed out.
- Exit `20`, `21`, `22`, or `30`: diagnose awc/profile/extension state with
  `awc sys:status`; do not treat it as an expired business login.

Before interactive login, tell the user that Chrome will open. The user must
complete any password, SSO, or CAPTCHA step. Set the command tool timeout to at
least 330 seconds. After login succeeds, retry the original command once.

Never display or persist passwords, cookies, tokens, awc session JSON, or
`cookieHeader`.

## Response rules

For service status, summarize healthy/degraded/down counts and affected
services. For logs, report the applied filters and relevant errors. For tables,
report names and row counts; for previews, identify the table and returned rows.
