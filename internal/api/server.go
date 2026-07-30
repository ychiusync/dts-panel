package api

import (
"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"time"
	"text/template"

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

// 全局安装状态
var installLog = &install.InstallLog{}
var installInProgress = false



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
	r.POST("/install/box", s.handleInstallBox)

	api := r.Group("/api")
	{
		api.GET("/system/status", s.apiSystemStatus)
		api.GET("/instances", s.apiListInstances)
		api.GET("/instances/:id/logs", s.apiInstanceLogs)
		api.GET("/mods", s.apiListMods)
		api.GET("/install/stream", s.apiInstallStream)
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
	if installInProgress {
		c.Redirect(http.StatusFound, "/install?msg=安装正在进行中，请稍候...")
		return
	}
	installLog = &install.InstallLog{}
	installInProgress = true
	go func() {
		defer func() { installInProgress = false }()
		err := s.installer.InstallDSTServer()
		if err != nil {
			installLog.Write("[install] 安装失败: " + err.Error())
		}
	}()
	c.Redirect(http.StatusFound, "/install")
}

// handleInstallBox 通过面板安装 Box86 + Box64（仅 ARM64 需要）
func (s *Server) handleInstallBox(c *gin.Context) {
	if runtime.GOARCH != "arm64" {
		installLog = &install.InstallLog{}
	c.Redirect(http.StatusFound, "/install?msg=✗ Box86/Box64 仅 ARM64 架构需要")
		return
	}
	if err := install.InstallBox86And64(); err != nil {
		c.Redirect(http.StatusFound, "/install?msg=✗ "+err.Error())
		return
	}
	c.Redirect(http.StatusFound, "/install?msg=✓ Box86 + Box64 安装完成")
}

// ===== API Handlers =====

func (s *Server) apiSystemStatus(c *gin.Context) {
	info := s.collectSystemInfo()
	Ok(c, "ok", map[string]interface{}{
		"os": info.OS, "arch": info.Arch, "cpu": info.CPU,
		"cpu_count": info.CPUCount, "is_arm64": info.Arch == "arm64",
		"steamcmd":  fileExists(s.cfg.SteamCMDPath),
		"game":      fileExists(filepath.Join(s.cfg.GameInstallDir, "bin", "linux64", "dedicated_server")),
		"box86":     cmdExists("box86"),
		"box64":     cmdExists("box64"),
		"data_dir":  s.cfg.DataDir, "game_dir": s.cfg.GameInstallDir,
		"in_progress": installInProgress,
		"logs":        installLog.Lines(),
	})
}

// apiInstallStream SSE 实时日志流
func (s *Server) apiInstallStream(c *gin.Context) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Flush()

	errChan := make(chan error, 1)

	// 后台执行安装
	go func() {
		installLog = &install.InstallLog{}
		err := s.installer.InstallDSTServer()
		installLog.Write(fmt.Sprintf("[install] 结果: %v", err))
		errChan <- err
		c.Writer.Flush()
	}()

	// SSE 推送
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	lastCount := 0

	for {
		select {
		case err := <-errChan:
			lastLines := installLog.Lines()
			for i := lastCount; i < len(lastLines); i++ {
				c.Writer.WriteString(fmt.Sprintf("data: %s\n\n", lastLines[i]))
				c.Writer.Flush()
			}
			lastCount = len(lastLines)
			// 发送结束标记
			status := "ok"
			if err != nil {
				status = "error"
			}
			c.Writer.WriteString(fmt.Sprintf("event: done\ndata: %s\n\n", status))
			c.Writer.Flush()
			return
		case <-ticker.C:
			lines := installLog.Lines()
			for i := lastCount; i < len(lines); i++ {
				c.Writer.WriteString(fmt.Sprintf("data: %s\n\n", lines[i]))
				c.Writer.Flush()
			}
			lastCount = len(lines)
		}
	}
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

func cmdExists(name string) bool { _, err := exec.LookPath(name); return err == nil }
func mustInt(s string) int { v, _ := strconv.Atoi(s); return v }
func mustIntOr(s string, defaultVal int) int { v, _ := strconv.Atoi(s); if v == 0 { return defaultVal }; return v }
