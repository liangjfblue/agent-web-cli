<div align="center">

<img src="assets/awc-readme-cover-v2.png" alt="Agent Web CLI cover">

**English** · [简体中文](README.md)

</div>

**Internal web apps with no API — now agent-native.**

Login-gated systems that ship a web UI but no agent-friendly API — internal admin panels, release and ops consoles, service-governance tools, legacy back offices, third-party SaaS dashboards — `awc` compiles them into CLI + skill that you **generate once, then run deterministically**.

Your signed-in Chrome is only the **auth bootstrap + API discovery** step, not the runtime. After generation, the agent calls the HTTP API directly — no page reading, no button clicking on every task. Passwords never leave the browser.

> **Where it fits:** `awc` is built for developer laptops and interactive ops consoles; it is not meant for unattended server cron jobs (every call needs a signed-in Chrome session).

## 1. Quick installation

Send this prompt to Codex, Claude Code, Cursor, or another coding agent:

```text
Install https://github.com/liangjfblue/agent-web-cli .
Read AGENTS.md first, install it for the current operating system, and run awc sys:setup.
Never print Cookie, Token, or session JSON values.
When done, tell me the Chrome extension directory and the one manual step I must complete, then verify with awc sys:status.
```

The agent downloads or builds `awc`, registers the Native Messaging Host, and
installs two companion skills:

- `awc-auth-config` identifies the site's login signal and generates an auth profile.
- `awc-build-business-cli` generates a CLI and business skill for requested operations.

Chrome does not allow silent installation of unpacked extensions, so you
perform one manual action:

1. Open `chrome://extensions` in the address bar.
2. Enable **Developer mode** and click **Load unpacked**.
3. Select the `extension` directory reported by the agent.

Installation is complete when `awc sys:status` shows both host and extension connected.

<details>
<summary>Manual installation without an agent</summary>

macOS / Linux:

```sh
curl -fsSL https://raw.githubusercontent.com/liangjfblue/agent-web-cli/main/install.sh | bash
```

