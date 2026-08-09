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
3. If the API returns 401, the command prints `✗ cookie 失效或未登录` and exits.

The session cookie is HttpOnly — page JavaScript cannot read it. Only `awc`
(via chrome.cookies API) can read it. This is the core value of awc: reusing
login sessions that are invisible to page-level tools.

## Handling login failures — agent behavior guide

**Default behavior: just call the command.** Do NOT pre-check login state or
run `awc auth:login` proactively — `demo-admin` already detects a bad session
and reports it. Pre-checking only adds latency; proactive login can block.

**When `demo-admin` outputs `✗ cookie 失效或未登录` (exit 1):**

1. **STOP.** Do not retry, do not loop, do not call `auth:login` yourself —
   `awc auth:login demo-admin` (without `--check`) opens a browser and **blocks
   up to 120 seconds** waiting for the user to log in. That's the user's job,
   not the agent's.
2. **Tell the user, in their language**, what happened and exactly what to do.
   A good message answers four questions at once:

   > 你的 demo-admin 登录已过期（cookie 失效）。
   >
   > 请在终端运行：
   > ```
   > awc auth:login demo-admin
   > ```
   > 它会打开 http://localhost:3000/login，用 admin / admin123 登录即可。
   > 这一步可能需要你在浏览器里手动操作，最多等 120 秒。
   > 登录成功后告诉我，我会重新查询。

3. **Wait for the user to confirm.** Do not assume success, do not poll.
4. **After the user says they're logged in**, re-run the original command to
   verify and return the result.
5. **If it fails again** with the same error after a confirmed login, the
   server may have restarted (clearing its in-memory session). Tell the user:
   > 仍然失败。服务端可能重启过，导致新登录的 session 也未被识别。
   > 请确认 demo server 在运行，并检查 ~/.awc/auth/demo-admin.json 的
   > loggedInWhen.cookie.url 是否为 http://localhost:3000。

**Why not rely on `awc auth:login --check`?**
`--check` only tests whether a cookie *exists* in Chrome — it cannot tell
whether the server still considers that session valid. A cookie can linger
after logout or a server restart while the server rejects it. Treat the actual
API response (401 → the cli's `cookie 失效` message) as the source of truth,
not `--check`.
