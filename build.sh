#!/bin/bash
# 构建脚本 - 编译前端并嵌入到后端可执行文件中
#
# 用法：
#   ./build.sh                  # 默认 linux/amd64，禁用 CGO，静态二进制
#   GOOS=darwin GOARCH=arm64 ./build.sh
#   GOOS=windows GOARCH=amd64 ./build.sh
#   SKIP_FRONTEND=1 ./build.sh  # 跳过前端构建，仅重新编译后端

set -euo pipefail

# Go 代理（国内加速，需要时取消注释 / 通过环境变量覆盖）
export GOPROXY="${GOPROXY:-https://goproxy.cn,direct}"

# 目标平台（可通过环境变量覆盖）
GOOS="${GOOS:-linux}"
GOARCH="${GOARCH:-amd64}"
CGO_ENABLED="${CGO_ENABLED:-0}"

# 产物名
BIN_NAME="${BIN_NAME:-llsoai-websocket}"

# ====== 路径常量 ======
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WEB_DIR="$SCRIPT_DIR/web"
SERVER_DIR="$SCRIPT_DIR/server"
EMBED_DIR="$SERVER_DIR/cmd/server/web"
BIN_DIR="$SCRIPT_DIR/bin"

mkdir -p "$BIN_DIR"

echo "=========================================="
echo "开始构建 (GOOS=$GOOS GOARCH=$GOARCH CGO_ENABLED=$CGO_ENABLED)"
echo "=========================================="

# ==========================================
# 1. 构建前端
# ==========================================
if [ "${SKIP_FRONTEND:-0}" = "1" ]; then
    echo ""
    echo ">>> 跳过前端构建 (SKIP_FRONTEND=1)"
else
    echo ""
    echo ">>> 构建前端 (web)..."

    cd "$WEB_DIR"

    # 优先使用已有的 node_modules，避免每次都重装；否则用 npm ci，失败再 fallback 到 npm install
    if [ ! -d node_modules ]; then
        npm ci 2>/dev/null || npm install
    fi

    npm run build

    echo ">>> 同步前端产物到 $EMBED_DIR"
    rm -rf "$EMBED_DIR"
    mkdir -p "$EMBED_DIR"
    cp -R dist/. "$EMBED_DIR/"

    cd "$SCRIPT_DIR"
fi

# 确保嵌入目录至少有 index.html，否则 go:embed 会失败
if [ ! -f "$EMBED_DIR/index.html" ]; then
    echo "!!! $EMBED_DIR/index.html 不存在，写入占位文件以满足 go:embed"
    mkdir -p "$EMBED_DIR"
    cat > "$EMBED_DIR/index.html" <<'EOF'
<!DOCTYPE html>
<html><head><meta charset="UTF-8"><title>llsoai-websocket</title></head>
<body><p>Frontend assets not bundled. Run ./build.sh from the repo root.</p></body></html>
EOF
fi

# ==========================================
# 2. 构建后端
# ==========================================
echo ""
echo ">>> 构建后端 (server/cmd/server)..."

cd "$SERVER_DIR"

# 拉取依赖（已存在则忽略）
go mod download 2>/dev/null || true

# 输出文件名（Windows 加 .exe）
OUT_NAME="$BIN_NAME"
if [ "$GOOS" = "windows" ]; then
    OUT_NAME="${BIN_NAME}.exe"
fi

OUT_PATH="$BIN_DIR/$OUT_NAME"
rm -f "$OUT_PATH"

CGO_ENABLED="$CGO_ENABLED" GOOS="$GOOS" GOARCH="$GOARCH" \
    go build -trimpath -ldflags="-s -w" -o "$OUT_PATH" ./cmd/server

cd "$SCRIPT_DIR"

# ==========================================
# 3. 汇总
# ==========================================
SIZE="$(du -h "$OUT_PATH" 2>/dev/null | awk '{print $1}')"

echo ""
echo "=========================================="
echo "构建完成"
echo "=========================================="
echo "二进制: $OUT_PATH (${SIZE:-?})"
echo "目标:   $GOOS/$GOARCH (CGO_ENABLED=$CGO_ENABLED)"
echo ""
echo "提示: 前端已嵌入到后端可执行文件中。"
echo "     运行: $OUT_PATH -config server/config.yaml"
echo ""
