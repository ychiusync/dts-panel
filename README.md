# 🌲 DTS Panel — Don't Starve Together 服务器管理面板

> 支持 Oracle ARM (aarch64) 架构的饥荒联机版 (DST) 专用服务器管理面板，提供 Web 控制台和 CLI 双模式管理。

## 功能概览

| 模块 | 功能 |
|------|------|
| **系统环境** | 检测 CPU 架构、OS 版本、依赖库；一键安装系统依赖 |
| **SteamCMD 安装** | 自动下载并安装 SteamCMD，支持 ARM64 架构 |
| **游戏安装** | 通过 SteamCMD 下载/更新 DST Dedicated Server (AppID 108730) |
| **实例管理** | 创建/启动/停止/重启/删除 多个 DST 服务器实例 |
| **房间管理** | 配置世界生成参数、地图种子、难度、昼夜长度等 |
| **模组管理** | 添加/启用/禁用/删除 Steam Workshop 模组 |
| **Web 面板** | 实时监控、日志查看、状态管理 |
| **CLI 工具** | 命令行模式，适合无 GUI 服务器 |

## 架构

```
┌─────────────┐    HTTP/Web     ┌──────────────────┐    Linux ARM64    ┌──────────────────┐
│  浏览器      │ ◄────────────► │  DTS Panel       │ ◄──────────────► │  DST Dedicated   │
│  (管理界面)  │                │  (Go 单二进制)    │                  │  Server          │
└─────────────┘                └────────┬─────────┘                  └──────────────────┘
                                        │
                              ┌─────────┴─────────┐
                              │  SQLite (dts.db)   │
                              │  instances        │
                              │  rooms / mods      │
                              └────────────────────┘
```

## 环境要求

- **服务器**: Oracle ARM (aarch64) / Ubuntu 20.04+ / Debian 11+
- **CPU**: ARM64 架构 (Oracle Ampere Altra)
- **RAM**: 建议 ≥ 2GB (每实例约 500MB-1GB)
- **存储**: DST 游戏约 2GB，模组额外空间
- **依赖**: glibc, libstdc++, libgl1-mesa-dri, curl

## 快速开始

### 方式一: 下载预编译二进制

从 [GitHub Releases](https://github.com/dts-panel/dts-panel/releases) 下载对应架构的二进制:

```bash
# Oracle ARM (aarch64)
wget https://github.com/dts-panel/dts-panel/releases/download/v1.0.0/dts-panel-linux-arm64
chmod +x dts-panel-linux-arm64
./dts-panel-linux-arm64
```

### 方式二: Docker

```bash
# 构建并运行
docker compose -f deploy/docker/docker-compose.yml up -d

# 访问面板
# http://<服务器IP>:8080
```

### 方式三: 源码编译

```bash
# 需要 Go 1.22+
git clone https://github.com/dts-panel/dts-panel.git
cd dts-panel

# ARM64 交叉编译
make build-arm64

# 运行
./dts-panel-linux-arm64
```

## Oracle ARM 部署指南

### 1. 安装系统依赖

```bash
sudo apt-get update
sudo apt-get install -y glibc libstdc++6 libgl1-mesa-dri curl
```

### 2. 部署 DTS Panel

```bash
# 上传二进制
scp dts-panel-linux-arm64 user@oracle-arm:/opt/dts-panel/
```

### 3. 通过 systemd 管理

```bash
# 上传服务文件
scp deploy/systemd/dts-panel.service /etc/systemd/system/

# 启用并启动
sudo systemctl daemon-reload
sudo systemctl enable dts-panel
sudo systemctl start dts-panel

# 查看状态
sudo systemctl status dts-panel
```

### 4. 安装 SteamCMD 和游戏

```bash
# 通过 CLI
./dts-panel -cmd install steamcmd   # 安装 SteamCMD
./dts-panel -cmd install game       # 安装 DST Dedicated Server

# 或通过 Web 面板 -> 游戏安装
```

### 5. 创建并启动实例

```bash
./dts-panel -cmd instance create my-world
./dts-panel -cmd instance start 1
```

## 项目结构

```
dts-panel/
├── cmd/
│   ├── panel/              # Web 面板入口 (http://0.0.0.0:8080)
│   └── cli/                # CLI 工具 (dts-panel -cmd ...)
├── internal/
│   ├── api/                # HTTP 路由 & 处理器
│   ├── config/             # 配置加载 (config.json)
│   ├── db/                 # SQLite 数据层 & 迁移
│   ├── install/            # SteamCMD & 游戏安装
│   ├── instance/           # 实例生命周期管理
│   ├── mod/                # 模组管理
│   ├── process/            # 进程监控
│   └── room/               # 房间/世界配置
├── templates/              # HTML 模板
├── static/                 # CSS / JS
├── deploy/
│   ├── docker/             # Dockerfile, docker-compose
│   ├── systemd/            # systemd 服务文件
│   └── install.sh          # 一键安装脚本
├── .github/
│   └── workflows/
│       └── build.yml       # CI/CD: ARM64 + AMD64 交叉编译
├── Makefile
├── go.mod
└── README.md
```

## 开发

### 本地开发 (Mac M1 Pro)

```bash
# 交叉编译到 Linux ARM64
make build-arm64

# 通过 SSH 验证 (Ubuntu VM)
scp dts-panel-linux-arm64 ubuntu@192.168.x.x:/tmp/
ssh ubuntu@192.168.x.x "/tmp/dts-panel-linux-arm64"
```

### 在 Oracle ARM 服务器上验证

```bash
# SSH 到 Oracle ARM
ssh user@oracle-arm
./dts-panel-linux-arm64
```

### 运行测试

```bash
make test
```

## 配置

面板首次运行时会在 `$HOME/.dts-panel/config.json` 生成配置文件:

```json
{
  "data_dir": "/root/.dts-panel",
  "game_install_dir": "/root/dst-server",
  "instance_root": "/root/.dts-panel/instances",
  "panel_host": "0.0.0.0",
  "panel_port": 8080,
  "default_master_port": 10999,
  "default_cluster_port": 11000,
  "default_max_players": 10
}
```

## 许可

MIT License

## 贡献

欢迎提交 Issue 和 Pull Request！
