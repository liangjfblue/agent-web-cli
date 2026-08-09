---
name: demo-svcgov
description: "[demo] Query the demo service governance platform (services, logs, database) via the business CLI. Use when the user asks about service status, error logs, or database tables from the service governance system (e.g. \"gateway有没有报错\" or \"数据库有哪些表\")."
---

# svcgov skill

Query the service governance platform's APIs through the business CLI
(`demo-svcgov`), which reads cookies from Chrome via `awc`.

## Prerequisites

- The demo CLI must be installed globally: `cd example/demo-svcgov/cli && npm link`
- Server running: `cd example/demo-svcgov/server && npm install && node server.js`
- User logged in (if not: `awc auth:login svcgov`)

## Commands

Once linked, the `demo-svcgov` command is available from any directory:

```sh
# Service list — shows status (healthy/degraded/down), instances, version, CPU
demo-svcgov services

# Logs — filter by service and/or level
demo-svcgov logs                           # all logs
demo-svcgov logs --service gateway         # gateway logs only
demo-svcgov logs --level error             # errors only across all services
demo-svcgov logs --service gateway --level error  # gateway errors only

# Database
demo-svcgov tables                         # list tables
demo-svcgov table users                    # preview users table data
demo-svcgov table orders                   # preview orders table data
demo-svcgov table products                 # preview products table data

# Login check
demo-svcgov status
```

## Services in the platform

| Service | Typical use |
|---|---|
| gateway | API gateway, routing, rate limiting |
| user-service | User auth, profiles |
| order-service | Order processing (currently degraded!) |
| payment-service | Payment processing |
| notification | Email/push/SMS (currently down!) |

## How to answer common questions

- **"服务状态怎么样"** → `demo-svcgov services` → summarize healthy/degraded/down
- **"gateway有没有报错"** → `demo-svcgov logs --service gateway --level error`
- **"order-service怎么了"** → `demo-svcgov logs --service order-service` → look at warn/error
- **"数据库有哪些表"** → `demo-svcgov tables`
- **"看看users表的数据"** → `demo-svcgov table users`

## How it works

`demo-svcgov` reads the session cookie from Chrome via
`awc cookies:get --url http://localhost:3001 --header`, passes it as a
`Cookie` header to `fetch()`, and calls the platform's APIs. If the API
returns 401, the command prints `✗ cookie 失效或未登录` and exits.

## Handling login failures — agent behavior guide

**Default behavior: just call the command.** Do NOT pre-check login state or
run `awc auth:login` proactively — `demo-svcgov` already detects a bad session
and reports it. Pre-checking only adds latency; proactive login can block.

**When `demo-svcgov` outputs `✗ cookie 失效或未登录` (exit 1):**

1. **STOP.** Do not retry, do not loop, do not call `auth:login` yourself —
   `awc auth:login svcgov` (without `--check`) opens a browser and **blocks
   up to 120 seconds** waiting for the user to log in. That's the user's job,
   not the agent's.
2. **Tell the user, in their language**, what happened and exactly what to do.
   A good message answers four questions at once:

   > 你的 svcgov 登录已过期（cookie 失效）。
   >
   > 请在终端运行：
   > ```
   > awc auth:login svcgov
   > ```
   > 它会打开 http://localhost:3001/login，用 admin / admin123 登录即可。
   > 这一步可能需要你在浏览器里手动操作，最多等 120 秒。
   > 登录成功后告诉我，我会重新查询。

3. **Wait for the user to confirm.** Do not assume success, do not poll.
4. **After the user says they're logged in**, re-run the original command to
   verify and return the result.
5. **If it fails again** with the same error after a confirmed login, check
   the auth config — the `awc auth:config` command is known to sometimes
   produce a wrong `loggedInWhen.cookie.url` for localhost. Tell the user:
   > 仍然失败。请检查 ~/.awc/auth/svcgov.json，确认
   > loggedInWhen.cookie.url 为 http://localhost:3001（不是 https://localhost）。
   > 另外确认 demo-svcgov server 在运行。

**Why not rely on `awc auth:login --check`?**
`--check` only tests whether a cookie *exists* in Chrome — it cannot tell
whether the server still considers that session valid. A cookie can linger
after logout or a server restart while the server rejects it. Treat the actual
API response (401 → the cli's `cookie 失效` message) as the source of truth,
not `--check`.
