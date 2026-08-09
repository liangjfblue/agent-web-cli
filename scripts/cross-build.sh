#!/usr/bin/env bash
# cross-build.sh — 为所有支持的平台交叉编译 awc + awc-host。
#
# 用法:
#   ./scripts/cross-build.sh                  # 编译全部平台到 dist/
#   ./scripts/cross-build.sh --pack           # 编译 + 每个平台打 npm tarball
#
# Go 的交叉编译不需要 CGO（我们只用标准库），所以可以纯静态编译。
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

VERSION="${VERSION:-$(node -e "console.log(require('./package.json').version)" 2>/dev/null || echo dev)}"

green()  { printf '\033[32m%s\033[0m\n' "$*"; }
red()    { printf '\033[31m%s\033[0m\n' "$*"; }
bold()   { printf '\033[1m%s\033[0m\n' "$*"; }

# 平台列表: "GOOS/GOARCH 描述"
TARGETS=(
    "darwin/arm64   macOS Apple Silicon"
    "darwin/amd64   macOS Intel"
    "linux/amd64    Linux x64"
    "linux/arm64    Linux ARM64"
    "windows/amd64  Windows x64"
)

DIST="$ROOT/dist"
rm -rf "$DIST"
mkdir -p "$DIST"

bold "awc cross-build — version $VERSION"
echo "────────────────────────────────────────"

LDFLAGS_BASE="-s -w"
LDFLAGS_VER="-X github.com/agent/web-cli/internal/cmd.Version=$VERSION -X github.com/agent/web-cli/internal/host.Version=$VERSION"

FAILED=0
BUILT=0

for entry in "${TARGETS[@]}"; do
    # 解析 "GOOS/GOARCH   描述"
    target=$(echo "$entry" | awk '{print $1}')
    desc=$(echo "$entry" | awk '{$1=""; print}' | sed 's/^ *//')
    goos="${target%/*}"
    goarch="${target#*/}"

    ext=""
    [[ "$goos" == "windows" ]] && ext=".exe"

    out_dir="$DIST/awc-${goos}-${goarch}"
    mkdir -p "$out_dir/bin"

    # 复制 extension（每个平台包都要带）
    cp -r "$ROOT/extension" "$out_dir/"
    cp "$ROOT/install-native-host.js" "$out_dir/"
    cp "$ROOT/README.md" "$out_dir/"

    # 生成 package.json（bin 路径适配）
    bin_field='./bin/awc'
    [[ "$goos" == "windows" ]] && bin_field='./bin/awc.exe'
    cat > "$out_dir/package.json" <<EOF
{
  "name": "@agent/web-cli",
  "version": "$VERSION",
  "description": "Agent Web CLI — drive Chrome from the command line",
  "license": "MIT",
  "bin": { "awc": "$bin_field" },
  "files": ["bin/", "extension/", "install-native-host.js", "README.md"],
  "scripts": { "postinstall": "node install-native-host.js" },
  "os": ["$goos"],
  "cpu": ["$goarch"],
  "engines": { "node": ">=14" }
}
EOF

    # 编译 awc 和 awc-host
    if GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 go build \
        -ldflags "$LDFLAGS_BASE $LDFLAGS_VER" \
        -o "$out_dir/bin/awc${ext}" ./cmd/awc 2>&1; then
        :
    else
        red "✗ $desc: awc build failed"
        FAILED=$((FAILED + 1))
        continue
    fi

    if GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 go build \
        -ldflags "$LDFLAGS_BASE $LDFLAGS_VER" \
        -o "$out_dir/bin/awc-host${ext}" ./cmd/host 2>&1; then
        :
    else
        red "✗ $desc: awc-host build failed"
        FAILED=$((FAILED + 1))
        continue
    fi

    # Windows 需要 awc.exe 而非 awc（已在 bin_field 处理）
    # 非 windows 的 awc 设可执行位
    [[ "$goos" != "windows" ]] && chmod +x "$out_dir/bin/awc" "$out_dir/bin/awc-host"

    SIZE=$(du -sh "$out_dir/bin" | cut -f1)
    green "✓ $desc ($SIZE)"
    BUILT=$((BUILT + 1))

    # 可选：打包归档（windows 用 zip，其余用 tar.gz）
    if [[ "${1:-}" == "--pack" ]]; then
        if [[ "$goos" == "windows" ]]; then
            (cd "$DIST" && zip -qr "awc-${goos}-${goarch}-${VERSION}.zip" "awc-${goos}-${goarch}")
            green "  packed: dist/awc-${goos}-${goarch}-${VERSION}.zip"
        else
            (cd "$DIST" && tar czf "awc-${goos}-${goarch}-${VERSION}.tar.gz" "awc-${goos}-${goarch}")
            green "  packed: dist/awc-${goos}-${goarch}-${VERSION}.tar.gz"
        fi
    fi
done

# 生成 checksums.txt（供 install.sh 可选校验 + GitHub Release 附件）
if [[ "${1:-}" == "--pack" ]]; then
    (cd "$DIST" && shasum -a 256 awc-*-*-${VERSION}.tar.gz awc-*-*-${VERSION}.zip 2>/dev/null > checksums-${VERSION}.txt || true)
    [[ -f "$DIST/checksums-${VERSION}.txt" ]] && green "✓ wrote dist/checksums-${VERSION}.txt"
fi

echo "────────────────────────────────────────"
bold "built $BUILT platform(s), $FAILED failed"
echo ""
echo "dist/:"
ls -1 "$DIST/" 2>/dev/null | head -20
echo ""
if [[ "$BUILT" -gt 0 ]]; then
    echo "to test locally on this platform:"
    echo "  ./dist/awc-\$(go env GOOS)-\$(go env GOARCH)/bin/awc --version"
fi
