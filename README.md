# Agent Web CLI (awc)

`awc` drives Chrome from the command line by reusing the browser's existing
login sessions. It speaks to a small Chrome extension over the standard
native-messaging channel, then to a local Go host over a binary-framed socket.

```
awc CLI ──AW frame (msgpack+CRC16)──► Go host ──native messaging──► Chrome extension ──chrome.*──► page
```

It does **not** start an HTTP/WebSocket server or open a local port.

## Build

```sh
# 编译当前平台 (bin/awc + bin/awc-host)
./scripts/build.sh

# 编译 + 打包 npm tarball
./scripts/build.sh --pack

# 交叉编译全部平台到 dist/ (macOS arm/x64, Linux x64/arm64, Windows x64)
./scripts/cross-build.sh

# 交叉编译 + 每个平台打 tarball
./scripts/cross-build.sh --pack

# 指定版本号
VERSION=v0.2.0 ./scripts/build.sh
```

版本号优先级: `$VERSION` 环境变量 > git tag > package.json。编译时通过
`-ldflags` 注入到 `awc --version` 和 `awc-host`。

输出:
- `bin/` — 当前平台二进制
- `dist/awc-<os>-<arch>/` — 各平台自包含包 (bin + extension + installer)
- `dist/awc-<os>-<arch>-<ver>.tar.gz` — 各平台 tarball

## Install the native-messaging host

```sh
awc sys install
```

This writes a Chrome native-messaging manifest pointing at a launcher for the
host binary, into:

| OS | Location |
|---|---|
| macOS | `~/Library/Application Support/Google/Chrome/NativeMessagingHosts/com.awc.host.json` |
| Linux | `~/.config/google-chrome/NativeMessagingHosts/com.awc.host.json` |
| Windows | registry `HKCU\Software\Google\Chrome\NativeMessagingHosts\com.awc.host` |

Uninstall with `awc sys uninstall`.

> **Note:** the extension's manifest currently has a placeholder `"key"`.
> Before loading, generate a real RSA key pair and put the base64 public key
> in `extension/manifest.json`, then update the `ExtensionID` constant in
> `internal/install/install.go` to the resulting 32-character ID. This pins
> the extension ID so the host's `allowed_origins` matches.

## Load the Chrome extension

1. Open `chrome://extensions`.
2. Enable **Developer mode**.
3. Click **Load unpacked** and select the `extension/` folder.
4. Reload the extension after any change.

## Commands

```
System        sys:status | sys:doctor | sys:install | sys:uninstall
Cookies       cookies:get [--url --name --header --redact]
Tabs          tabs:list | tabs:open <url> [--foreground] | tabs:focus <id>
Screenshot    shot:page [-o file] [--tab-id]
DOM           dom:snapshot [--include-hidden]
              dom:click / dom:type / dom:query / dom:text
              locator: --anchor | --selector | --role | --name
                         --text | --label | --testid | --strict
Network       net:watch [--duration --url-pattern --include-static]
              net:stop [--capture-id | --all]
              net:debug [--duration --url-pattern --max-body-bytes --no-body --include-console]
              net:body <key> [-o file | --raw]
Console       console:watch [--duration --level] | console:clear
CDP           cdp:send <method> [--params JSON | --params-file f]
              cdp:listen [--event pat]... [--enable cmd]... [--duration]
Wait          wait:for [--kind selector|text|url|xhr]
                     [--selector | --text | --url-pattern | --status]
Record        rec:start [--mouse --scroll -o file] | rec:status | rec:stop [-o file]
Profiles      profiles:list | profiles:rename <name>
              profiles:default [name | --clear]
```

Global flags: `--json` (raw JSON output), `--timeout 10s` (per-call).

## Check connectivity

```sh
awc sys:status      # host + extension summary
awc sys:status --json
awc sys:doctor      # full diagnostics
```

When everything is connected, `awc sys:status` reports the host version and
endpoint, and the extension reports `connected: true`.

