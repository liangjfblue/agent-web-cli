---
name: awc-auth-config
description: Configure login automation for a website using agent-web-cli (awc). Use when the user wants to set up auto-login / login-state detection for a site (e.g. "帮我配一下 sysop 登录" or "set up login for my-site.com"). The skill inspects the target site via awc commands, identifies the login-state cookie, and writes an auth config to ~/.awc/auth/<name>.json, then validates it.
---

# awc-auth-config

Configure `awc auth:login` for a website by inspecting it live, then writing
and validating a config file. The user never needs to guess cookie names or
CSS selectors — the AI figures them out.

## Prerequisites

- `awc` must be on PATH and connected (`awc sys:status` shows the extension).
- Chrome must be running with the Agent Web CLI extension loaded.

If `awc` is not set up, tell the user to run `awc sys:setup` first.

## What you are producing

One file: `~/.awc/auth/<name>.json` in this schema:

```json
{
  "loginUrl": "https://example.com/login",
  "loggedInWhen": {
    "cookie": { "url": "https://example.com", "name": "sessionid" }
  }
}
```

The **cookie condition** is the preferred login-state signal — it is reliable
(uses `chrome.cookies.get`, not DOM guessing). Only fall back to URL/button
heuristics if no suitable cookie exists.

`name` is the short identifier the user will pass to `awc auth:login <name>`.
Use the site's short name: `sysop`, `github`, `my-portal`, etc.

## Procedure

Work through these steps. Each step uses `awc` to inspect the real browser
state — do NOT guess; always verify with live data.

### Step 1 — Ask the user for the site

You need two things from the user:
1. **The login page URL** (e.g. `https://s.sysop.yy.com/`). If they only know
   the site name, open it and find the login page yourself.
2. **A short name** for the config (e.g. `sysop`). If they don't specify one,
   derive it from the domain.

### Step 2 — Open the site and check current login state

```sh
awc tabs:open <loginUrl>
```

Then check if the user is already logged in. Look at two things:

```sh
# What cookies does this domain have?
awc cookies:get --url <siteUrl> --json

# What does the page look like? (buttons, text)
awc dom:snapshot --url <loginUrl>
```

### Step 3 — Identify the login-state cookie

This is the key analysis step. Look at the cookie list from step 2 and find a
cookie that reliably indicates "logged in". Good candidates, in priority:

| Pattern | Why it's good | Examples |
|---|---|---|
| Named `logged_in`, `is_login`, `auth` | Designed as a login flag | GitHub: `logged_in=yes` |
| Session IDs that only exist when logged in | Absent for anonymous users | `sessionid`, `JSESSIONID`, `ASP.NET_SessionId` |
| User identifiers | Only set post-login | `user_session`, `dotcom_user`, `BDUSS`, `DedeUserID` |
| Token cookies | Auth tokens absent for anon | `token`, `auth_token`, `access_token`, `SESSDATA` |

**Bad candidates** (avoid these — they exist for anonymous users too):
- `BAIDUID`, `_ga`, `_gid` — tracking cookies, always present
- `csrf_*`, `_device_id` — present regardless of login
- `timezone`, `locale`, `theme` — preferences, not auth
- `buvid*`, `b_nut`, `_uuid` — device/browser fingerprints, not auth

If the user is **not logged in**, you won't see the login cookie yet. In that
case:
1. List what cookies ARE present (these are the anon cookies to avoid).
2. Ask the user to log in manually in Chrome.
3. Re-run `awc cookies:get --url <siteUrl> --json`.
4. The **new** cookies that appeared after login are your candidates.

If the user **is logged in**, check the cookie value too — some sites set
`logged_in=no` for anonymous and `logged_in=yes` for authenticated. In that
case, include `"value"` in the config:

```json
"cookie": { "url": "https://github.com", "name": "logged_in", "value": "yes" }
```

### Step 4 — Write the config

Write `~/.awc/auth/<name>.json`. Minimal form (cookie exists = logged in):

```json
{
  "loginUrl": "<loginUrl>",
  "loggedInWhen": {
    "cookie": { "url": "<cookieUrl>", "name": "<cookieName>" }
  }
}
```

With value check (cookie must have specific value):

```json
{
  "loginUrl": "<loginUrl>",
  "loggedInWhen": {
    "cookie": { "url": "<cookieUrl>", "name": "<cookieName>", "value": "<value>" }
  }
}
```

**Cookie URL**: use the domain the cookie is set on. Check the `domain` field
in the cookie list — if it's `.example.com`, use `https://example.com`. If
it's `backend.example.com`, use `https://backend.example.com`.

**Login URL**: the page where the user enters credentials or clicks the login
button. For SSO sites, this is the entry point that triggers the SSO flow.

### Step 5 — Validate

```sh
awc auth:login <name> --check
```

- If it returns `logged in ✓` and the user IS logged in → config is correct.
- If it returns `not logged in ✗` and the user is NOT logged in → config is correct.
- If the result disagrees with reality → the cookie condition is wrong. Go
  back to step 3 and pick a different cookie.

Then show the user:

```sh
awc auth:list
```

### Step 6 — Optional: configure SSO steps

If the site uses SSO (e.g. redirects to a central auth page with "免密登录" or
"passwordless" buttons), add `ssoSteps`:

```json
{
  "loginUrl": "https://app.example.com/",
  "loggedInWhen": { "cookie": { "url": "https://app.example.com", "name": "sessionid" } },
  "ssoSteps": [
    { "hostContains": "sso.example.com", "clickText": "免密登录" }
  ]
}
```

The extension auto-detects login buttons (by text "登录"/"login"/"sign in",
or `type=submit`, or buttons in password forms), so `loginButton` is usually
not needed. Only add it if auto-detect fails.

## Common patterns by site type

### Internal enterprise (SSO + session cookie)

```json
{
  "loginUrl": "https://s.sysop.yy.com/",
  "loggedInWhen": { "cookie": { "url": "https://s-backend.sysop.yy.com", "name": "sessionid" } },
  "ssoSteps": [
    { "hostContains": "uuap.baidu.com", "clickText": "免密登录" }
  ]
}
```

### OAuth / "Login with X" (cookie flag)

```json
{
  "loginUrl": "https://github.com/login",
  "loggedInWhen": { "cookie": { "url": "https://github.com", "name": "logged_in", "value": "yes" } }
}
```

### Simple form login (session cookie)

```json
{
  "loginUrl": "https://my-app.com/login",
  "loggedInWhen": { "cookie": { "url": "https://my-app.com", "name": "connect.sid" } }
}
```

## Troubleshooting

- **`awc auth:login <name> --check` times out**: the extension may not be
  loaded. Run `awc sys:status`; if the extension is disconnected, reload it at
  `chrome://extensions`.
- **Cookie condition never matches**: verify the cookie actually exists with
  `awc cookies:get --url <url> --name <name>`. Check the `domain` field — the
  cookie URL must match the cookie's domain.
- **`awc cookies:get` returns empty**: the user may be in incognito mode, or
  the extension lacks cookie permission for that domain. Check that
  `<all_urls>` host permission is granted.