Windows: download `awc-windows-amd64-<ver>.zip` from
[Releases](https://github.com/liangjfblue/agent-web-cli/releases), extract it,
and run:

```powershell
.\bin\awc.exe sys:setup
```

Then load the extension using the three steps above and run `awc sys:status`.

</details>

## 2. Generate a business CLI + skill

Sign in to the target business system in Chrome, open the page you want to
wrap, and tell the agent:

```text
Use awc-build-business-cli to turn the currently signed-in admin site into a CLI and skill.
Support order listing, order details, order creation, and order cancellation.
```

Replace the last line with the real operations you need. Query, create, update,
and delete commands are all supported; the generic template does not preset
resources such as `user` or `role`.

The agent will:

1. Check `awc`, the Chrome extension, and current login state.
2. Use `awc-auth-config` to identify the auth cookie, storing only its name and login rules, never its value.
3. Analyze HTTP APIs only within the requested scope.
4. Generate the business CLI, auth config, tests, and companion business skill.
5. Verify normal calls, expired login recovery, re-login, and one retry of the original command.

After generation, keep using natural language instead of memorizing CLI flags:

```text
List orders created today
Create a test order for product 1001 with quantity 2
Cancel order 20260811001
```

The business skill translates these requests into deterministic CLI commands.
When login expires, it opens the login page, waits for you to complete a
password, SSO, or CAPTCHA flow, and retries the original command once.

For create, update, and delete operations, the agent must first resolve the
exact target and effect. It must not mutate live data merely to discover an API
or smoke-test a CLI. Cookie, Token, JWT, and `cookieHeader` values must never
appear in terminal output, logs, source code, or skill content.

Examples:

- [`demo-admin`](example/demo-admin/): simulated order administration.
- [`demo-svcgov`](example/demo-svcgov/): simulated service governance.
- [`ruoyi-cli`](example/ruoyi-cli/): real user/role queries and login recovery
  against the [RuoYi online demo](https://vue.ruoyi.vip/index).

## 3. How it differs from browser agents

Projects like Browser Use or Grok Bot make the agent drive a browser. `awc` instead *compiles* a logged-in web system into a CLI the agent can call deterministically. Chrome is used only during the initial build; day-to-day execution goes straight to the HTTP API.

|  | Browser agents (Browser Use / Grok Bot) | `awc` |
|--|--|--|
| Target systems | API or no API | **Internal backends with no agent API, only a login-gated web UI** |
| First-time onboarding | Drive the browser | Drive the browser + API discovery |
| Day-to-day execution | Read pages / click buttons each time | **Call the HTTP API directly** |
| LLM involvement | Heavy on every task | **Generate once, run deterministically after** |
| Impact of UI changes | High | **Low (unaffected as long as the API contract holds)** |
| Token cost | High | **Low** |
| Speed | Seconds to tens of seconds | **Close to a native API** |
| Output artifact | prompt / browser task | **CLI + skill (reusable, shareable)** |
| In essence | Runtime automation | **Web → Agent tool compiler** |

## 4. How it works

The flow has two phases: generating an integration and using it day to day.
The auth profile stores only Cookie names, URLs, and login rules. Cookie and
Token values remain in Chrome and are used only briefly inside the business CLI process.

```mermaid
flowchart TB
    subgraph BUILD["Generate the business integration"]
        U["User describes the target page and operations"] --> BUILDER["awc-build-business-cli"]
        BUILDER --> READY["Check awc, the extension, and requested scope"]
        READY --> PROFILE{"Matching auth profile exists?"}
        PROFILE -- "No" --> AUTH["awc-auth-config"]
        AUTH --> DIFF["Compare Cookies before and after login<br/>generate auth.json"]
        PROFILE -- "Yes" --> DISCOVER["Analyze only the requested HTTP APIs"]
        DIFF --> DISCOVER
        DISCOVER --> GENERATE["Generate the business CLI, skill, and tests"]
        GENERATE --> VERIFY["Verify normal calls, login recovery, and one retry"]
    end

    subgraph RUN["Use the business integration"]
        REQUEST["Natural-language business request"] --> BSKILL["Business skill<br/>select command and translate arguments"]
        BSKILL --> BCLI["Business CLI"]
        BCLI --> SESSION["awc session:acquire"]
        SESSION --> CONFIG["Read auth.json"]
        CONFIG --> BRIDGE["awc-host + Chrome extension<br/>Native Messaging"]
        BRIDGE --> LOGIN{"loggedInWhen login signal exists?"}
        LOGIN -- "No" --> INTERACTIVE["Business CLI login<br/>open Chrome and wait for the user"]
        LOGIN -- "Yes" --> CREDENTIALS["Acquire Cookies for the API origin"]
        INTERACTIVE --> CREDENTIALS
        CREDENTIALS --> API["Business CLI calls the business HTTP API"]
        API --> ACCEPTED{"API accepts the credentials?"}
        ACCEPTED -- "Yes" --> RESULT["Return the business result"]
        ACCEPTED -- "No: 401/403" --> REFRESH["Business CLI login --refresh<br/>clear rejected Cookie and sign in again"]
        REFRESH --> RETRY["Retry the original command once"]
        RETRY --> SESSION
    end

    GENERATE -. "generated artifact" .-> BSKILL
    GENERATE -. "generated artifact" .-> BCLI
    DIFF -. "auth rules" .-> CONFIG
```

---

**Complete contract for agents and CLI developers:** [AGENTS.md](AGENTS.md)

**Simulated backend walkthrough:** [example/DEMO-WALKTHROUGH.md](example/DEMO-WALKTHROUGH.md)

**CLI commands:** `awc --help` / `awc <command> --help`

<details>
<summary>Why does the release contain both awc and awc-host?</summary>

`awc` is the command invoked by people and business CLIs. `awc-host` is the
Native Messaging bridge started automatically by Chrome; do not run it
manually. They communicate through a user-scoped Named Pipe or private Unix
socket, with no local HTTP/WebSocket port. `awc-host` exits when the extension disconnects.

</details>

<details>
<summary>Development and tests</summary>

```sh
./scripts/build.sh
./scripts/cross-build.sh --pack

go test ./...
go vet ./...
node --test example/ruoyi-cli/test/cli.test.js
```

</details>
