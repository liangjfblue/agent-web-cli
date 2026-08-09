# awc 使用指南

> 从零开始用上 awc：安装、连通、然后用 AI agent 配置你**自己**的后台登录。

---

## ⚠️ 开始前注意

如果你是从仓库目录开发 awc 本身，你的 shell PATH 可能含仓库 `bin/`，
导致 `which awc` 指向本地编译版本而非 `~/.awc/bin`。

**建议：开一个全新的终端窗口**走这个流程，并在里面先跑：
```sh
PATH=$(echo "$PATH" | tr ':' '\n' | grep -v "agent-web-cli/bin" | tr '\n' ':')
export PATH
which awc    # 应显示"未找到"，说明环境干净
```

---

# 第一部分：核心流程（所有用户必做）

这三步装好 awc，然后用它接入**你自己的**后台。

## 第 1 步：安装 awc

```sh
curl -fsSL https://raw.githubusercontent.com/liangjfblue/agent-web-cli/main/install.sh | bash
```

**会发生什么：**
1. 检测平台（macOS arm64/amd64、Linux x64/arm64）
2. 从 GitHub Releases 下载对应二进制
3. 解压到 `~/.awc/`（含 `bin/`、`extension/`、`awc-auth-config` skill）
4. 把 `~/.awc/bin` 加进 `~/.zshrc` 的 PATH
5. 跑 `awc sys:setup`——注册 native host、安装 skills 到各 AI agent 目录

不需要 Node，不需要 sudo。预期末尾：
```
✓ binaries / key / host / path
finish the manual step(s) above, then run: awc sys:status
```

## 第 2 步：让 PATH 生效 + 加载扩展

```sh
source ~/.zshrc        # 或开新终端
which awc              # 应是 /Users/<你>/.awc/bin/awc
awc --version          # awc version 0.1.0
```

`sys:setup` 已帮你打开了 `chrome://extensions`。加载扩展（**唯一的手动步骤**，Chrome 不允许静默装）：
1. 右上角打开**「开发者模式」**
2. 点**「加载已解压的扩展程序」**
3. 选目录 **`~/.awc/extension`**（Finder 里 `Cmd+Shift+G` 输入这个路径直达）

验证连通：
```sh
awc sys:status         # host: 0.1.0 @ /tmp/awc-host-...
awc tabs:list          # 能列出 Chrome 标签页 = 全链路通
```

## 第 3 步：用 skill 配置你自己的后台登录（核心）

**这是 awc 的主价值**：让 AI agent 配置任意后台的登录检测，之后 CLI 和 agent
都能复用你 Chrome 里的登录态——不用存密码、不用开 headless 浏览器。

`sys:setup` 已把 `awc-auth-config` skill 装进了你的 AI agent 目录。在你的 agent
（ZCode / Claude Code / Cursor 等）里直接说：

```
帮我配一下 <名字> 的登录，地址 <你的登录页 URL>
```

例如：
```
帮我配一下 sysop 的登录，地址 https://sysop.example.com/login
帮我配一下 gitlab 的登录，地址 https://gitlab.example.com/users/sign_in
```

**agent 会自动：**
1. 打开登录页
2. 读浏览器当前的 cookie
3. 如果没登录，让你登录，再读一次
4. 对比找出"登录后才出现的 cookie"（登录态信号）
5. 写配置到 `~/.awc/auth/<名字>.json`
6. 验证 `awc auth:login <名字> --check` 返回 `logged in ✓`

你**不需要懂 cookie 名字、不需要懂 cookie 机制**——agent 全包了。

配好后，任何工具都能用这个登录态：
```sh
# 命令行直接读 cookie 调 API
awc cookies:get --url https://sysop.example.com --header
# 输出: sessionid=abc123; token=xyz789; ...  → 当 Cookie 头传给 curl/fetch

# 或检查登录状态
awc auth:login sysop --check    # logged in ✓
```

---

**到这里，核心流程就完成了。** awc 已连通、你的后台登录已配置好，可以开始用了。

下面的 demo 是可选的——想看 awc 完整工作样例（含业务 CLI + skill 自然语言查询）才走。

---

# 第二部分：体验 demo（可选）

