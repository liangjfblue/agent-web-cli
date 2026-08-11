# awc 集成演示

三个 demo，展示如何用 **awc** 把浏览器的登录态接到命令行工具——让 CLI 和
AI agent 复用你**已经登录的 Chrome**，无需 headless 浏览器，无需存储密码。

```
                    awc 的位置
        ┌─────────────────────────────────┐
        │                                 ▼
┌────────────────┐   chrome.cookies   ┌─────────────┐   Cookie header   ┌────────────────┐
│  Chrome        │ ◄───────────────── │  awc        │ ────────────────► │  server        │
│  (已登录)      │                    │  (读cookie) │                   │  (后台API)     │
│  HttpOnly cookie│                   └─────────────┘                   └────────────────┘
        ▲                                 │
        │                          ┌──────┴──────┐
        │                          ▼             ▼
        │                   ┌────────────┐  ┌──────────────────┐
        │                   │  cli       │  │  skill (SKILL.md)│
        │                   │  命令行工具 │  │  让 AI 懂怎么用  │
        │                   └────────────┘  └──────────────────┘
        │                         ▲                  ▲
        │                         │                  │
        └─────────────────────────┴──────────────────┘
              用户：登录一次，CLI 和 agent 都能复用
```

## 三个角色

每个 demo 由三部分组成，对应接入 awc 时你的三种工作：

| 角色 | 目录 | 职责 | 是不是你要写的 |
|---|---|---|---|
| **server** | `*/server/` | 一个需要登录的后台（被访问的对象） | ❌ 替身——代表你**已有的系统** |
| **cli** | `*/cli/` | 用 awc 读 cookie、调用 server API 的命令行工具 | ✅ 你要**新写**的接入层 |
| **skill** | `*/skill/` | 一份说明，告诉 AI agent 这个 cli 怎么用 | ✅ 你要**新写**的接入层 |

> **关键洞察**：server 不是 awc 集成的重点——它只是"任意一个需要登录的已有后台"。
> 真正展示"如何接入 awc"的是 **cli + skill**。注意 `cli/` 的 package.json 是
> **零依赖**的（只用 Node 内置的 `child_process` + `fetch`）。业务 CLI 通过
> `awc session:acquire ... --json` 获取稳定的会话契约，不解析面向人的输出。

## 三个 demo

| demo | 模拟什么 | 端口 | 全局命令 | skill |
|---|---|---|---|---|
| **demo-admin** | 订单后台（dashboard / users） | 3000 | `demo-admin` | `/demo-admin` |
| **demo-svcgov** | 服务治理平台（services / logs / database） | 3001 | `demo-svcgov` | `/demo-svcgov` |
| **ruoyi-cli** | 真实 RuoYi 后台（用户 / 角色，只读） | HTTPS | `ruoyi` | `ruoyi-admin` |

---

## 一次性准备

### 1. 安装并连通 awc

```sh
# macOS / Linux（一行装好，不需要 Node）
curl -fsSL https://raw.githubusercontent.com/liangjfblue/agent-web-cli/main/install.sh | bash
# 它会自动跑 awc sys:setup（注册 host、配置 PATH、安装 skills）
awc sys:status         # 应显示 host + extension connected
```

