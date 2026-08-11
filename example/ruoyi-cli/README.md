# RuoYi read-only CLI

This example wraps the public RuoYi management console at `https://vue.ruoyi.vip`.
It only exposes user and role queries. There are deliberately no create, update,
delete, export, authorization-change, trigger, or sync commands.

## Setup

Load the repository's `extension/` folder in Chrome and sign in to RuoYi. Then
install the auth profile and link the CLI:

```powershell
New-Item -ItemType Directory -Force "$HOME\.awc\auth" | Out-Null
Copy-Item .\auth\ruoyi.json "$HOME\.awc\auth\ruoyi.json"
npm link
```

Install the business skill for your agent:

```powershell
New-Item -ItemType Directory -Force "$HOME\.codex\skills\ruoyi-admin" | Out-Null
Copy-Item .\skill\SKILL.md "$HOME\.codex\skills\ruoyi-admin\SKILL.md"
```

For a source build that is not on `PATH`, point the adapter at it:

```powershell
$env:AWC_BIN = "C:\path\to\agent-web-cli\bin\awc.exe"
```

## Commands

```text
ruoyi login [--refresh]
ruoyi status
ruoyi user:list --page 1 --page-size 20
ruoyi user:list --name admin --status enabled
ruoyi user:get 1
ruoyi role:list --name 管理员
ruoyi role:get 1
```

Interactive login waits up to five minutes. Agents invoking `ruoyi login`
should give their command execution tool a timeout of at least 330 seconds.

Every command supports `--json`. The CLI obtains browser credentials through
`awc session:acquire`; it never prints or persists the token. API responses with
HTTP 401/403 or RuoYi code 401 are treated as expired browser credentials.
The reported `session:acquire --interactive --refresh` command removes only
the configured auth cookie before opening a fresh browser login.
The `ruoyi-admin` skill calls `ruoyi login` or `ruoyi login --refresh` for this
recovery and retries the original read-only query after browser login succeeds.
