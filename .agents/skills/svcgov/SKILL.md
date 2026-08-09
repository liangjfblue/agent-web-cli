---
name: demo-svcgov
description: "[demo] Query the demo service governance platform (services, logs, database) via the business CLI. Use when the user asks about service status, error logs, or database tables from the demo2 service governance system (e.g. \"gateway有没有报错\" or \"数据库有哪些表\")."
---

# svcgov skill

Query the service governance platform's APIs through the business CLI
(`demo2/cli.js`), which reads cookies from Chrome via `awc`.

## Prerequisites

- Server running: `cd demo2 && node server.js`
- User logged in (if not: `awc auth:login svcgov`)

## Commands

Commands (run from project root):

```sh
# Service list — shows status (healthy/degraded/down), instances, version, CPU
node demo2/cli.js services

# Logs — filter by service and/or level
node demo2/cli.js logs                           # all logs
node demo2/cli.js logs --service gateway         # gateway logs only
node demo2/cli.js logs --level error             # errors only across all services
node demo2/cli.js logs --service gateway --level error  # gateway errors only

# Database
node demo2/cli.js tables                         # list tables
node demo2/cli.js table users                    # preview users table data
node demo2/cli.js table orders                   # preview orders table data
node demo2/cli.js table products                 # preview products table data

# Login check
node demo2/cli.js status
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

- **"服务状态怎么样"** → `node demo2/cli.js services` → summarize healthy/degraded/down
- **"gateway有没有报错"** → `node demo2/cli.js logs --service gateway --level error`
- **"order-service怎么了"** → `node demo2/cli.js logs --service order-service` → look at warn/error
- **"数据库有哪些表"** → `node demo2/cli.js tables`
- **"看看users表的数据"** → `node demo2/cli.js table users`

## Handling expired sessions

If `cli.js` outputs `cookie 失效或未登录`, tell the user:

```
登录已过期，请运行: awc auth:login svcgov
```
