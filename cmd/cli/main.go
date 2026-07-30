package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/dts-panel/dts-panel/internal/config"
	"github.com/dts-panel/dts-panel/internal/db"
	"github.com/dts-panel/dts-panel/internal/install"
	"github.com/dts-panel/dts-panel/internal/instance"
	"github.com/dts-panel/dts-panel/internal/mod"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	installCmd := flag.NewFlagSet("install", flag.ExitOnError)
	instanceCmd := flag.NewFlagSet("instance", flag.ExitOnError)
	modCmd := flag.NewFlagSet("mod", flag.ExitOnError)

	var subCommand string
	flag.StringVar(&subCommand, "cmd", "", "子命令: install, instance, mod")
	flag.Parse()

	cfg := config.Load()

	database, err := db.Init(cfg.DataDir)
	if err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}
	defer database.Close()
	_ = db.Migrate(database)

	installer := install.NewSteamCMDInstaller(cfg.DataDir, cfg.SteamCMDPath, cfg.GameInstallDir)
	instMgr := instance.NewManager(database, cfg.InstanceRoot, cfg.GameInstallDir)
	modMgr := mod.NewManager(database, cfg.DataDir+"/mods", cfg.InstanceRoot)

	switch subCommand {
	case "install":
		installCmd.Parse(os.Args[2:])
		switch installCmd.Arg(0) {
		case "deps":
			checker := install.NewSystemDepChecker()
			if err := checker.CheckAndInstall(true); err != nil {
				log.Fatalf("依赖检查/安装失败: %v", err)
			}
		case "steamcmd":
			if err := installer.InstallSteamCMD(); err != nil {
				log.Fatalf("SteamCMD 安装失败: %v", err)
			}
		case "game":
			if err := installer.InstallDSTServer(); err != nil {
				log.Fatalf("游戏安装失败: %v", err)
			}
		case "update":
			if err := installer.UpdateDSTServer(); err != nil {
				log.Fatalf("游戏更新失败: %v", err)
			}
		case "verify":
			if err := installer.Verify(); err != nil {
				log.Fatalf("验证失败: %v", err)
			}
			fmt.Println("✓ 验证通过")
		default:
			log.Println("用法: dts-panel -cmd install <deps|steamcmd|game|update|verify>")
		}

	case "instance":
		instanceCmd.Parse(os.Args[2:])
		switch instanceCmd.Arg(0) {
		case "list":
			instances, err := instMgr.List()
			if err != nil {
				log.Fatalf("获取实例失败: %v", err)
			}
			fmt.Printf("实例列表 (%d):\n", len(instances))
			for _, inst := range instances {
				fmt.Printf("  [%s] %s (master:%d, cluster:%d)\n",
					inst.Status, inst.Name, inst.MasterPort, inst.ClusterPort)
			}
		case "create":
			name := instanceCmd.Arg(1)
			port, _ := instanceCmd.Get("master_port").Value.(interface{})
			if name == "" {
				log.Fatal("请指定实例名称")
			}
			_, err := instMgr.Create(name, 10999, 11000, 10, "")
			if err != nil {
				log.Fatalf("创建失败: %v", err)
			}
			fmt.Printf("✓ 创建实例: %s\n", name)
		case "start", "stop", "restart":
			id := instanceCmd.Arg(1)
			if id == "" {
				log.Fatal("请指定实例 ID")
			}
			inst, err := instMgr.Get(parseInt(id))
			if err != nil {
				log.Fatalf("获取实例失败: %v", err)
			}
			switch instanceCmd.Arg(0) {
			case "start":
				_ = instMgr.Start(inst)
			case "stop":
				_ = instMgr.Stop(inst)
			case "restart":
				_ = instMgr.Restart(inst)
			}
		default:
			log.Println("用法: dts-panel -cmd instance <list|create NAME|start ID|stop ID|restart ID>")
		}

	case "mod":
		modCmd.Parse(os.Args[2:])
		switch modCmd.Arg(0) {
		case "list":
			mods, err := modMgr.List()
			if err != nil {
				log.Fatalf("获取模组失败: %v", err)
			}
			fmt.Printf("模组列表 (%d):\n", len(mods))
			for _, m := range mods {
				enabled := "✓"
				if !m.Enabled {
					enabled = "✗"
				}
				fmt.Printf("  [%s] %s (%s)\n", enabled, m.ModName, m.ModID)
			}
		default:
			log.Println("用法: dts-panel -cmd mod list")
		}

	default:
		printUsage()
	}
}

func printUsage() {
	fmt.Println(`
DTS Panel - Don't Starve Together Server Manager

用法: dts-panel -cmd <子命令> <动作>

子命令:
  install   游戏环境/游戏安装
    deps      检查并安装系统依赖
    steamcmd  安装 SteamCMD
    game      安装 DST Dedicated Server
    update    更新游戏
    verify    验证安装

  instance  实例管理
    list            列出实例
    create <NAME>   创建实例
    start <ID>      启动实例
    stop <ID>       停止实例
    restart <ID>    重启实例

  mod       模组管理
    list  列出模组

Web 面板:
  直接运行: dts-panel
`)
}

func parseInt(s string) int64 {
	var n int64
	_, _ = fmt.Sscanf(s, "%d", &n)
	return n
}
