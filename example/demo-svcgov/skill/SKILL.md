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

## Handling expired sessions

If `demo-svcgov` outputs `cookie 失效或未登录`, tell the user:

```
登录已过期，请运行: awc auth:login svcgov
```
