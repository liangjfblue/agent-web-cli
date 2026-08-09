# awc 完整体验流程（手动走一遍）

从零开始，完整走一遍 awc 的安装、配置、使用——模拟一个真实用户的体验。

> 适合：第一次接触 awc、想验证发布流程、准备给别人演示。

---

## ⚠️ 开始前注意

如果你是从仓库目录开发 awc 本身，你的 shell PATH 可能含仓库 `bin/`，
导致 `which awc` 指向本地编译版本而非 `~/.awc/bin`。

**建议：开一个全新的终端窗口走这个流程**，并在里面先跑：
```sh
# 临时去掉仓库 bin，确保测试到的是真正安装的版本
PATH=$(echo "$PATH" | tr ':' '\n' | grep -v "agent-web-cli/bin" | tr '\n' ':')
export PATH
which awc    # 应该显示"未找到"，说明环境干净
```

---

## 第 1 步：安装 awc

```sh
curl -fsSL https://raw.githubusercontent.com/liangjfblue/agent-web-cli/main/install.sh | bash
```

**会发生什么：**
1. 检测你的平台（macOS arm64 / amd64 / Linux ...）
2. 从 GitHub Releases 下载对应二进制（当前 v0.1.0）
3. 解压到 `~/.awc/`（含 `bin/`、`extension/`）
4. 把 `~/.awc/bin` 加进 `~/.zshrc` 的 PATH
5. 自动跑 `awc sys:setup`——注册 native host、配置 PATH、检测扩展

**预期输出末尾：**
```
✓ binaries / key / host / path / extension
all steps complete. run: awc sys:status
```

> 不需要 Node，不需要 sudo。

---

## 第 2 步：让 PATH 生效

install.sh 改了 `~/.zshrc`，但当前终端还没生效。二选一：
```sh
source ~/.zshrc        # 方式 A：当前终端立即生效
# 或直接开一个新终端窗口  # 方式 B
```

验证：
```sh
which awc              # 应显示 /Users/<你>/.awc/bin/awc
awc --version          # awc version 0.1.0
```

---

## 第 3 步：加载 Chrome 扩展（唯一的手动步骤）

Chrome 不允许脚本静默安装未打包扩展，所以这步必须手动。

1. 打开 `chrome://extensions`
2. 右上角打开**「开发者模式」**
3. 点**「加载已解压的扩展程序」**
4. 选择目录：**`~/.awc/extension`**

> 路径展开：macOS 上是 `/Users/<你的用户名>/.awc/extension`
> Finder 里按 `Cmd+Shift+G` 输入 `~/.awc/extension` 能直达

加载后验证：
```sh
awc sys:status
# 应显示: host: 0.1.0 @ /tmp/awc-host-<uid>.sock
```

> 如果报 "host not reachable"：确认扩展已加载且启用，再跑一次 `awc sys:status`。

---

## 第 4 步：启动 demo 后台服务

需要两个终端窗口（或 tab），各跑一个：

```sh
# 终端 1 — 订单后台（localhost:3000）
cd ~/ljf/code/me/web-cli/agent-web-cli
cd example/demo-admin/server && node server.js
```

```sh
# 终端 2 — 服务治理（127.0.0.1:3001，注意不是 localhost）
cd ~/ljf/code/me/web-cli/agent-web-cli
cd example/demo-svcgov/server && node server.js
```

> 首次跑要在各自 server 目录先 `npm install`（装 express）。
>
> **为什么 svcgov 用 127.0.0.1？** Chrome 按 host（而非端口）存 cookie。
> 两个后台都用 localhost 的话，cookie 会互相覆盖，没法同时登录。
> 用不同 host（localhost vs 127.0.0.1）能天然隔离。

---

## 第 5 步：把两个 demo CLI 装成全局命令

```sh
cd ~/ljf/code/me/web-cli/agent-web-cli
cd example/demo-admin/cli && npm link     # 注册 demo-admin 命令
cd ../..
cd example/demo-svcgov/cli && npm link    # 注册 demo-svcgov 命令
```

验证：
```sh
which demo-admin demo-svcgov    # 两个都应有路径
```

---

## 第 6 步：在浏览器登录两个后台

用 Chrome（已装扩展那个）访问。**注意两个 host 不同**（为了 cookie 隔离）：

| 后台 | URL | 账号 |
|---|---|---|
| 订单后台 | http://localhost:3000/login | admin / admin123 |
| 服务治理 | http://127.0.0.1:3001/login | admin / admin123 |

各自点登录。**登录只在这里做一次**，CLI 和 agent 后面复用的就是这个登录态。

> **重要**：svcgov 一定要用 `127.0.0.1` 访问，别写成 `localhost`。
> cookie 存在哪个 host 取决于地址栏写的是什么——写错了会和 demo-admin 冲突。

