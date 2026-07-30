package api

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"text/template"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/dts-panel/dts-panel/internal/config"
	"github.com/dts-panel/dts-panel/internal/db"
	"github.com/dts-panel/dts-panel/internal/install"
	"github.com/dts-panel/dts-panel/internal/instance"
	"github.com/dts-panel/dts-panel/internal/mod"
	"github.com/dts-panel/dts-panel/internal/room"
)

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

func JSON(c *gin.Context, code int, msg string, data interface{}) {
	c.JSON(http.StatusOK, Response{Code: code, Message: msg, Data: data})
}
func Ok(c *gin.Context, msg string, data interface{}) { JSON(c, 200, msg, data) }

type Server struct {
	cfg         *config.Config
	installer   *install.SteamCMDInstaller
	systemCheck *install.SystemDepChecker
	instMgr     *instance.Manager
	roomMgr     *room.Manager
	modMgr      *mod.Manager
	assetsDir   string
	templateDir string
}

func NewServer(cfg *config.Config) *Server {
	return &Server{
		cfg:         cfg,
		installer:   install.NewSteamCMDInstaller(cfg.DataDir, cfg.SteamCMDPath, cfg.GameInstallDir),
		systemCheck: install.NewSystemDepChecker(),
		instMgr:     instance.NewManager(db.DB, cfg.InstanceRoot, cfg.GameInstallDir),
		roomMgr:     room.NewManager(db.DB),
		modMgr:      mod.NewManager(db.DB, filepath.Join(cfg.DataDir, "mods"), cfg.InstanceRoot),
		assetsDir:   filepath.Join(filepath.Dir(os.Args[0]), "static"),
		templateDir: filepath.Join(filepath.Dir(os.Args[0]), "templates"),
	}
}

func (s *Server) RegisterRoutes() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	if _, err := os.Stat(s.assetsDir + "/css"); err == nil {
		r.Static("/static", s.assetsDir)
	}

	r.GET("/", s.handleDashboard)
	r.GET("/dashboard", s.handleDashboard)
	r.GET("/system", s.handleSystem)
	r.GET("/install", s.handleInstall)
	r.GET("/instances", s.handleInstances)
	r.GET("/rooms", s.handleRooms)
	r.GET("/mods", s.handleMods)

	r.POST("/instances/create", s.handleCreateInstance)
	r.POST("/instances/action", s.handleInstanceAction)
	r.POST("/rooms/action", s.handleRoomAction)
	r.POST("/mods/action", s.handleModAction)
	r.POST("/install/steamcmd", s.handleInstallSteamCMD)
	r.POST("/install/game", s.handleInstallGame)

	api := r.Group("/api")
	{
		api.GET("/system/status", s.apiSystemStatus)
		api.GET("/instances", s.apiListInstances)
		api.GET("/instances/:id/logs", s.apiInstanceLogs)
		api.GET("/mods", s.apiListMods)
	}
	return r
}

func (s *Server) render(c *gin.Context, tmplName string, data map[string]interface{}) {
	if data == nil {
		data = make(map[string]interface{})
	}
	data["PageID"] = tmplName

	tmplLock.RLock()
	tmpl := globalTmpl
	tmplLock.RUnlock()

	if tmpl != nil {
		if err := tmpl.ExecuteTemplate(c.Writer, tmplName+".html", data); err == nil {
			return
		}
	}

	// fallback: read from file
	tmplFile := filepath.Join(s.templateDir, tmplName+".html")
	b, err := os.ReadFile(tmplFile)
	if err != nil {
		c.String(http.StatusInternalServerError, "模板加载失败: "+err.Error())
		return
	}

	funcMap := template.FuncMap{
		"pageActive": pageActive, "statusClass": statusClass,
		"lenVal": lenVal, "runningCount": countRunning,
		"stoppedCount": countStopped, "formatTime": formatTime,
		"formatInt": formatInt,
	}
	t := template.Must(template.New(tmplName+".html").Funcs(funcMap).Parse(string(b)))
	_ = t.Execute(c.Writer, data)
}

// ===== Page Handlers =====

func (s *Server) handleDashboard(c *gin.Context) {
	insts, _ := s.instMgr.List()
	running := 0
	for _, i := range insts {
		if i.Status == "running" {
			running++
		}
	}
	s.render(c, "dashboard", map[string]interface{}{
		"Title": "DTS Panel", "Instances": insts,
		"Running": running, "Stopped": len(insts) - running,
		"FlashMsg": c.Query("msg"),
	})
}

func (s *Server) handleSystem(c *gin.Context) {
	s.render(c, "system", map[string]interface{}{"Title": "系统环境"})
}

