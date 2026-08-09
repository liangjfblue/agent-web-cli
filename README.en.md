<div align="center">

# Agent Web CLI (awc)

**English** · [简体中文](README.md)

<img src="assets/awc-readme-cover-v2.png" alt="Agent Web CLI cover">

</div>

`awc` lets your **command-line tools and AI agents reuse your Chrome login
sessions** — no headless browser, no separate browser instance, no stored
passwords. It reads cookies live from your already-logged-in Chrome via a
small extension, and can also drive Chrome (tabs, DOM, network, screenshots).

```
your CLI / AI agent ──► awc ──► Chrome (the one you're already logged into)
                            reads HttpOnly cookies via chrome.cookies API
```

Typical use: you're logged into some internal site in Chrome; `awc` reads
that session cookie so a script or agent can call the site's authenticated
APIs — without you re-entering credentials anywhere.

## Install

**macOS / Linux** (one line, no Node required):

```sh
curl -fsSL https://raw.githubusercontent.com/liangjfblue/agent-web-cli/main/install.sh | bash
```

The installer detects your platform, downloads the binary from the
[latest release](https://github.com/liangjfblue/agent-web-cli/releases), installs
to `~/.awc/bin`, adds it to PATH, and runs `awc sys:setup` (registers the
native host + installs the `awc-auth-config` skill for your AI agents).

> **Windows**: download `awc-windows-amd64-<ver>.zip` from
> [Releases](https://github.com/liangjfblue/agent-web-cli/releases), extract,
> and run `awc sys:setup`. Or use WSL with the command above.

**Then load the Chrome extension** (the only manual step — Chrome won't allow
silent installs of unpacked extensions):

1. `chrome://extensions` → enable **Developer mode**
2. **Load unpacked** → select `~/.awc/extension`
3. Verify: `awc sys:status` (should show the host connected)

## Quick start: reuse a login session

### Option A — let an AI agent configure it (recommended)

`sys:setup` installs the `awc-auth-config` skill into your AI agent's skill
directory (ZCode, Claude Code, Cursor, Codex). In your agent, just say:

```
帮我配一下 <name> 的登录，地址 <login URL>
```

e.g. `帮我配一下 sysop 的登录，地址 https://sysop.example.com/login`

The agent opens the site, reads the cookies, asks you to log in if needed,
identifies which cookie signals "logged in", writes the config, and verifies
it. **You never need to know cookie names.**

### Option B — read cookies directly

```sh
# Read cookies as an HTTP Cookie header
awc cookies:get --url "https://api.example.com" --header
# → sessionid=abc123; token=xyz789; ...

# Use it with curl / any HTTP client
COOKIE=$(awc cookies:get --url "https://api.example.com" --header)
curl -H "Cookie: $COOKIE" "https://api.example.com/api/data"
```

Cookies are read live from Chrome each time — never cached or stored.

### Check login state

```sh
awc auth:login <name> --check    # instant: "logged in ✓" / "not logged in ✗"
```

> `auth:login` (without `--check`) may block up to 120s waiting for a manual
> login — only use it interactively, never in cron/CI/daemons.

## Commands

```
System     sys:status | sys:doctor | sys:install | sys:uninstall
Cookies    cookies:get [--url --name --header --json]
Auth       auth:login <name> [--check] | auth:config <name> --url <u> | auth:list
Tabs       tabs:list | tabs:open <url> [--foreground] | tabs:focus <id>
DOM        dom:snapshot | dom:click | dom:type | dom:query | dom:text
           locators: --anchor | --selector | --text | --role | --name | --label | --testid
Screenshot shot:page [-o file] [--tab-id]
Network    net:watch | net:debug | net:stop | net:body
Console    console:watch [--level] | console:clear
CDP        cdp:send <method> [--params] | cdp:listen [--event]
Wait       wait:for [--selector | --text | --url-pattern | --status]
```

Global flags: `--json` (raw JSON output), `--timeout 10s` (per-call).

Run `awc --help` for the full list, `awc <command> --help` for any command's flags.

For the complete integration guide (cookie patterns, auth handling, every
command), see **[AGENTS.md](AGENTS.md)** — it's written for AI agents but is
the most thorough reference for programmatic use.

## Handling expired cookies

Cookies expire or get revoked. Each `awc` command stays single-purpose —
`cookies:get` returns instantly (never auto-logs-in), so it's safe in scripts.
Handle expiry explicitly:

```sh
COOKIE=$(awc cookies:get --url "https://api.example.com" --header)
resp=$(curl -s -H "Cookie: $COOKIE" "https://api.example.com/api/data")
# API rejected the cookie → tell the user to re-login
echo "$resp" | grep -q '"code":401' && echo "re-login: awc auth:login <name>"
```

> **cookie existence ≠ validity.** `auth:login --check` only tests whether a
> cookie exists in Chrome; the API response is the only authoritative test.

## Demos

Two complete demos (business CLI + skill) live in [`example/`](example/) —
an admin panel and a service-governance platform. They show the full pattern:
awc reads the cookie → a business CLI calls authenticated APIs → a skill lets
agents query via natural language. See
[`example/DEMO-WALKTHROUGH.md`](example/DEMO-WALKTHROUGH.md).

---

# For developers / contributors

## Build from source

```sh
./scripts/build.sh                  # current platform → bin/awc + bin/awc-host
./scripts/build.sh --pack           # + npm tarball
./scripts/cross-build.sh --pack     # all platforms → dist/awc-<os>-<arch>-<ver>.tar.gz
VERSION=v0.2.0 ./scripts/build.sh   # override version
```

Version source: `$VERSION` env > git tag > `package.json`. Injected via
`-ldflags` into `awc --version`.

## How it works (architecture)

```
awc CLI ──AW frame (msgpack+CRC16)──► Go host ──native messaging──► Chrome extension ──chrome.*──► page
```

No HTTP/WebSocket server, no local port opened.

**CLI ↔ host** (Unix socket / named pipe) — each frame:
```
| magic "AW" | ver u16=1 | payload len uint32 BE | msgpack payload | crc16 |
```
`crc16` = CRC-16/CCITT-FALSE over the payload, big-endian.

**host ↔ extension** (native messaging) — 4-byte LE length prefix + UTF-8 JSON:
```
→ { tid, op, args? }    ← { tid, ok, data?, code?, msg? }
```

## Project layout

```
agent-web-cli/
├── cmd/awc/        # CLI entry (cobra)
├── cmd/host/       # native-messaging host entry
├── internal/
│   ├── proto/      # AW frame codec + msgpack + CRC16
│   ├── ipc/        # Unix socket / named pipe client
│   ├── host/       # bridges CLI socket ↔ native messaging
│   ├── cmd/        # cobra command tree
│   └── install/    # native-messaging manifest + registration
├── extension/      # Chrome extension (MV3)
└── go.mod
```

## Test

```sh
go test ./...
```

## Native-messaging host registration

`awc sys:install` writes the Chrome native-messaging manifest:

| OS | Location |
|---|---|
| macOS | `~/Library/Application Support/Google/Chrome/NativeMessagingHosts/com.awc.host.json` |
| Linux | `~/.config/google-chrome/NativeMessagingHosts/com.awc.host.json` |
| Windows | registry `HKCU\Software\Google\Chrome\NativeMessagingHosts\com.awc.host` |

Uninstall with `awc sys:uninstall`.