## Using cookies with other CLI tools

`awc cookies:get --header` outputs a standard `Cookie:` header value that can
be passed directly to curl, wget, Python, Node, Go, or any HTTP client. The
cookies are read live from Chrome's cookie store each time — they are never
cached or written to disk.

```sh
# Read cookies as a header string
awc cookies:get --url "https://api.bilibili.com" --header

# Use with curl
COOKIE=$(awc cookies:get --url "https://api.bilibili.com" --header)
curl -H "Cookie: $COOKIE" "https://api.bilibili.com/x/web-interface/nav"
```

### Handling expired cookies

Cookies expire, get revoked, or become invalid. `awc` does not auto-retry —
each command stays single-purpose (see design note below). The calling script
checks the API response and triggers re-login explicitly:

```sh
#!/bin/sh
set -e

SITE=bilibili
API=https://api.bilibili.com/x/web-interface/nav

# 1. Check login state (fast, non-blocking)
awc auth:login "$SITE" --check || {
    echo "not logged in — starting login flow..."
    awc auth:login "$SITE"
}

# 2. Read cookie and call the API
COOKIE=$(awc cookies:get --url "https://api.bilibili.com" --header)
resp=$(curl -s -H "Cookie: $COOKIE" "$API")

# 3. API says cookie invalid? — needs manual login
case "$resp" in
    *'"code":-101'*|*'"code":401'*)
        echo "cookie rejected by API — re-login required:" >&2
        echo "  awc auth:login $SITE" >&2
        exit 1
        ;;
esac

echo "$resp"
```

### Why `cookies:get` doesn't auto-login on failure

`auth:login` (without `--check`) may block for up to 2 minutes waiting for
the user to complete a manual login in the browser. If `cookies:get` called it
implicitly, scripts running in cron jobs, CI pipelines, or background services
would hang silently with nobody watching.

Instead, each command has a single, predictable behavior:

| Command | Always |
|---|---|
| `cookies:get` | returns instantly (read-only) |
| `auth:login --check` | returns instantly (checks cookie existence) |
| `auth:login` | may block (but the caller opted in explicitly) |

Also note: **cookie *existence* ≠ cookie *validity*.** A cookie may be present
but rejected by the API (banned, insufficient permissions, IP restriction).
Only the API response can authoritatively say whether the cookie works.

## Wire protocol

### CLI ↔ host (Unix socket / named pipe)

Each frame:

```
+--------+--------+--------+--------+--------+-----------+--------+
| magic  | ver    | payload length (uint32 BE) | msgpack   | crc16  |
| "AW"   | u16=1  |        4 bytes            | N bytes   | 2 bytes|
+--------+--------+--------+--------+--------+-----------+--------+
```

- `magic` = `0x41 0x57` (`"AW"`) — frame sync.
- `crc16` = CRC-16/CCITT-FALSE over the payload, big-endian.

Payload is a msgpack-encoded `Request` or `Response`:

```
Request:  { tid, op, args? }
Response: { tid, ok, data?, err? }
err:      { code?, msg? }
```

### host ↔ extension (native messaging)

Chrome mandates a 4-byte little-endian length prefix followed by UTF-8 JSON.
The JSON uses this project's own field names:

```
→ request:  { tid, op, args? }
← reply:    { tid, ok, data?, code?, msg? }
```

## Project layout

```
agent-web-cli/
├── cmd/
│   ├── awc/         # CLI entry (cobra)
│   └── host/        # native-messaging host entry
├── internal/
│   ├── proto/       # AW frame codec + msgpack messages + CRC16
│   ├── ipc/         # Unix socket / named pipe client
│   ├── host/        # host: bridges CLI socket ↔ native messaging
│   ├── cmd/         # cobra command tree (sys, ...)
│   └── install/     # native-messaging manifest + launcher + registration
├── extension/       # Chrome extension (MV3)
│   ├── manifest.json
│   └── background.js
└── go.mod
```

## Test

```sh
go test ./...
```
