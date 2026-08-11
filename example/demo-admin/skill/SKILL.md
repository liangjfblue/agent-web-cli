---
name: demo-admin
description: "[demo] Query the demo admin panel through the demo-admin business CLI. Use when the user asks about dashboard data, order counts, revenue, user lists, or browser-login recovery for the demo admin system."
---

# demo-admin skill

Use the `demo-admin` CLI for demo admin data. The CLI acquires a versioned
browser session through `awc session:acquire`; do not call `cookies:get` or
parse awc's human-facing output yourself.

## Prerequisites

- Install the CLI: `cd example/demo-admin/cli && npm link`.
- Run the server: `cd example/demo-admin/server && npm install && node server.js`.
- Configure auth profile `demo-admin` for `http://localhost:3000/login`.

## Command map

```text
dashboard totals and revenue -> demo-admin dashboard
list users                   -> demo-admin users
check browser credentials    -> demo-admin status
interactive login            -> demo-admin login
replace rejected credentials -> demo-admin login --refresh
```

## Login recovery

Run the requested business command first. Do not preflight with cookie
existence: a stale cookie can exist while the API rejects it.

- Exit `0`: answer from command output.
- Exit `2`: correct the command arguments; do not change login state.
- Exit `10` with `login required`: run `demo-admin login`.
- Exit `10` with `browser credentials were rejected`: run
  `demo-admin login --refresh`.
- Exit `11`: report that interactive login timed out.
- Exit `20`, `21`, `22`, or `30`: diagnose awc/profile/extension state with
  `awc sys:status`; do not treat it as an expired business login.

Before interactive login, tell the user that Chrome will open. The user must
complete any password, SSO, or CAPTCHA step. Set the command tool timeout to at
least 330 seconds. After login succeeds, retry the original command once.

Never display or persist passwords, cookies, tokens, awc session JSON, or
`cookieHeader`.

## Response rules

Summarize dashboard order, revenue, pending, update-time, and current-user
fields. For user lists, report the total and useful identity/role fields. State
clearly when no records match.
