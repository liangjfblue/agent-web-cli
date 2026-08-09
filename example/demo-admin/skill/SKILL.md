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

**When `demo-admin` outputs `✗ cookie 失效或未登录` (exit 1) — auto-poll mode:**

The goal is a hands-off recovery: tell the user to log in, then **poll the
real command until it succeeds or times out** — the user does not need to
come back and say "I'm logged in".

1. **STOP.** Do not call `awc auth:login demo-admin` yourself — it (without
   `--check`) opens a browser and **blocks up to 120 seconds**. That is the
   user's job, not the agent's. Auto-polling the cli command replaces this.

2. **Tell the user, in their language**, what happened and exactly what to do.
   Say you will auto-detect success so they need not reply:

   > 你的 demo-admin 登录已过期（cookie 失效）。
   >
   > 请在浏览器打开 http://localhost:3000/login，用 admin / admin123 登录。
   > 我会在后台自动检测，登录成功后立即继续查询，你无需再回复我。

3. **Poll the real command, not `auth:login --check`.** Re-run the original
   command (e.g. `demo-admin dashboard`) every ~5s, up to ~90s:
   - On success → return the result and note "已检测到登录成功".
   - On repeated `cookie 失效` → keep polling until the deadline.

   Why the real command and not `--check`? `--check` only tests whether a
   cookie *exists* in Chrome. A cookie can linger after logout or a server
   restart while the server rejects it → `--check` would falsely report
   "logged in". The actual API response (401 → the cli's `cookie 失效`
   message) is the source of truth.

4. **If the ~90s deadline passes with no success**, stop polling and escalate
   to manual troubleshooting. Tell the user:
   > 90 秒内仍未检测到登录。可能原因：
   > - 服务端重启过（内存 session 被清），请确认 demo server 在运行
   > - auth 配置错误，请检查 ~/.awc/auth/demo-admin.json 的
   >   loggedInWhen.cookie.url 是否为 http://localhost:3000
   > 排查后请告诉我，我再重试。

**Never let polling run unbounded.** The ~90s cap is mandatory — an agent
that polls forever hangs the session. When in doubt, escalate to the user.
