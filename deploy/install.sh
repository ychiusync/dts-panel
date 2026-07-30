#!/bin/bash
set -e

# DTS Panel 一键安装脚本
# 适用于 Oracle ARM (aarch64) / Ubuntu / Debian

BINARY_URL="${1:-}"
INSTALL_DIR="/opt/dts-panel"
DATA_DIR="$HOME/.dts-panel"

echo "=== DTS Panel 一键安装脚本 ==="

# 检测架构
ARCH=$(uname -m)
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$ARCH" in
    aarch64) GOARCH="arm64" ;;
    x86_64)  GOARCH="amd64" ;;
    i686)    GOARCH="386" ;;
    *)       echo "不支持的架构: $ARCH"; exit 1 ;;
esac

echo "检测到: $OS/$GOARCH"

# 1. 下载二进制
if [ -n "$BINARY_URL" ]; then
    echo "从指定 URL 下载: $BINARY_URL"
    curl -sL "$BINARY_URL" -o dts-panel
else
    echo "请从 GitHub Release 下载对应架构的二进制文件，然后指定 URL:"
    echo "  $0 https://github.com/.../releases/download/v1.0.0/dts-panel-linux-arm64"
    exit 1
fi

# 2. 安装
sudo mkdir -p "$INSTALL_DIR"
sudo cp dts-panel "$INSTALL_DIR/"
sudo chmod +x "$INSTALL_DIR/dts-panel"
rm -f dts-panel

# 3. 创建目录
sudo mkdir -p "$DATA_DIR" "$DATA_DIR/instances" "$DATA_DIR/mods"
sudo chown -R "$USER:$USER" "$DATA_DIR"

# 4. 安装 systemd 服务
sudo cp "$INSTALL_DIR/../dts-panel.service" /etc/systemd/system/ || \
    cp deploy/systemd/dts-panel.service /etc/systemd/system/ 2>/dev/null || true
sudo systemctl daemon-reload

echo ""
echo "安装完成！"
echo ""
echo "手动启动:"
echo "  sudo systemctl enable dts-panel"
echo "  sudo systemctl start dts-panel"
echo ""
echo "直接运行:"
echo "  $INSTALL_DIR/dts-panel"
echo ""
echo "打开浏览器: http://localhost:8080"
