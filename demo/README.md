# Demo: Admin Panel + Business CLI + awc

A complete demo showing how `awc` connects a browser login session to a
command-line tool.

## Architecture

```
Chrome (logged in to localhost:3000)
  ↓ chrome.cookies API
awc (reads session cookie)
  ↓ --header
demo/cli.js (business CLI)
  ↓ fetch with Cookie header
demo/server.js (Express admin panel)
```

The session cookie is **HttpOnly** — page JavaScript cannot read it. Only `awc`
(via the chrome.cookies extension API) can. This is the core value: awc bridges
the gap between browser sessions and CLI tools.

## Setup

```sh
# 1. Start the admin panel
cd demo
npm install
node server.js

# 2. In Chrome, open http://localhost:3000/login
#    Log in as admin / admin123

# 3. Configure awc login detection
awc auth:config demo-admin --url http://localhost:3000/login
# (interactive: detects the session cookie automatically)

# 4. Use the business CLI
node cli.js dashboard
node cli.js users
node cli.js status
```

## What each piece does

| Component | File | Role |
|---|---|---|
| Admin panel | `server.js` | Express server with login + 2 authenticated APIs |
| Business CLI | `cli.js` | Reads cookie via awc, calls APIs, formats output |
| Auth config | `~/.awc/auth/demo-admin.json` | Login detection (session cookie) |
| Agent skill | `.agents/skills/demo-admin/SKILL.md` | Lets AI agents query via natural language |

## API reference

```
GET /login            Login form (admin / admin123)
POST /login           Auth → Set-Cookie: session=<uuid>; HttpOnly
GET /                 Dashboard page (requires cookie)
GET /api/dashboard    JSON: { orders, revenue, pending, updatedAt, user }
GET /api/users        JSON: { users: [...], total }
GET /logout           Clear session
```

## Handling expired cookies

The session cookie expires after 24 hours (Max-Age=86400). When it expires:

```sh
awc auth:login demo-admin    # re-login in Chrome
node cli.js dashboard        # works again
```
