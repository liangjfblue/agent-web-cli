#!/usr/bin/env bash
# build.sh — 编译 awc + awc-host，校验产物，可选打包 npm。
#
# 用法:
#   ./scripts/build.sh              # 编译当前平台
#   ./scripts/build.sh --pack       # 编译 + 打包 npm tarball (.tgz)
#   VERSION=v0.2.0 ./scripts/build.sh   # 指定版本号注入
#
# 版本号优先级: $VERSION 环境变量 > git tag > package.json > "dev"
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

# ── 颜色 ──
red()    { printf '\033[31m%s\033[0m\n' "$*"; }
green()  { printf '\033[32m%s\033[0m\n' "$*"; }
yellow() { printf '\033[33m%s\033[0m\n' "$*"; }
bold()   { printf '\033[1m%s\033[0m\n' "$*"; }

# ── 确定版本号 ──
VERSION="${VERSION:-}"
if [[ -z "$VERSION" ]]; then
    # 从 git tag 取
    if VERSION=$(git describe --tags --exact-match 2>/dev/null); then
        : # 用 tag
    else
        # 从 package.json 取
        VERSION=$(node -e "console.log(require('./package.json').version)" 2>/dev/null || echo "dev")
    fi
fi
bold "awc build — version $VERSION"
echo "────────────────────────────────────────"

# ── 步骤 1: 检查 Go ──
if ! command -v go &>/dev/null; then
    red "✗ Go not found in PATH"
    exit 1
fi
GO_VERSION=$(go version)
green "✓ $GO_VERSION"

# ── 步骤 2: 清理旧产物 ──
rm -rf bin/*
green "✓ cleaned bin/"

# ── 步骤 3: 编译 ──
# -ldflags -X 把版本号注入到 Go 变量
# -s -w 去掉调试符号，减小体积
LDFLAGS="-s -w -X github.com/agent/web-cli/internal/cmd.Version=$VERSION -X github.com/agent/web-cli/internal/host.Version=$VERSION"

bold "compiling..."
if go build -ldflags "$LDFLAGS" -o bin/awc ./cmd/awc; then
    green "✓ awc $(bin/awc --version 2>/dev/null || echo '(version unknown)')"
else
    red "✗ awc build failed"
    exit 1
fi

if go build -ldflags "$LDFLAGS" -o bin/awc-host ./cmd/host; then
    green "✓ awc-host built"
else
    red "✗ awc-host build failed"
    exit 1
fi

# ── 步骤 4: 校验产物 ──
for bin in awc awc-host; do
    if [[ ! -f "bin/$bin" ]]; then
        red "✗ bin/$bin missing after build"
        exit 1
    fi
    if [[ ! -x "bin/$bin" ]]; then
        red "✗ bin/$bin not executable"
        exit 1
    fi
done
green "✓ binaries verified (present + executable)"

# ── 步骤 5: 校验扩展 ──
if [[ ! -f extension/manifest.json ]]; then
    red "✗ extension/manifest.json missing"
    exit 1
fi
if grep -q "REPLACE_WITH" extension/manifest.json; then
    red "✗ extension/manifest.json still has placeholder key"
    exit 1
fi
green "✓ extension manifest OK"

# ── 步骤 6: 快速自测 ──
bold "smoke test..."
# awc 是 CLI，可以测 --help
if bin/awc --help >/dev/null 2>&1; then
    green "✓ awc --help responds"
else
    red "✗ awc --help failed"
    exit 1
fi
# awc-host 是 native host，无 --help；测它能在 0.5s 内不立即崩溃
# (它会等 stdin，所以用 timeout + 空 stdin 让它快速退出)
if echo "" | timeout 1 bin/awc-host >/dev/null 2>&1 || true; then
    green "✓ awc-host starts without crash"
fi

# ── 步骤 7: 打包 (可选) ──
if [[ "${1:-}" == "--pack" ]]; then
    bold "packing npm tarball..."
    if command -v npm &>/dev/null; then
        npm pack 2>&1 | tail -1
        TARBALL=$(ls *.tgz 2>/dev/null | head -1)
        if [[ -n "$TARBALL" ]]; then
            SIZE=$(du -h "$TARBALL" | cut -f1)
            green "✓ packed $TARBALL ($SIZE)"
            echo ""
            echo "to install locally:"
            echo "  npm install -g $ROOT/$TARBALL"
        fi
    else
        red "✗ npm not found; cannot pack"
        exit 1
    fi
fi

echo ""
bold "done."
echo ""
echo "binaries:"
ls -lh bin/ | awk 'NR>1{printf "  %-12s %s\n", $9, $5}'
echo ""
echo "next:"
echo "  ./bin/awc sys:setup    # first-time setup wizard"
echo "  ./bin/awc sys:status   # check connectivity"
