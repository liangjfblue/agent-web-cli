<div align="center">

<img src="assets/awc-readme-cover-v2.png" alt="Agent Web CLI cover">

**[English](README.en.md)** · 简体中文

</div>

`awc` 让你的**命令行工具和 AI agent 复用 Chrome 的登录态**——不需要 headless
浏览器、不需要单独开浏览器、不需要存密码。它通过一个小扩展，从你**已经登录的
Chrome** 里实时读取 cookie，还能驱动 Chrome（标签页、DOM、网络、截图）。

```
你的 CLI / AI agent ──► awc ──► Chrome（你已经登录的那个）
                        通过 chrome.cookies API 读 HttpOnly cookie
```

典型场景：你在 Chrome 里登录了某个内部系统；`awc` 读出那个 session cookie，
让脚本或 agent 能调用该系统的鉴权 API——你不需要在任何地方重新输密码。

## 安装

**macOS / Linux**（一行装好，不需要 Node）：

```sh
curl -fsSL https://raw.githubusercontent.com/liangjfblue/agent-web-cli/main/install.sh | bash
```

安装器会检测平台、从[最新 release](https://github.com/liangjfblue/agent-web-cli/releases)
下载二进制、装到 `~/.awc/bin`、加进 PATH，并跑 `awc sys:setup`（注册 native host
+ 给你的 AI agent 装 `awc-auth-config` skill）。

> **Windows**：从 [Releases](https://github.com/liangjfblue/agent-web-cli/releases)
> 下载 `awc-windows-amd64-<ver>.zip`，解压后跑 `awc sys:setup`。或用 WSL 跑上面的命令。

**然后加载 Chrome 扩展**（唯一的手动步骤——Chrome 不允许静默安装未打包扩展）：

1. 打开 `chrome://extensions` → 右上角开启**开发者模式**
2. 点**加载未打包的扩展程序** → 选 `~/.awc/extension` 目录
3. 验证：`awc sys:status`（应显示 host 已连接）

## 快速上手：复用一个登录态

### 方式 A — 让 AI agent 配置（推荐）

`sys:setup` 会把 `awc-auth-config` skill 装进你的 AI agent 目录（ZCode、Claude
Code、Cursor、Codex）。在你的 agent 里直接说：

```
帮我配一下 <名字> 的登录，地址 <登录页 URL>
```

例如：`帮我配一下 sysop 的登录，地址 https://sysop.example.com/login`

agent 会打开网站、读 cookie、需要时让你登录、识别哪个 cookie 代表"已登录"、写配置、
验证。**你完全不需要知道 cookie 名字。**

### 方式 B — 直接读 cookie

```sh
# 读 cookie，输出为 HTTP Cookie 头格式
awc cookies:get --url "https://api.example.com" --header
# → sessionid=abc123; token=xyz789; ...

# 配合 curl / 任何 HTTP 客户端用
COOKIE=$(awc cookies:get --url "https://api.example.com" --header)
curl -H "Cookie: $COOKIE" "https://api.example.com/api/data"
```

cookie 每次都从 Chrome 实时读取——不缓存、不落盘。

### 检查登录态

```sh
awc auth:login <名字> --check    # 瞬时返回："logged in ✓" / "not logged in ✗"
```

> `auth:login`（不带 `--check`）可能阻塞最多 120 秒等你手动登录——只在交互式
> 场景用，**绝不要**用在 cron / CI / 后台服务里。

## 命令一览

```
系统      sys:status | sys:doctor | sys:install | sys:uninstall
Cookie   cookies:get [--url --name --header --json]
登录      auth:login <名字> [--check] | auth:config <名字> --url <u> | auth:list
标签页    tabs:list | tabs:open <url> [--foreground] | tabs:focus <id>
DOM      dom:snapshot | dom:click | dom:type | dom:query | dom:text
          定位: --anchor | --selector | --text | --role | --name | --label | --testid
截图      shot:page [-o 文件] [--tab-id]
网络      net:watch | net:debug | net:stop | net:body
控制台    console:watch [--level] | console:clear
CDP      cdp:send <方法> [--params] | cdp:listen [--event]
等待      wait:for [--selector | --text | --url-pattern | --status]
```

全局参数：`--json`（输出原始 JSON）、`--timeout 10s`（单次调用超时）。

跑 `awc --help` 看完整列表，`awc <命令> --help` 看某命令的参数。

完整的集成指南（cookie 用法、登录处理、每个命令的细节）见
**[AGENTS.md](AGENTS.md)**——它是写给 AI agent 的，但也是最详尽的程序化使用参考。

## cookie 过期处理

cookie 会过期或被吊销。每个 `awc` 命令都是单一职责——`cookies:get` 瞬时返回
（绝不自动登录），所以在脚本里用是安全的。显式处理过期：

```sh
COOKIE=$(awc cookies:get --url "https://api.example.com" --header)
resp=$(curl -s -H "Cookie: $COOKIE" "https://api.example.com/api/data")
# API 拒绝了 cookie → 提示重新登录
echo "$resp" | grep -q '"code":401' && echo "请重新登录: awc auth:login <名字>"
```

> **cookie 存在 ≠ cookie 有效。** `auth:login --check` 只检查 Chrome 里有没有这个
> cookie；API 的响应才是判断 cookie 是否有效的唯一权威。

## Demo

两个完整 demo（业务 CLI + skill）在 [`example/`](example/)——一个订单后台、一个
服务治理平台。它们展示了完整模式：awc 读 cookie → 业务 CLI 调鉴权 API → skill 让
agent 用自然语言查询。见 [`example/DEMO-WALKTHROUGH.md`](example/DEMO-WALKTHROUGH.md)。

---

# 开发者 / 贡献者

## 从源码构建

```sh
./scripts/build.sh                  # 当前平台 → bin/awc + bin/awc-host
./scripts/build.sh --pack           # + npm tarball
./scripts/cross-build.sh --pack     # 全平台 → dist/awc-<os>-<arch>-<ver>.tar.gz
VERSION=v0.2.0 ./scripts/build.sh   # 指定版本号
```

版本号来源：`$VERSION` 环境变量 > git tag > `package.json`。通过 `-ldflags` 注入到
`awc --version`。

## 工作原理（架构）

```
awc CLI ──AW 帧 (msgpack+CRC16)──► Go host ──native messaging──► Chrome 扩展 ──chrome.*──► 页面
```

不开 HTTP/WebSocket 服务，不开本地端口。

**CLI ↔ host**（Unix socket / named pipe）——每帧：
```
| magic "AW" | ver u16=1 | payload 长度 uint32 BE | msgpack payload | crc16 |
```
`crc16` = 对 payload 算 CRC-16/CCITT-FALSE，大端。

**host ↔ 扩展**（native messaging）——4 字节小端长度前缀 + UTF-8 JSON：
```
→ { tid, op, args? }    ← { tid, ok, data?, code?, msg? }
```

## 项目结构

```
agent-web-cli/
├── cmd/awc/        # CLI 入口 (cobra)
├── cmd/host/       # native-messaging host 入口
├── internal/
│   ├── proto/      # AW 帧编解码 + msgpack + CRC16
│   ├── ipc/        # Unix socket / named pipe 客户端
│   ├── host/       # 桥接 CLI socket ↔ native messaging
│   ├── cmd/        # cobra 命令树
│   └── install/    # native-messaging manifest + 注册
├── extension/      # Chrome 扩展 (MV3)
└── go.mod
```

## 测试

```sh
go test ./...
```

## native-messaging host 注册

`awc sys:install` 写入 Chrome native-messaging manifest：

| 系统 | 位置 |
|---|---|
| macOS | `~/Library/Application Support/Google/Chrome/NativeMessagingHosts/com.awc.host.json` |
| Linux | `~/.config/google-chrome/NativeMessagingHosts/com.awc.host.json` |
| Windows | 注册表 `HKCU\Software\Google\Chrome\NativeMessagingHosts\com.awc.host` |

卸载用 `awc sys:uninstall`。
