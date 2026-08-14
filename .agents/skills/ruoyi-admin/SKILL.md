---
name: ruoyi-admin
description: "Query the read-only RuoYi management console through the ruoyi CLI. Use this skill whenever the user asks about RuoYi users, roles, departments, account status, operation logs, login logs, or browser-login recovery, even if they do not mention the CLI. It also handles an expired browser session by opening the RuoYi login flow and retrying the original query after the user completes any CAPTCHA."
compatibility: "Requires the ruoyi command, the agent-web-cli Chrome extension/host, and a configured RuoYi browser profile."
---

# RuoYi admin skill

## Prerequisites
Install the CLI: `cd example/ruoyi-cli && npm link`. The `ruoyi` command then
reuses the signed-in Chrome session through awc (auth profile `ruoyi` for
`https://vue.ruoyi.vip`).


Use the `ruoyi` business CLI for all RuoYi data access. The CLI is read-only:
it exposes user and role status/details and operation/login logs only. Do not
invent or emulate create, update, delete, export, authorization, trigger, or
sync commands, including log delete or clean.

## Command map

Translate requests to these commands:

```text
status                         -> ruoyi status
list users                     -> ruoyi user:list
find users by name             -> ruoyi user:list --name <name>
find users by phone            -> ruoyi user:list --phone <phone>
filter users by status         -> ruoyi user:list --status enabled|disabled
show one user                  -> ruoyi user:get <numeric-id>
list roles                     -> ruoyi role:list
find roles by name/key         -> ruoyi role:list --name|--key <value>
show one role                  -> ruoyi role:get <numeric-id>
list operation logs            -> ruoyi operlog:list [--title <title>]
                                 [--oper-name <name>] [--business-type <0-9|label>]
                                 [--status success|failed]
list login logs                -> ruoyi loginlog:list [--user <name>] [--ip <ip>]
                                 [--status success|failed]
```

Add `--json` when structured output is useful. Preserve the CLI's read-only
scope and summarize the returned fields instead of exposing browser credentials.

## Login recovery

Call the requested `ruoyi` command first. Do not run `awc auth:login --check`
as a preflight: a stale cookie can exist while the RuoYi API rejects it.

Handle failures by exit code and message:

- Exit `0`: answer from the command output.
- Exit `2`: correct the command arguments; do not touch login state.
- Exit `10` with `login required`: run `ruoyi login`.
- Exit `10` with `browser credentials were rejected`: run `ruoyi login --refresh`.
- Exit `11`: report that the browser login timed out and ask the user to retry.
- Exit `20` or `30`: report the awc host/extension problem and suggest
  `awc sys:status`; do not ask the user to log in for an infrastructure error.

Before starting either login command, tell the user that a RuoYi login page
will open. The user must enter any CAPTCHA or password themselves. Never read,
guess, or print passwords, CAPTCHA values, cookies, JWTs, or `cookieHeader`.
The login command captures awc's JSON internally, so its credential payload is
not displayed. When invoking `ruoyi login` through a command execution tool,
set that tool's timeout to at least 330 seconds. The browser login itself waits
up to 300 seconds and the business CLI allows 315 seconds, so a tool's default
10-15 second timeout would terminate a healthy login flow. Wait for the command
to finish, then rerun the original `ruoyi` query once. Do not poll forever or
ask the user to repeat a successful login.

If login times out, leave the browser page available for the user and report
the exact next command (`ruoyi login` or `ruoyi login --refresh`).

## Response rules

Report the result in the user's language. For list commands include the total
and the useful columns: users show id/username/nickname/department/phone/
status, roles show id/name/key/data scope/status, operation logs show
id/title/type/operator/ip/status/cost/time, login logs show id/user/ip/
location/browser/status/time. For detail commands include the requested
record's identity, status, and relevant department/role fields. Log rows
contain the full detail fields (parameters, results, error messages); use
`--json` on the list commands to read them. State clearly when no records
match. If the user asks for a write operation, refuse that operation and point
them to the supported read-only commands.
