---
name: demo-admin
description: Query the demo admin panel (orders, revenue, users) via the business CLI. Use when the user asks about dashboard data, order counts, revenue, or user lists from the demo admin system (e.g. "查一下后台数据" or "how many orders today").
---

# demo-admin skill

Query the demo admin panel's APIs through the business CLI (`demo/cli.js`),
which reads cookies from Chrome via `awc` and calls the authenticated APIs.

## Prerequisites

- The demo server must be running: `cd demo && node server.js`
- The user must be logged in (if not, run `awc auth:login demo-admin`)

## Commands

All commands run from the `demo/` directory:

```sh
# Dashboard: orders, revenue, pending count
node cli.js dashboard

# User list
node cli.js users

# Login status check
node cli.js status
```

## How it works

1. `cli.js` calls `awc cookies:get --url http://localhost:3000 --header` to
   read the session cookie from Chrome.
2. The cookie is passed as a `Cookie` header to `fetch()`.
3. If the API returns 401, the user is told to run `awc auth:login demo-admin`.

The session cookie is HttpOnly — page JavaScript cannot read it. Only `awc`
(via chrome.cookies API) can read it. This is the core value of awc: reusing
login sessions that are invisible to page-level tools.

## Handling expired sessions

If `cli.js` outputs `cookie 失效或未登录`, tell the user:

```
你的登录已过期，请运行: awc auth:login demo-admin
```

After re-login, retry the command.