func (s *Server) handleInstall(c *gin.Context) {
	s.render(c, "install", map[string]interface{}{
		"Title":        "游戏安装",
		"SteamCMDOK":   fileExists(s.cfg.SteamCMDPath),
		"GameOK":       fileExists(filepath.Join(s.cfg.GameInstallDir, "bin", "linux64", "dedicated_server")),
		"GameDir":      s.cfg.GameInstallDir,
		"SteamCMDPath": s.cfg.SteamCMDPath,
	})
}

func (s *Server) handleInstances(c *gin.Context) {
	insts, _ := s.instMgr.List()
	s.render(c, "instances", map[string]interface{}{
		"Title": "实例管理", "Instances": insts, "FlashMsg": c.Query("msg"),
	})
}

func (s *Server) handleCreateInstance(c *gin.Context) {
	name := c.PostForm("name")
	if name == "" {
		s.render(c, "instances", map[string]interface{}{"Title": "实例管理", "FlashMsg": "✗ 实例名称不能为空"})
		return
	}
	masterPort := mustIntOr(c.PostForm("master_port"), s.cfg.DefaultMasterPort)
	clusterPort := mustIntOr(c.PostForm("cluster_port"), s.cfg.DefaultClusterPort)
	maxPlayers := mustIntOr(c.PostForm("max_players"), s.cfg.DefaultMaxPlayers)

	_, err := s.instMgr.Create(name, masterPort, clusterPort, maxPlayers, c.PostForm("server_token"))
	if err != nil {
		s.render(c, "instances", map[string]interface{}{"Title": "实例管理", "FlashMsg": "✗ " + err.Error()})
		return
	}
	c.Redirect(http.StatusFound, "/instances?msg=✓ 实例 "+name+" 创建成功")
}

func (s *Server) handleInstanceAction(c *gin.Context) {
	action := c.PostForm("action")
	id, _ := strconv.ParseInt(c.PostForm("id"), 10, 64)

	inst, err := s.instMgr.Get(id)
	if err != nil {
		c.Redirect(http.StatusFound, "/instances?msg=✗ 实例不存在")
		return
	}

	var actionErr error
	switch action {
	case "start":
		actionErr = s.instMgr.Start(inst)
	case "stop":
		actionErr = s.instMgr.Stop(inst)
	case "restart":
		actionErr = s.instMgr.Restart(inst)
	case "delete":
		actionErr = s.instMgr.Delete(id)
	default:
		c.Redirect(http.StatusFound, "/instances?msg=✗ 未知操作")
		return
	}

	if actionErr != nil {
		c.Redirect(http.StatusFound, "/instances?msg=✗ "+actionErr.Error())
		return
	}
	labels := map[string]string{"start": "启动", "stop": "停止", "restart": "重启", "delete": "删除"}
	c.Redirect(http.StatusFound, "/instances?msg=✓ 实例 "+inst.Name+" 已"+labels[action])
}

func (s *Server) handleRooms(c *gin.Context) {
	rooms, _ := s.roomMgr.List()
	s.render(c, "rooms", map[string]interface{}{
		"Title": "房间管理", "Rooms": rooms, "FlashMsg": c.Query("msg"),
	})
}

func (s *Server) handleRoomAction(c *gin.Context) {
	action := c.PostForm("action")
	switch action {
	case "create":
		r := &db.Room{
			InstanceID: int64(mustInt(c.PostForm("instance_id"))),
			Name:       c.PostForm("name"),
			WorldName:  c.PostForm("world_name"),
			MaxPlayers: mustInt(c.PostForm("max_players")),
			Seed:       c.PostForm("seed"),
			Autopause:  c.PostForm("autopause") == "on",
			AllowTransfer: c.PostForm("allow_transfer") == "on",
		}
		if err := s.roomMgr.Create(r); err != nil {
			c.Redirect(http.StatusFound, "/rooms?msg=✗ "+err.Error())
			return
		}
		c.Redirect(http.StatusFound, "/rooms?msg=✓ 房间 "+r.Name+" 创建成功")
	case "delete":
		id, _ := strconv.ParseInt(c.PostForm("id"), 10, 64)
		if err := s.roomMgr.Delete(id); err != nil {
			c.Redirect(http.StatusFound, "/rooms?msg=✗ "+err.Error())
			return
		}
		c.Redirect(http.StatusFound, "/rooms?msg=✓ 房间已删除")
	}
}

func (s *Server) handleMods(c *gin.Context) {
	mods, _ := s.modMgr.List()
	s.render(c, "mods", map[string]interface{}{
		"Title": "模组管理", "Mods": mods, "FlashMsg": c.Query("msg"),
	})
}

