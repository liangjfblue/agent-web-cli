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
`awc cookies:get --url http://127.0.0.1:3001 --header`, passes it as a
`Cookie` header to `fetch()`, and calls the platform's APIs. If the API
returns 401, the command prints `✗ cookie 失效或未登录` and exits.

## Handling login failures — agent behavior guide

**Default behavior: just call the command.** Do NOT pre-check login state or
run `awc auth:login` proactively — `demo-svcgov` already detects a bad session
and reports it. Pre-checking only adds latency; proactive login can block.

**When `demo-svcgov` outputs `✗ cookie 失效或未登录` (exit 1) — auto-poll mode:**

The goal is a hands-off recovery: tell the user to log in, then **poll the
real command until it succeeds or times out** — the user does not need to
come back and say "I'm logged in".

1. **STOP.** Do not call `awc auth:login svcgov` yourself — it (without
   `--check`) opens a browser and **blocks up to 120 seconds**. That is the
   user's job, not the agent's. Auto-polling the cli command replaces this.

2. **Tell the user, in their language**, what happened and exactly what to do.
   Say you will auto-detect success so they need not reply:

   > 你的 svcgov 登录已过期（cookie 失效）。
   >
   > 请在浏览器打开 http://127.0.0.1:3001/login，用 admin / admin123 登录。
   > 我会在后台自动检测，登录成功后立即继续查询，你无需再回复我。

3. **Poll the real command, not `auth:login --check`.** Re-run the original
   command (e.g. `demo-svcgov services`) every ~5s, up to ~90s:
   - On success → return the result and note "已检测到登录成功".
   - On repeated `cookie 失效` → keep polling until the deadline.

   Why the real command and not `--check`? `--check` only tests whether a
   cookie *exists* in Chrome. A cookie can linger after logout or a server
   restart while the server rejects it → `--check` would falsely report
   "logged in". The actual API response (401 → the cli's `cookie 失效`
   message) is the source of truth.

4. **If the ~90s deadline passes with no success**, stop polling and escalate
   to manual troubleshooting. The `awc auth:config` command is known to
   sometimes produce a wrong `loggedInWhen.cookie.url` for localhost. Tell
   the user:
   > 90 秒内仍未检测到登录。可能原因：
   > - 服务端重启过（内存 session 被清），请确认 demo-svcgov server 在运行
   > - auth 配置错误，请检查 ~/.awc/auth/svcgov.json 的
   >   loggedInWhen.cookie.url 是否为 http://127.0.0.1:3001（auth:config 有时
   >   会错误生成 https://localhost）
   > 排查后请告诉我，我再重试。

**Never let polling run unbounded.** The ~90s cap is mandatory — an agent
that polls forever hangs the session. When in doubt, escalate to the user.
