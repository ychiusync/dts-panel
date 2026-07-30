package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/dts-panel/dts-panel/internal/api"
	"github.com/dts-panel/dts-panel/internal/config"
	"github.com/dts-panel/dts-panel/internal/db"
)

func main() {
	cfg := config.Load()

	log.Printf("=== DTS Panel 启动 ===")
	log.Printf("数据目录: %s", cfg.DataDir)
	log.Printf("游戏目录: %s", cfg.GameInstallDir)
	log.Printf("实例目录: %s", cfg.InstanceRoot)
	log.Printf("监听地址: %s:%d", cfg.PanelHost, cfg.PanelPort)

	database, err := db.Init(cfg.DataDir)
	if err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}
	defer database.Close()

	srv := api.NewServer(cfg, database)
	handler := srv.RegisterRoutes()

	addr := fmt.Sprintf("%s:%d", cfg.PanelHost, cfg.PanelPort)

	log.Printf("DTS Panel 正在运行: http://%s", addr)
	log.Fatal(http.ListenAndServe(addr, handler))
}

func init() {
	// 确保 templates 和 static 目录存在（二进制同目录）
	for _, d := range []string{"templates", "static"} {
		if _, err := os.Stat(d); os.IsNotExist(err) {
			log.Printf("[warn] %s/ 目录不存在，将使用默认渲染", d)
		}
	}
}