func (s *Server) handleModAction(c *gin.Context) {
	action := c.PostForm("action")
	modID := c.PostForm("mod_id")

	switch action {
	case "add":
		if _, err := s.modMgr.Add(modID, c.PostForm("mod_name"), c.PostForm("mod_url")); err != nil {
			c.Redirect(http.StatusFound, "/mods?msg=✗ "+err.Error())
			return
		}
		c.Redirect(http.StatusFound, "/mods?msg=✓ 模组添加成功")
	case "enable":
		s.modMgr.Enable(modID, true)
		c.Redirect(http.StatusFound, "/mods?msg=✓ 模组已启用")
	case "disable":
		s.modMgr.Enable(modID, false)
		c.Redirect(http.StatusFound, "/mods?msg=✓ 模组已禁用")
	case "delete":
		if err := s.modMgr.Delete(modID); err != nil {
			c.Redirect(http.StatusFound, "/mods?msg=✗ "+err.Error())
			return
		}
		c.Redirect(http.StatusFound, "/mods?msg=✓ 模组已删除")
	case "link":
		instID, _ := strconv.ParseInt(c.PostForm("instance_id"), 10, 64)
		if err := s.modMgr.LinkToInstance(instID, modID, true); err != nil {
			c.Redirect(http.StatusFound, "/mods?msg=✗ "+err.Error())
			return
		}
		c.Redirect(http.StatusFound, "/mods?msg=✓ 模组已绑定")
	}
}

func (s *Server) handleInstallSteamCMD(c *gin.Context) {
	if err := s.installer.InstallSteamCMD(); err != nil {
		c.Redirect(http.StatusFound, "/install?msg=✗ "+err.Error())
		return
	}
	c.Redirect(http.StatusFound, "/install?msg=✓ SteamCMD 安装完成")
}

func (s *Server) handleInstallGame(c *gin.Context) {
	if err := s.installer.InstallDSTServer(); err != nil {
		c.Redirect(http.StatusFound, "/install?msg=✗ "+err.Error())
		return
	}
	c.Redirect(http.StatusFound, "/install?msg=✓ DST Dedicated Server 安装完成")
}

// ===== API Handlers =====

func (s *Server) apiSystemStatus(c *gin.Context) {
	info := s.collectSystemInfo()
	Ok(c, "ok", map[string]interface{}{
		"os": info.OS, "arch": info.Arch, "cpu": info.CPU,
		"cpu_count": info.CPUCount, "is_arm64": info.Arch == "arm64",
		"steamcmd": fileExists(s.cfg.SteamCMDPath),
		"game":     fileExists(filepath.Join(s.cfg.GameInstallDir, "bin", "linux64", "dedicated_server")),
		"data_dir": s.cfg.DataDir, "game_dir": s.cfg.GameInstallDir,
	})
}

func (s *Server) apiListInstances(c *gin.Context) {
	insts, err := s.instMgr.List()
	if err != nil {
		JSON(c, 400, err.Error(), nil)
		return
	}
	Ok(c, "ok", insts)
}

func (s *Server) apiInstanceLogs(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	inst, err := s.instMgr.Get(id)
	if err != nil {
		JSON(c, 400, "实例不存在", nil)
		return
	}
	logs, err := s.instMgr.RefreshLogs(inst, 100)
	if err != nil {
		JSON(c, 500, err.Error(), nil)
		return
	}
	Ok(c, "ok", map[string]string{"logs": logs})
}

func (s *Server) apiListMods(c *gin.Context) {
	mods, err := s.modMgr.List()
	if err != nil {
		JSON(c, 500, err.Error(), nil)
		return
	}
	Ok(c, "ok", mods)
}

// ===== Helpers =====

type sysInfo struct{ OS, Arch, CPU string; CPUCount int }

func (s *Server) collectSystemInfo() sysInfo {
	cpuInfo := "Unknown"
	if runtime.GOOS == "linux" {
		cmd := exec.Command("bash", "-c", "lscpu 2>/dev/null | grep 'Model name' | head -1 | cut -d: -f2 | xargs")
		out, _ := cmd.Output()
		if len(out) > 0 {
			cpuInfo = string(out)
		}
	}
	return sysInfo{runtime.GOOS, runtime.GOARCH, cpuInfo, runtime.NumCPU()}
}

func fileExists(p string) bool { _, err := os.Stat(p); return err == nil }
func mustInt(s string) int { v, _ := strconv.Atoi(s); return v }
func mustIntOr(s string, defaultVal int) int { v, _ := strconv.Atoi(s); if v == 0 { return defaultVal }; return v }