仓库里有两个 demo，展示"awc 接入一个后台"的完整模式：业务 CLI 读 cookie 调 API，
skill 让 agent 用自然语言查询。

> demo 需要 clone 这个仓库。如果你只想用 awc 接入自己的系统，**跳过这部分**。

## demo 概览

| demo | 模拟什么 | 地址 | 全局命令 | skill |
|---|---|---|---|---|
| demo-admin | 订单后台 | localhost:3000 | `demo-admin` | `/demo-admin` |
| demo-svcgov | 服务治理 | 127.0.0.1:3001 | `demo-svcgov` | `/demo-svcgov` |

> **为什么 svcgov 用 127.0.0.1？** Chrome 按 host（不分端口）存 cookie，两个后台
> 都用 localhost 会互相覆盖。用不同 host 天然隔离。你的真实后台部署在不同域名，
> 不会有这个问题。

## D1. 启动 demo 服务

需要两个终端窗口（demo 需要 clone 仓库）：
```sh
# 终端 1 — 订单后台
cd ~/ljf/code/me/web-cli/agent-web-cli
cd example/demo-admin/server && npm install && node server.js

# 终端 2 — 服务治理（注意 127.0.0.1）
cd ~/ljf/code/me/web-cli/agent-web-cli
cd example/demo-svcgov/server && npm install && node server.js
```

## D2. 安装 demo CLI

```sh
cd ~/ljf/code/me/web-cli/agent-web-cli
cd example/demo-admin/cli && npm link && cd ../..
cd example/demo-svcgov/cli && npm link && cd ../..
which demo-admin demo-svcgov    # 都应有路径
```

## D3. 浏览器登录两个后台

**注意 host 不同**（cookie 隔离）：
- demo-admin：`http://localhost:3000/login` → admin / admin123
- demo-svcgov：`http://127.0.0.1:3001/login` → admin / admin123

> svcgov 一定要用 `127.0.0.1`，别写成 localhost。

## D4. 配置登录检测

和第 3 步一样，**推荐用 skill**。在 agent 里：
```
帮我配一下 demo-admin 的登录，地址 http://localhost:3000/login
帮我配一下 svcgov 的登录，地址 http://127.0.0.1:3001/login
```

或手动：`awc auth:config demo-admin --url http://localhost:3000/login`
（已登录的话会报 "no new cookies"，先访问 `/logout` 退出再跑）

验证：
```sh
awc auth:login demo-admin --check    # logged in ✓
awc auth:login svcgov --check        # logged in ✓
```

## D5. CLI 命令行查询

```sh
demo-admin dashboard
#   今日订单: 42 / 今日收入: ¥99,800 / 待处理: 3

demo-svcgov services
#   ✓ gateway healthy  ⚠ order-service degraded  ✗ notification down

demo-svcgov logs --service order-service --level error   # 查故障日志
```

## D6. 用 skill 自然语言查询（AI 驱动）

在 agent 里直接说话，不用记命令：
```
/demo-admin 今日订单有多少
/demo-svcgov order-service 为什么出问题了
/demo-svcgov 数据库有哪些表
```

---

# 故障排查

| 现象 | 原因 | 解决 |
|---|---|---|
| `awc: command not found` | PATH 没生效 | `source ~/.zshrc` 或开新终端 |
| `host not reachable` | 扩展没装/没启用 | chrome://extensions 确认，再 `awc sys:status` |
| `cookie 失效或未登录` | session 过期/服务重启 | 浏览器重新登录 |
| `auth:config` 报 no new cookies | 本来就登录着 | 先 `/logout` 退出，或用 skill 配置 |
| `auth:login --check` 误报已登录 | cookie 残留但服务端已失效 | 重新登录（真实后台用持久化 session 不会这样） |
| demo 两个后台 cookie 冲突 | 都用 localhost | svcgov 用 127.0.0.1 |

---

# 卸载

```sh
npm unlink -g awc-demo-admin awc-demo-svcgov   # demo CLI
rm -rf ~/.awc                                   # awc 本体 + 配置 + skills
rm -f ~/Library/Application\ Support/Google/Chrome/NativeMessagingHosts/com.awc.host.json
# .zshrc 手动删 "# added by awc installer" 那两行
# Chrome 扩展去 chrome://extensions 手动移除
```
