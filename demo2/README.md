# Demo2: Service Governance Platform + Business CLI + awc

A simulated service governance platform (like sysop/consul/apollo) with
services, logs, and database pages. The business CLI reads cookies via awc
to call authenticated APIs.

## Architecture

```
Chrome (logged in to localhost:3001)
  ↓ chrome.cookies API (HttpOnly cookie)
awc (reads session cookie)
  ↓ --header
demo2/cli.js (business CLI)
  ↓ fetch with Cookie header
demo2/server.js (Express service governance platform)
```

## Setup

```sh
cd demo2
npm install
node server.js

# In Chrome, open http://localhost:3001/login → admin / admin123

# Configure awc login detection
awc auth:config svcgov --url http://localhost:3001/login

# Use the business CLI
node cli.js services
node cli.js logs --service gateway --level error
node cli.js tables
node cli.js table users
```

## Platform pages

| URL | Page |
|---|---|
| `/login` | Login (admin / admin123) |
| `/` | Service list (status, instances, version, CPU, memory) |
| `/logs` | Log query (filter by service and level) |
| `/database` | Database tables (click to preview data) |

## APIs (all require session cookie)

```
GET /api/services              Service list with health status
GET /api/logs?service=&level=  Filtered logs
GET /api/database/tables       Table list
GET /api/database/:table       Table data preview
```

## CLI commands

```
node cli.js services                           List services + highlight issues
node cli.js logs --service gateway             Gateway logs
node cli.js logs --level error                 All error logs
node cli.js logs --service gateway --level err Filtered
node cli.js tables                             Database tables
node cli.js table users                        Preview users data
node cli.js status                             Check login
```