> Windows 用户：从 [Releases](https://github.com/liangjfblue/agent-web-cli/releases)
> 下载 `awc-windows-amd64-<ver>.zip` 解压后跑 `awc sys:setup`，或用 WSL。
> 开发者也可从源码构建：`cd agent-web-cli && ./scripts/build.sh`。

`sys:setup` 会打印一个扩展目录路径——去 `chrome://extensions`（开启开发者模式）
加载它。这是唯一的手动步骤。

### 2. 启动两个后台服务

```sh
cd example/demo-admin/server && npm install && node server.js    # 端口 3000
cd example/demo-svcgov/server && npm install && node server.js   # 127.0.0.1:3001
```

### 3. 把两个 CLI 安装为全局命令

```sh
cd example/demo-admin/cli && npm link    # 注册 demo-admin
cd example/demo-svcgov/cli && npm link   # 注册 demo-svcgov
which demo-admin demo-svcgov             # 确认已注册
```

### 4. 在 Chrome 登录 + 配置 awc 登录检测

**登录**（两个 host 不同，为了 cookie 隔离）：
- demo-admin：`http://localhost:3000/login` → admin / admin123
- demo-svcgov：`http://127.0.0.1:3001/login` → admin / admin123

> **为什么 svcgov 用 127.0.0.1？** Chrome 按 host（不分端口）存 cookie。
> 都用 localhost 会互相覆盖。用不同 host 天然隔离。

**配置登录检测**（推荐用 skill）：

最简单的方式——在你的 AI agent 里说：
```
帮我配一下 demo-admin 的登录，地址 http://localhost:3000/login
帮我配一下 svcgov 的登录，地址 http://127.0.0.1:3001/login
```
agent 会用 `awc-auth-config` skill 自动读 cookie、识别登录态、写配置。

或手动跑：
```sh
awc auth:config demo-admin --url http://localhost:3000/login
awc auth:config svcgov --url http://127.0.0.1:3001/login
```

---

## 演示流程

### Demo 1：订单后台（demo-admin）

**手动命令行查询：**

```sh
demo-admin dashboard       # 今日订单 42 / 收入 ¥99,800 / 待处理 3
demo-admin users           # 用户列表
demo-admin status          # 检查登录态
```

**用 AI 自然语言查询（skill 的价值）：**

```
/demo-admin 今日订单有多少
/demo-admin 查一下后台数据
```

→ agent 读 skill → 知道该敲 `demo-admin dashboard` → 解析结果回答你。

### Demo 2：服务治理（demo-svcgov）

**手动命令行查询：**

```sh
demo-svcgov services                              # 服务状态总览
demo-svcgov logs --service gateway --level error  # gateway 的错误日志
demo-svcgov tables                                # 数据库表
demo-svcgov table users                           # users 表数据预览
```

**用 AI 自然语言查询：**

```
/demo-svcgov gateway有没有报错
/demo-svcgov 数据库有哪些表
/demo-svcgov 服务状态怎么样
```

---

## 这个演示想说明什么

1. **HttpOnly cookie 不是障碍**。session cookie 是 HttpOnly，页面 JS 读不到，但 awc
   通过 `chrome.cookies` 扩展 API 能读——这是 awc 的核心价值：复用浏览器里那些对
   页面级工具不可见的登录态。

2. **接入 awc 极轻量**。看 `cli/` 目录：零外部依赖，核心是调用
   `awc session:acquire <name> --url ... --json`，把进程内的 `cookieHeader` 传给
   `fetch`。CLI 同时获得稳定退出码、Profile 选择和显式登录恢复，不需要自己拼登录流程。

3. **CLI + skill 是完整模式**。CLI 让人用，skill 让 AI 用。同一个后台，一套登录态，
   两种使用方式——你写了 cli，再写一份 skill，AI agent 就能用自然语言驱动它。

---

## 跨平台提示

skill 通过**符号链接**让 `.agents/skills/` 发现 `example/*/skill/` 里的 SKILL.md：

```
.agents/skills/demo-admin    → ../../example/demo-admin/skill
.agents/skills/demo-svcgov   → ../../example/demo-svcgov/skill
```

- macOS / Linux：开箱即用，git 也跟踪符号链接。
- **Windows**：默认不创建符号链接。两种处理方式：
  - 以开发者模式运行 git（Win10 1703+ 支持），或
  - 把 `example/*/skill/SKILL.md` 复制一份到 `.agents/skills/<name>/SKILL.md`（放弃自包含，换兼容性）

## 登录态过期怎么办

session cookie 会过期或被服务端吊销。Cookie 缺失时，显式打开 Chrome 等待登录：

```sh
awc session:acquire demo-admin --url http://localhost:3000 --interactive --json
awc session:acquire svcgov --url http://127.0.0.1:3001 --interactive --json
```

Cookie 仍存在但 API 已拒绝它时，使用 `--refresh` 清除配置指定的认证 Cookie 后重登：

```sh
awc session:acquire demo-admin --url http://localhost:3000 --interactive --refresh --json
```

> 注意：`--interactive` 会阻塞等待用户完成登录，只在有人操作的终端使用；自动化脚本、
> CI 和后台服务必须保持非交互，让退出码 `10` 交给上层处理。不要打印或保存成功 JSON
> 中的 `cookieHeader`。
