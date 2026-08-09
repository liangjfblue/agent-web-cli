# AGENTS.md — agent-web-cli integration guide for AI agents

> You are an AI agent (Claude, GPT, Cursor, etc.) writing a CLI, script, or
> service that needs to reuse the user's Chrome login sessions. This file
> tells you everything you need to know to use `awc` programmatically.

## Installation

If `awc` is not yet installed, help the user set it up:

```sh
# 1. Install (one command — postinstall auto-registers the native host)
npm install -g @agent/web-cli

# 2. Run setup wizard — builds, registers host, configures PATH,
#    installs AI skills, and detects the Chrome extension
awc sys:setup
```

> **Not published yet.** `@agent/web-cli` is not yet on the npm registry, so
> `npm install -g @agent/web-cli` will fail with 404 until the first release.
> Until then, install from a local checkout instead:
> ```sh
> cd agent-web-cli && npm install -g .   # triggers postinstall
> # or, for development:
> ./scripts/build.sh                     # builds ./bin/awc (add to PATH)
> ```
> Once published, the `npm install -g @agent/web-cli` line above works as-is.

After `sys:setup`, the only remaining manual step is loading the Chrome
extension (Chrome doesn't allow silent installs of unpacked extensions).
`sys:setup` prints the exact folder path to load — guide the user there.

Verify when done:

```sh
awc sys:status    # should show host + extension connected
```

If the user doesn't have npm/Node, they can download a prebuilt binary + run
`awc sys:setup` directly. See README.md "Build" for building from source.

## What awc does

`awc` reads cookies and drives Chrome from the command line. It reuses the
user's **existing, already-logged-in Chrome** — no headless browser, no
separate browser instance, no stored credentials. Cookies are read live from
Chrome's cookie store via a native-messaging extension.

**One-line summary**: `awc cookies:get --url <url> --header` gives you a
`Cookie:` header you can pass to any HTTP client.

## Prerequisites check

Before using any `awc` command, verify it's connected:

```sh
awc sys:status
```

- Outputs host version + endpoint → connected, proceed.
- Error "host not reachable" → tell the user: run `awc sys:setup`, then load
  the extension at `chrome://extensions`, then retry.

## Core operations (the 90% you'll use)

### Read cookies as an HTTP header

```sh
awc cookies:get --url "https://api.example.com" --header
```

Output: a single line like `sessionid=abc123; token=xyz789; ...` — pass it as
the `Cookie` header to curl/requests/fetch/etc.

**Flags:**
- `--url <url>` — read cookies for this domain (required)
- `--header` — output as `name=value; name=value` (for HTTP)
- `--name <name>` — filter to one cookie
- `--json` — structured output with domain, path, expiry, etc.

### Check if logged in to a site

```sh
awc auth:login <name> --check
```

- Outputs `logged in ✓` or `not logged in ✗`
- Returns instantly (checks cookie existence — does NOT block)
- Use this before reading cookies if you're unsure of login state.

### Trigger login (may block — see below)

```sh
awc auth:login <name>
```

- Opens the login page in Chrome, auto-clicks login buttons, then polls.
- **May block up to 120 seconds** waiting for the user to log in manually.
- Idempotent: if already logged in, returns instantly.
- Only call this when `--check` returned `not logged in` AND the user is
  present to complete login.

### List configured auth profiles

```sh
awc auth:list
```

### Configure login for a new site

```sh
awc auth:config <name> --url <loginPageUrl>
```

Interactive: reads cookies before login, asks user to log in, reads cookies
after, detects which cookies are new (the login-state signal), writes config
automatically. The user doesn't need to know cookie names.

## Browser operations

### Tabs

```sh
awc tabs:list                              # list all open tabs (--json for structured)
awc tabs:open <url> [--foreground]         # open or reuse a tab
awc tabs:focus <tabId>                     # activate a tab
```

### DOM

```sh
awc dom:snapshot [--url <url> | --tab-id <id>]   # get elements with anchors
awc dom:click  --anchor <a>  | --text "..." | --selector "..." | --role button
awc dom:type   --anchor <a> --value "..."         # type into an input
awc dom:query  --text "..." [--json]              # find elements without acting
awc dom:text   [--selector "..."]                 # read page/element text
```

**Locator flags** (shared by click/type/query): `--anchor` (from snapshot),
`--selector` (CSS), `--text`, `--role`, `--name`, `--label`, `--testid`,
`--strict` (reject if multiple matches).

### Screenshot

```sh
awc shot:page [-o file.png] [--tab-id <id>]
```

### Network capture

```sh
awc net:watch  --url <tabUrl> --duration 10s [--include-static]
awc net:debug  --duration 10s [--url-pattern "/api/"] [--json] [--include-console]
awc net:stop   [--capture-id <id> | --all]
```

### Console

```sh
awc console:watch --duration 5s [--level error]
awc console:clear
```

### CDP (Chrome DevTools Protocol)

```sh
awc cdp:send  <method> [--params '{"expression":"..."}']
awc cdp:listen --event "Network.*" --duration 10s
```

### Wait

```sh
awc wait:for --selector ".loaded" --timeout 30s        # wait for element
awc wait:for --text "Done" --timeout 30s               # wait for text
awc wait:for --url-pattern "/dashboard" --timeout 30s  # wait for URL
awc wait:for --url-pattern "/api/login" --status 200   # wait for XHR
```

## Cookie handling patterns for code you write

### Pattern 1: read cookie → call API (simplest)

```python
import subprocess, requests

cookie = subprocess.check_output(
    ["awc", "cookies:get", "--url", "https://api.example.com", "--header"]
).decode().strip()

resp = requests.get("https://api.example.com/api/data", headers={"Cookie": cookie})
```

### Pattern 2: check login state first

```python
import subprocess

result = subprocess.run(["awc", "auth:login", "mysite", "--check"], capture_output=True, text=True)
if "not logged in" in result.stdout:
    # cookie might be expired — tell the user to run awc auth:login mysite
    raise RuntimeError("not logged in — run: awc auth:login mysite")

cookie = subprocess.check_output(
    ["awc", "cookies:get", "--url", "https://api.example.com", "--header"]
).decode().strip()
```

### Pattern 3: call API, handle rejection, advise re-login

```sh
#!/bin/sh
set -e
COOKIE=$(awc cookies:get --url "https://api.example.com" --header)
resp=$(curl -s -H "Cookie: $COOKIE" "https://api.example.com/api/data")

# API rejected the cookie — it's expired or invalid
if echo "$resp" | grep -q '"code":-101\|"code":401\|unauthorized'; then
    echo "cookie rejected by API" >&2
    echo "re-login: awc auth:login mysite" >&2
    exit 1
fi
echo "$resp"
```

### Critical: do NOT call `auth:login` (without --check) in non-interactive code

`auth:login` may block for 120s waiting for manual login. It is safe to call
in scripts the user is watching, but **never** in:
- cron jobs
- CI/CD pipelines
- background services / daemons
- any context where nobody is watching the terminal

Use `auth:login --check` (instant, non-blocking) for automated checks, and
instruct the user to run `auth:login` interactively when re-login is needed.

## Auth config file format

Location: `~/.awc/auth/<name>.json`

```json
{
  "loginUrl": "https://example.com/login",
  "loggedInWhen": {
    "cookie": { "url": "https://example.com", "name": "sessionid" }
  }
}
```

Minimal — just `loginUrl` + a cookie condition. The cookie condition is the
authoritative login signal: `chrome.cookies.get` checks if the cookie exists.
Optional `value` field for cookies like GitHub's `logged_in=yes`.

To create a new config, either:
- Run `awc auth:config <name> --url <loginUrl>` (interactive, auto-detects)
- Write the JSON file directly
- Ask the user and write it for them (see the awc-auth-config skill)

## Output conventions

- All commands support `--json` for structured output.
- Non-JSON output is human-readable (tables, colored text).
- Exit code 0 = success, non-zero = failure.
- Errors go to stderr; data goes to stdout.
- `--timeout 10s` can be added to any command (default 30s; long operations
  like `auth:login` auto-extend).

## Full command reference

Run `awc --help` for the complete list (30+ commands). Run
`awc <command> --help` for any command's flags.
