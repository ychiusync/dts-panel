#!/bin/bash
set -e
echo "=== DTS Panel Docker Entrypoint ==="
echo "数据目录: $DST_DATA_DIR"
echo "游戏目录: $DST_GAME_DIR"
echo "面板端口: $DST_PANEL_PORT"

# 创建必要目录
mkdir -p "$DST_DATA_DIR" "$DST_DATA_DIR/instances" "$DST_DATA_DIR/mods"

# 启动
exec ./dts-panel "$@"
