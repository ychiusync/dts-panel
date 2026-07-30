package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/dts-panel/dts-panel/internal/api"
	"github.com/dts-panel/dts-panel/internal/config"
	"github.com/dts-panel/dts-panel/internal/db"
)

func main() {
	cfg := config.Load()

	log.Printf("=== DTS Panel 启动 ===")
	log.Printf("数据目录: %s | 游戏目录: %s | 监听: %s:%d",
		cfg.DataDir, cfg.GameInstallDir, cfg.PanelHost, cfg.PanelPort)

	if err := db.Init(cfg.DataDir); err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}
	if err := db.AutoMigrate(); err != nil {
		log.Fatalf("数据库迁移失败: %v", err)
	}

	// 确保模板和静态资源复制到部署目录
	baseDir := filepath.Dir(os.Args[0])
	for src, dst := range map[string]string{
		"static":    filepath.Join(baseDir, "static"),
		"templates": filepath.Join(baseDir, "templates"),
	} {
		if _, err := os.Stat(src); err == nil {
			_ = exec.Command("cp", "-rf", src, dst).Run()
			log.Printf("同步资源: %s -> %s", src, dst)
		}
	}

	srv := api.NewServer(cfg)
	r := srv.RegisterRoutes()

	addr := fmt.Sprintf("%s:%d", cfg.PanelHost, cfg.PanelPort)
	log.Printf("运行中: http://%s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}