---

## 第 7 步：配置 awc 登录检测

让 awc 知道"怎么判断你已登录"。有**两种方式，推荐用 skill**。

### 方式 A（推荐）：用 awc-auth-config skill 让 AI agent 配置

awc 自带一个 `awc-auth-config` skill（`sys:setup` 时自动安装），它教 AI agent
怎么读 cookie、识别登录态、写配置。在你的 AI agent（ZCode / Claude Code 等）里
直接说：

```
帮我配一下 demo-admin 的登录，地址 http://localhost:3000/login
帮我配一下 svcgov 的登录，地址 http://127.0.0.1:3001/login
```

agent 会：读浏览器 cookie → 分析哪个是登录态 cookie → 写 `~/.awc/auth/<name>.json` → 验证。

**这是推荐方式**——agent 直接读 live cookie 判断，不依赖"登录前后对比"，
不会卡在 "no new cookies" 的坑里。而且对任何真实后台都通用，你不用懂 cookie 细节。

### 方式 B（手动）：跑 awc auth:config

```sh
awc auth:config demo-admin --url http://localhost:3000/login
awc auth:config svcgov --url http://127.0.0.1:3001/login
```
它通过"登录前后 cookie 对比"来检测登录信号。交互式：打开登录页 → 登录 → 按 Enter。

> **常见坑："no new cookies detected"**。这是因为你本来就登录着，cookie 没变化。
> 解决：先访问 `/logout` 退出，再跑。或干脆用方式 A（skill），它不依赖这个机制。

### 验证

不管哪种方式，配完都要验证：
```sh
awc auth:login demo-admin --check    # logged in ✓
awc auth:login svcgov --check        # logged in ✓
```

---

## 第 8 步：CLI 命令行查询（核心体验）

现在可以直接用命令查后台数据了——不需要再登录，cookie 由 awc 从 Chrome 读：

```sh
# 订单后台
demo-admin dashboard
#   今日订单: 42
#   今日收入: ¥99,800
#   待处理:   3

demo-admin users      # 用户列表

# 服务治理
demo-svcgov services
#   ✓ gateway         healthy
#   ✓ user-service    healthy
#   ⚠ order-service   degraded
#   ✗ notification    down

demo-svcgov logs --service order-service --level error   # 查故障日志
demo-svcgov tables                                      # 数据库表
```

**这一步体现了 awc 的核心价值**：HttpOnly 的 session cookie 页面 JS 读不到，
但 awc 经 chrome.cookies API 能读出来，传给 CLI 当 Cookie 头调 API。

---

## 第 9 步：用 skill 自然语言查询（AI 驱动）

在 AI agent（如 ZCode）里直接说话，不用记命令：

```
/demo-admin 今日订单有多少
/demo-admin 查一下后台数据

/demo-svcgov user-service 服务什么情况
/demo-svcgov order-service 为什么出问题了
/demo-svcgov 数据库有哪些表
```

agent 会读 skill 说明 → 知道该敲哪条命令 → 解析结果回答你。

---

## 第 10 步（可选）：体验登录过期自动恢复

演示 cookie 失效时 agent 怎么自动处理：

1. 浏览器访问 `http://127.0.0.1:3001/logout`（退出，cookie 清掉）
2. 在 agent 里说 `查 services`
3. agent 发现 cookie 失效 → 提示你去登录 → 后台轮询检测 → 自动恢复查询

你不需要再说"登录好了"，agent 会自己检测到。

---

## 故障排查

| 现象 | 原因 | 解决 |
|---|---|---|
| `awc: command not found` | PATH 没生效 | `source ~/.zshrc` 或开新终端 |
| `host not reachable` | 扩展没装/没启用 | 去 chrome://extensions 确认，再 `awc sys:status` |
| `cookie 失效或未登录` | session 过期或服务重启 | 浏览器重新登录，或 `awc auth:login <name>` |
| `auth:config` 报 no new cookies | 本来就登录着 | 先访问 /logout 退出再 config |
| demo CLI 命令找不到 | 没 npm link | 回第 5 步 |
| 端口被占 | server 没正常退出 | `lsof -i:3000` 找 PID，kill |

---

## 卸载（想清掉一切时）

```sh
# 卸全局 demo 命令
npm unlink -g awc-demo-admin awc-demo-svcgov

# 删 awc 本体
rm -rf ~/.awc

# 删 native host manifest
rm -f ~/Library/Application\ Support/Google/Chrome/NativeMessagingHosts/com.awc.host.json

# 清 .zshrc 的 PATH 行（手动删 "# added by awc installer" 那两行）

# Chrome 扩展去 chrome://extensions 手动移除
```
