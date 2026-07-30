package api

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dts-panel/dts-panel/internal/config"
	"github.com/dts-panel/dts-panel/internal/db"
	"github.com/dts-panel/dts-panel/internal/install"
	"github.com/dts-panel/dts-panel/internal/instance"
	dtsmod "github.com/dts-panel/dts-panel/internal/mod"
	"github.com/dts-panel/dts-panel/internal/room"
)

// Server Web 面板服务
type Server struct {
	cfg         *config.Config
	db          *sql.DB
	instMgr     *instance.Manager
	roomMgr     *room.Manager
	modMgr      *dtsmod.Manager

	installer     *install.SteamCMDInstaller
	systemChecker *install.SystemDepChecker
}

func NewServer(cfg *config.Config, database *sql.DB) *Server {
	_ = db.Migrate(database)

	s := &Server{
		cfg:     cfg,
		db:      database,
		instMgr: instance.NewManager(database, cfg.InstanceRoot, cfg.GameInstallDir),
		roomMgr: room.NewManager(database),
		modMgr:  dtsmod.NewManager(database, filepath.Join(cfg.DataDir, "mods"), cfg.InstanceRoot),

		installer:     install.NewSteamCMDInstaller(cfg.DataDir, cfg.SteamCMDPath, cfg.GameInstallDir),
		systemChecker: install.NewSystemDepChecker(),
	}

	return s
}

func (s *Server) RegisterRoutes() http.Handler {
	mux := http.NewServeMux()

	// 静态资源
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// 页面路由
	mux.HandleFunc("/dashboard", s.handler(s.handleDashboard))
	mux.HandleFunc("/system", s.handler(s.handleSystem))
	mux.HandleFunc("/system/check-deps", s.handler(s.handleCheckDeps))
	mux.HandleFunc("/install", s.handler(s.handleInstall))
	mux.HandleFunc("/install/steamcmd", s.handler(s.handleInstallSteamCMD))
	mux.HandleFunc("/install/game", s.handler(s.handleInstallGame))
	mux.HandleFunc("/instances", s.handler(s.handleInstances))
	mux.HandleFunc("/rooms", s.handler(s.handleRooms))
	mux.HandleFunc("/mods", s.handler(s.handleMods))

	// 实例 CRUD
	mux.HandleFunc("/instances/action", s.handler(s.handleInstanceAction))
	mux.HandleFunc("/instances/create", s.handler(s.handleCreateInstance))
	mux.HandleFunc("/instances/{id}/logs", s.handler(s.handleInstanceLogs))

	// 房间 CRUD
	mux.HandleFunc("/rooms/action", s.handler(s.handleRoomAction))

	// 模组操作
	mux.HandleFunc("/mods/action", s.handler(s.handleModAction))

	// JSON API
	mux.HandleFunc("/api/instances", s.apiListInstances)
	mux.HandleFunc("/api/system/status", s.apiSystemStatus)
	mux.HandleFunc("/api/mods", s.apiListMods)

	log.Printf("[api] 路由注册完成")
	return mux
}

// 中间件
func (s *Server) handler(fn func(http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("[api] 恐慌恢复: %v", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		fn(w, r)
	}
}

func jsonResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func pathParam(r *http.Request, key string) string {
	if key == "id" {
		for _, part := range strings.Split(r.URL.Path, "/") {
			if _, err := strconv.Atoi(part); err == nil {
				return part
			}
		}
	}
	return ""
}
