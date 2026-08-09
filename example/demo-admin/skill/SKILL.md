---
name: demo-admin
description: "[demo] Query the demo admin panel (orders, revenue, users) via the business CLI. Use when the user asks about dashboard data, order counts, revenue, or user lists from the demo admin system (e.g. \"查一下后台数据\" or \"how many orders today\")."
---

# demo-admin skill

Query the demo admin panel's APIs through the business CLI (`demo-admin`),
which reads cookies from Chrome via `awc` and calls the authenticated APIs.

## Prerequisites

- The demo CLI must be installed globally: `cd example/demo-admin/cli && npm link`
- The demo server must be running: `cd example/demo-admin/server && npm install && node server.js`
- The user must be logged in (if not, run `awc auth:login demo-admin`)

## Commands

Once linked, the `demo-admin` command is available from any directory:

```sh
# Dashboard: orders, revenue, pending count
demo-admin dashboard

# User list
demo-admin users

# Login status check
demo-admin status
```

## How it works

1. `demo-admin` calls `awc cookies:get --url http://localhost:3000 --header` to
   read the session cookie from Chrome.
2. The cookie is passed as a `Cookie` header to `fetch()`.
3. If the API returns 401, the user is told to run `awc auth:login demo-admin`.

The session cookie is HttpOnly — page JavaScript cannot read it. Only `awc`
(via chrome.cookies API) can read it. This is the core value of awc: reusing
login sessions that are invisible to page-level tools.

## Handling expired sessions

If `demo-admin` outputs `cookie 失效或未登录`, tell the user:

```
你的登录已过期，请运行: awc auth:login demo-admin
```

After re-login, retry the command.
