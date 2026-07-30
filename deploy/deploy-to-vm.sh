#!/bin/bash
set -e

# ============================================================
# DTS Panel 一键部署脚本 (Ubuntu VM)
# 用法: ./deploy-to-vm.sh user@10.211.55.5
# ============================================================

REMOTE="${1:-ubuntu@10.211.55.5}"
REMOTE_USER=$(echo "$REMOTE" | cut -d@ -f1)
REMOTE_HOST=$(echo "$REMOTE" | cut -d@ -f2)
LOCAL_DIR="$(cd "$(dirname "$0")/.." && pwd)"
DTS_BINARY="dts-panel"
REMOTE_INSTALL_DIR="/opt/dts-panel"
REMOTE_DATA_DIR="/home/$REMOTE_USER/.dts-panel"

echo "================================================"
echo "  DTS Panel 一键部署脚本"
echo "  目标: $REMOTE ($REMOTE_HOST)"
echo "  本地: $LOCAL_DIR"
echo "================================================"

# ---------- Step 0: 构建 ARM64 二进制 ----------
echo ""
echo "[0/6] 检查 Go 工具链..."
if ! command -v go &>/dev/null; then
    echo "  ✗ Go 未安装，请先运行: brew install go"
    exit 1
fi
echo "  Go 版本: $(go version)"

echo ""
echo "[1/6] 构建 ARM64 二进制..."
if [ ! -f "$DTS_BINARY" ]; then
    CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o "$DTS_BINARY" ./cmd/panel/
    echo "  ✓ 构建完成: $(ls -lh $DTS_BINARY | awk '{print $5}')"
else
    echo "  ✓ 使用已有二进制: $(ls -lh $DTS_BINARY | awk '{print $5}')"
fi

# ---------- Step 1: 测试 SSH ----------
echo ""
echo "[2/6] 测试 SSH 连接..."
if ! ssh -o ConnectTimeout=5 -o BatchMode=no "$REMOTE" "echo SSH_OK" 2>&1; then
    echo "  ✗ SSH 连接失败，请检查用户/主机/密钥"
    exit 1
fi
echo "  ✓ SSH 连接成功"

# ---------- Step 2: 远程环境探测 ----------
echo ""
echo "[3/6] 探测远程环境..."
REMOTE_INFO=$(ssh "$REMOTE" "
  echo OS:\$(uname -o)
  echo ARCH:\$(uname -m)
  echo KVER:\$(uname -r)
  echo HOME:\$(getent passwd $REMOTE_USER | cut -d: -f6)
  echo USER:\$USER
")
echo "$REMOTE_INFO"

REMOTE_ARCH=$(echo "$REMOTE_INFO" | grep "^ARCH:" | cut -d: -f2)
if [ "$REMOTE_ARCH" != "aarch64" ] && [ "$REMOTE_ARCH" != "arm64" ]; then
    echo "  ⚠ 远程架构是 $REMOTE_ARCH，不是 ARM64，请确认"
fi

# ---------- Step 3: 安装系统依赖 ----------
echo ""
echo "[4/6] 安装系统依赖..."
ssh "$REMOTE" "
  sudo apt-get update -qq
  sudo apt-get install -y -qq \
    curl ca-certificates libstdc++6 libglib2.0-0 \
    libgl1-mesa-dri libgl1 2>&1 | tail -5
  echo DEPS_DONE
"
echo "  ✓ 系统依赖安装完成"

# ---------- Step 4: 部署二进制 ----------
echo ""
echo "[5/6] 部署二进制到 $REMOTE_INSTALL_DIR..."
ssh "$REMOTE" "
  sudo mkdir -p '$REMOTE_INSTALL_DIR' '$REMOTE_DATA_DIR/instances' '$REMOTE_DATA_DIR/mods'
  sudo chown -R $REMOTE_USER:$REMOTE_USER '$REMOTE_DATA_DIR'
"
scp "$DTS_BINARY" "$REMOTE:$REMOTE_INSTALL_DIR/dts-panel"
ssh "$REMOTE" "
  chmod +x '$REMOTE_INSTALL_DIR/dts-panel'
  echo '  ✓ 部署完成'
"

# ---------- Step 5: 系统环境验证 ----------
echo ""
echo "[6/6] 运行系统环境验证..."
ssh "$REMOTE" "
  echo '=== 系统信息 ==='
  echo 'OS: \$(uname -o)'
  echo 'Arch: \$(uname -m)'
  echo 'CPU: \$(lscpu | grep 'Model name' | head -1 | cut -d: -f2 | xargs)'
  echo 'RAM: \$(free -h | grep Mem | awk '{print \$2}')"
  echo 'Disk: \$(df -h / | tail -1 | awk '{print \$4}')"
  echo ''
  echo '=== Go 二进制验证 ==='
  file '$REMOTE_INSTALL_DIR/dts-panel'
  echo ''
  echo '=== 目录结构 ==='
  ls -la '$REMOTE_INSTALL_DIR/'
  echo ''
  echo '=== 依赖库检查 ==='
  for lib in libstdc++.so.6 libGL.so.1 libglib-2.0.so.0; do
    result=\$(ldconfig -p | grep \$lib 2>/dev/null | head -1)
    if [ -n \"\$result\" ]; then
      echo \"  ✓ \$lib found\"
    else
      echo \"  ✗ \$lib MISSING\"
    fi
  done
"

echo ""
echo "================================================"
echo "  部署完成！"
echo "  在 VM 上启动面板:"
echo "    ssh $REMOTE"
echo "    $REMOTE_INSTALL_DIR/dts-panel"
echo "  然后从 Mac 浏览器访问:"
echo "    http://$REMOTE_HOST:8080"
echo "================================================"
