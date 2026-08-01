package api

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
	"text/template"

	"github.com/gin-gonic/gin"
	"github.com/dts-panel/dts-panel/internal/config"
	"github.com/dts-panel/dts-panel/internal/db"
	"github.com/dts-panel/dts-panel/internal/install"
	"github.com/dts-panel/dts-panel/internal/cluster"
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
	clusterMgr  *cluster.Manager
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
		clusterMgr:  cluster.NewManager(db.DB, cfg.ClusterRoot, cfg.GameInstallDir, cfg.InstanceRoot),
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
	r.GET("/clusters", s.handleClusters)

	r.POST("/instances/create", s.handleCreateInstance)
	r.POST("/instances/action", s.handleInstanceAction)
	r.POST("/rooms/action", s.handleRoomAction)
	r.POST("/mods/action", s.handleModAction)
	r.POST("/clusters/create", s.handleCreateCluster)
	r.POST("/clusters/action", s.handleClusterAction)
	r.POST("/clusters/worlds", s.handleWorlds)
	r.POST("/install/steamcmd", s.handleInstallSteamCMD)
	r.POST("/install/game", s.handleInstallGame)
	r.POST("/install/box", s.handleInstallBox)
	r.POST("/install/cdn", s.handleInstallCDN)
	r.DELETE("/install/cdn", s.handleRemoveCDN)

	api := r.Group("/api")
	{
		api.GET("/system/status", s.apiSystemStatus)
		api.GET("/instances", s.apiListInstances)
		api.GET("/instances/:id/logs", s.apiInstanceLogs)
		api.GET("/mods", s.apiListMods)
		api.GET("/install/stream", s.apiInstallStream)
		api.GET("/system/settings", s.apiSystemSettings)
		api.POST("/system/settings", s.apiSaveSystemSettings)
		api.GET("/logs/json", s.apiLogsJSON)
	api.GET("/logs/sse", s.apiLogsSSE)
		api.GET("/logs/clusters", s.apiListClusterNames)
		api.GET("/clusters/:id/worlds/:world_id/params", s.apiGetWorldParams)
		api.PUT("/clusters/:id/worlds/:world_id/params", s.apiSaveWorldParams)
		api.GET("/clusters/:id/connect", s.apiGetClusterConnectURL)
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


// ===== Cluster (房间) Handlers =====

func (s *Server) handleClusters(c *gin.Context) {
	action := c.Query("action")
	idStr := c.Query("id")

	if action == "detail" && idStr != "" {
		id, _ := strconv.ParseInt(idStr, 10, 64)
		if id == 0 {
			s.render(c, "clusters", map[string]interface{}{
				"Title": "房间管理", "FlashMsg": "✗ 无效的集群 ID",
			})
			return
		}
		cluster, worlds, err := s.clusterMgr.Get(id)
		if err != nil {
			s.render(c, "clusters", map[string]interface{}{
				"Title": "房间管理", "FlashMsg": "✗ " + err.Error(),
			})
			return
		}
		clusterPath := s.clusterMgr.GetClusterPath(id)
		masterIP := cluster.MasterIP
		masterPort := 10999
		for _, w := range worlds {
			if w.IsMaster {
				masterPort = w.ServerPort
				break
			}
		}
		adminList, _ := s.clusterMgr.ReadAdminList(id)
		s.render(c, "clusters", map[string]interface{}{
			"Title":       "房间详情 - " + cluster.Name,
			"ShowDetail":  true,
			"Cluster":     cluster,
			"Worlds":      worlds,
			"ClusterPath": clusterPath,
			"MasterIP": masterIP,
			"MasterPort": masterPort,
			"AdminList":   adminList,
			"FlashMsg":    c.Query("msg"),
		})
		return
	}
	clusters, _ := s.clusterMgr.List()
	s.render(c, "clusters", map[string]interface{}{
		"Title": "房间管理", "Clusters": clusters, "FlashMsg": c.Query("msg"),
		"DefaultClusterToken": s.cfg.DefaultClusterToken,
		"DefaultAdminUids":    s.cfg.DefaultAdminUids,
	})
}

func (s *Server) handleCreateCluster(c *gin.Context) {
	name := c.PostForm("name")
	if name == "" {
		s.render(c, "clusters", map[string]interface{}{
			"Title": "房间管理", "FlashMsg": "✗ 房间名称不能为空",
		})
		return
	}
	maxPlayers := mustIntOr(c.PostForm("max_players"), 10)
	password := c.PostForm("password")
	gameMode := c.PostForm("game_mode")
	if gameMode == "" {
		gameMode = "endless"
	}
	isOffline := c.PostForm("is_offline") == "on"
	masterIP := c.PostForm("master_ip")
	if masterIP == "" {
		masterIP = "127.0.0.1"
	}

	// 优先用表单提交的值，否则回退全局默认值
	clusterToken := c.PostForm("cluster_token")
	if clusterToken == "" {
		clusterToken = s.cfg.DefaultClusterToken
	}
	adminUids := c.PostForm("admin_uids")
	if adminUids == "" {
		adminUids = s.cfg.DefaultAdminUids
	}

	cluster, _, err := s.clusterMgr.Create(name, maxPlayers, password, gameMode, isOffline, masterIP, clusterToken, adminUids)
	if err != nil {
		s.render(c, "clusters", map[string]interface{}{
			"Title": "房间管理", "FlashMsg": "✗ " + err.Error(),
		})
		return
	}
	c.Redirect(http.StatusFound, fmt.Sprintf("/clusters?action=detail&id=%d&msg=✓ 房间 %s 创建成功（地面+洞穴）", cluster.ID, name))
}

func (s *Server) handleClusterAction(c *gin.Context) {
	action := c.PostForm("action")
	id, _ := strconv.ParseInt(c.PostForm("id"), 10, 64)

	switch action {
	case "detail":
		c.Redirect(http.StatusFound, fmt.Sprintf("/clusters?action=detail&id=%d", id))
	case "start":
		cCluster, _, _ := s.clusterMgr.Get(id)
		if cCluster != nil && cCluster.Status == "running" {
			c.Redirect(http.StatusFound, "/clusters?msg=✗ 集群已在运行中")
			return
		}
		if err := s.clusterMgr.Start(id); err != nil {
			c.Redirect(http.StatusFound, "/clusters?msg=✗ "+err.Error())
			return
		}
		c.Redirect(http.StatusFound, "/clusters?msg=✓ 集群已启动")
	case "stop":
		cCluster, _, _ := s.clusterMgr.Get(id)
		if cCluster != nil && cCluster.Status == "stopped" {
			c.Redirect(http.StatusFound, "/clusters?msg=✗ 集群已在停止状态")
			return
		}
		if err := s.clusterMgr.Stop(id); err != nil {
			c.Redirect(http.StatusFound, "/clusters?msg=✗ "+err.Error())
			return
		}
		c.Redirect(http.StatusFound, "/clusters?msg=✓ 集群已停止")
	case "restart":
		if err := s.clusterMgr.Restart(id); err != nil {
			c.Redirect(http.StatusFound, "/clusters?msg=✗ "+err.Error())
			return
		}
		c.Redirect(http.StatusFound, "/clusters?msg=✓ 集群已重启")
	case "delete":
		if err := s.clusterMgr.Delete(id); err != nil {
			c.Redirect(http.StatusFound, "/clusters?msg=✗ "+err.Error())
			return
		}
		c.Redirect(http.StatusFound, "/clusters?msg=✓ 集群已删除")
	case "save_admins":
		adminUids := c.PostForm("admin_uids")
		var uids []string
		for _, u := range strings.Split(adminUids, "\n") {
			u = strings.TrimSpace(u)
			if u != "" {
				uids = append(uids, u)
			}
		}
		if err := s.clusterMgr.WriteAdminList(id, uids); err != nil {
			c.Redirect(http.StatusFound, fmt.Sprintf("/clusters?action=detail&id=%d&msg=✗ 保存管理员失败", id))
			return
		}
		c.Redirect(http.StatusFound, fmt.Sprintf("/clusters?action=detail&id=%d&msg=✓ 管理员列表已更新", id))
	case "console":
		worldID, _ := strconv.ParseInt(c.PostForm("world_id"), 10, 64)
		cmd := c.PostForm("cmd")
		if worldID > 0 && cmd != "" {
			if err := s.clusterMgr.ConsoleCmd(worldID, cmd); err != nil {
				c.Redirect(http.StatusFound, "/clusters?msg=✗ 命令执行失败")
				return
			}
		}
		c.Redirect(http.StatusFound, fmt.Sprintf("/clusters?action=detail&id=%d", id))
	case "save_level":
		worldID, _ := strconv.ParseInt(c.PostForm("world_id"), 10, 64)
		levelData := c.PostForm("level_data")
		if worldID > 0 && levelData != "" {
			if err := s.clusterMgr.UpdateWorldLevel(worldID, levelData); err != nil {
				c.Redirect(http.StatusFound, "/clusters?msg=✗ "+err.Error())
				return
			}
		}
		c.Redirect(http.StatusFound, fmt.Sprintf("/clusters?action=detail&id=%d", id))
	default:
		c.Redirect(http.StatusFound, "/clusters")
	}
}

func (s *Server) handleWorlds(c *gin.Context) {
	action := c.PostForm("action")
	if action == "save_level" {
		worldID, _ := strconv.ParseInt(c.PostForm("world_id"), 10, 64)
		levelData := c.PostForm("level_data")
		if err := s.clusterMgr.UpdateWorldLevel(worldID, levelData); err != nil {
			s.render(c, "clusters", map[string]interface{}{
				"Title": "房间管理", "FlashMsg": "✗ " + err.Error(),
			})
			return
		}
		c.Redirect(http.StatusFound, fmt.Sprintf("/clusters?action=detail&id=%s", c.PostForm("cluster_id")))
	}
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
		"Title": "DTS Panel",
		"Instances": insts,
		"Running": running, "Stopped": len(insts) - running,
		"GameOK":     s.gameInstalled(),
		"SteamCMDOK": fileExists(s.cfg.SteamCMDPath),
		"Box64OK":    cmdExists("box64"),
		"CDNOK":      install.CheckSteamCDNHosts(),
		"FlashMsg":   c.Query("msg"),
	})
}

func (s *Server) handleSystem(c *gin.Context) {
	s.render(c, "system", map[string]interface{}{"Title": "系统环境"})
}

func (s *Server) handleInstall(c *gin.Context) {
	s.render(c, "install", map[string]interface{}{
		"Title":        "游戏安装",
		"SteamCMDOK":   fileExists(s.cfg.SteamCMDPath),
		"GameOK":       s.gameInstalled(),
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

// handleInstallCDN 添加 Steam CDN hosts 映射
func (s *Server) handleInstallCDN(c *gin.Context) {
	if err := install.AddSteamCDNHosts(); err != nil {
		c.Redirect(http.StatusFound, "/install?msg=✗ "+err.Error())
		return
	}
	c.Redirect(http.StatusFound, "/install?msg=✓ Steam CDN hosts 添加完成")
}

// handleRemoveCDN 移除 Steam CDN hosts 映射
func (s *Server) handleRemoveCDN(c *gin.Context) {
	if err := install.RemoveSteamCDNHosts(); err != nil {
		c.Redirect(http.StatusFound, "/install?msg=✗ "+err.Error())
		return
	}
	c.Redirect(http.StatusFound, "/install?msg=✓ Steam CDN hosts 已移除")
}

// ===== Log API Handlers =====

// getLogContent 统一获取日志内容
func (s *Server) getLogContent(source, clusterName, worldName string, lines int) string {
	switch source {
	case "panel":
		logPaths := []string{
			filepath.Join(s.cfg.DataDir, "dts-panel.log"),
			filepath.Join(filepath.Dir(os.Args[0]), "dts-panel.log"),
		}
		for _, p := range logPaths {
			if _, err := os.Stat(p); err == nil {
				out, _ := exec.Command("bash", "-c", "tail -n "+fmt.Sprintf("%d", lines)+" "+p).Output()
				return string(out)
			}
		}
	case "install":
		arr := installLog.Lines()
		data := strings.Join(arr, "\n")
		if len(data) > 0 {
			data += "\n"
		}
		stage := install.GetInstallStage()
		if stage != "idle" {
			data = "[stage:" + stage + "]\n" + data
		}
		return data
	case "cluster":
		if clusterName != "" && worldName != "" {
			safeName := cluster.SafeClusterName(clusterName)
			srvPath := filepath.Join(s.cfg.ClusterRoot, safeName, worldName, "server_log.txt")
			chatPath := filepath.Join(s.cfg.ClusterRoot, safeName, worldName, "server_chat_log.txt")
			srvLog, chatLog := "", ""
			if _, err := os.Stat(srvPath); err == nil {
				out, _ := exec.Command("bash", "-c", "tail -n "+fmt.Sprintf("%d", lines)+" "+srvPath).Output()
				srvLog = string(out)
			}
			if _, err := os.Stat(chatPath); err == nil {
				out, _ := exec.Command("bash", "-c", "tail -n "+fmt.Sprintf("%d", lines/2)+" "+chatPath).Output()
				chatLog = string(out)
			}
			data := "== SERVER ==\n" + srvLog
			if chatLog != "" {
				data += "\n\n== CHAT ==\n" + chatLog
			}
			return data
		}
	}
	return ""
}

func (s *Server) apiLogsSSE(c *gin.Context) {
	source := c.Query("source")
	if source == "" {
		source = "panel"
	}
	clusterName := c.Query("cluster_name")
	worldName := c.Query("world")
	lines := 200
	if n := c.Query("lines"); n != "" {
		if v, _ := strconv.Atoi(n); v > 0 && v <= 500 {
			lines = v
		}
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.WriteHeader(200)
	c.Writer.Flush()

	c.Writer.WriteString("event: ping\ndata: connected\n\n")
	c.Writer.Flush()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			c.Writer.WriteString("event: done\ndata: timeout\n\n")
			c.Writer.Flush()
			return
		case <-ticker.C:
			var data string
			switch source {
			case "panel":
				logPaths := []string{
					filepath.Join(s.cfg.DataDir, "dts-panel.log"),
					filepath.Join(filepath.Dir(os.Args[0]), "dts-panel.log"),
				}
				for _, p := range logPaths {
					if _, err := os.Stat(p); err == nil {
						out, _ := exec.Command("bash", "-c", "tail -n "+fmt.Sprintf("%d", lines)+" "+p).Output()
						data = string(out)
						break
					}
				}
			case "install":
				arr := installLog.Lines()
				data = strings.Join(arr, "\n")
				if len(data) > 0 {
					data += "\n"
				}
				stage := install.GetInstallStage()
				if stage != "idle" {
					data = "[stage:" + stage + "]\n" + data
				}
			case "cluster":
				if clusterName != "" && worldName != "" {
					safeName := cluster.SafeClusterName(clusterName)
					srvPath := filepath.Join(s.cfg.ClusterRoot, safeName, worldName, "server_log.txt")
					chatPath := filepath.Join(s.cfg.ClusterRoot, safeName, worldName, "server_chat_log.txt")
					srvLog, chatLog := "", ""
					if _, err := os.Stat(srvPath); err == nil {
						out, _ := exec.Command("bash", "-c", "tail -n "+fmt.Sprintf("%d", lines)+" "+srvPath).Output()
						srvLog = string(out)
					}
					if _, err := os.Stat(chatPath); err == nil {
						out, _ := exec.Command("bash", "-c", "tail -n "+fmt.Sprintf("%d", lines/2)+" "+chatPath).Output()
						chatLog = string(out)
					}
					data = "== SERVER ==" + "\n" + srvLog
					if chatLog != "" {
						data += "\n\n== CHAT ==" + "\n" + chatLog
					}
				}
			}
			if data != "" {
				c.Writer.WriteString("event: log\ndata: " + data + "\n\n")
			}
			c.Writer.Flush()
		}
	}
}

func (s *Server) apiLogsJSON(c *gin.Context) {
	source := c.Query("source")
	if source == "" {
		source = "panel"
	}
	clusterName := c.Query("cluster_name")
	worldName := c.Query("world")
	lines := 200
	if n := c.Query("lines"); n != "" {
		if v, _ := strconv.Atoi(n); v > 0 && v <= 500 {
			lines = v
		}
	}
	data := s.getLogContent(source, clusterName, worldName, lines)
	Ok(c, "ok", gin.H{"data": data})
}

func (s *Server) apiListClusterNames(c *gin.Context) {
	clusters, _ := s.clusterMgr.List()
	names := []string{}
	if len(clusters) > 0 {
		for _, cl := range clusters {
			names = append(names, cl.Name)
		}
	}
	// 如果数据库为空，从文件系统扫描
	if len(names) == 0 {
		entries, err := os.ReadDir(s.cfg.ClusterRoot)
		if err == nil {
			for _, e := range entries {
				if e.IsDir() {
					names = append(names, e.Name())
				}
			}
		}
	}
	Ok(c, "ok", names)
}

// ===== API Handlers =====

func (s *Server) apiSystemStatus(c *gin.Context) {
	info := s.collectSystemInfo()
	Ok(c, "ok", map[string]interface{}{
		"os": info.OS, "arch": info.Arch, "cpu": info.CPU,
		"cpu_count": info.CPUCount, "is_arm64": info.Arch == "arm64",
		"steamcmd":  fileExists(s.cfg.SteamCMDPath),
		"game":      s.gameInstalled(),
		"box86":     cmdExists("box86"),
		"box64":     cmdExists("box64"),
		"data_dir":  s.cfg.DataDir, "game_dir": s.cfg.GameInstallDir,
		"in_progress": installInProgress,
		"logs":        installLog.Lines(),
		"cdn_hosts":   install.CheckSteamCDNHosts(),
	})
}

// apiSystemSettings 读取全局默认设置
func (s *Server) apiSystemSettings(c *gin.Context) {
	Ok(c, "ok", gin.H{
		"DefaultClusterToken": s.cfg.DefaultClusterToken,
		"DefaultAdminUids":    s.cfg.DefaultAdminUids,
	})
}

// apiSaveSystemSettings 保存全局默认设置
func (s *Server) apiSaveSystemSettings(c *gin.Context) {
	var req struct {
		DefaultClusterToken string `json:"DefaultClusterToken"`
		DefaultAdminUids    string `json:"DefaultAdminUids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		JSON(c, 400, "参数错误", nil)
		return
	}
	s.cfg.DefaultClusterToken = req.DefaultClusterToken
	s.cfg.DefaultAdminUids = req.DefaultAdminUids
	err := s.cfg.Save()
	if err != nil {
		JSON(c, 500, "保存失败: "+err.Error(), nil)
		return
	}
	Ok(c, "保存成功", nil)
}

// apiInstallStream SSE 实时日志流
// 只负责推送日志，不启动安装（安装由 handleInstallGame 触发）
func (s *Server) apiInstallStream(c *gin.Context) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.WriteHeader(200)
	c.Writer.Flush()

	// 发送初始心跳
	c.Writer.WriteString("event: heartbeat\ndata: connected\n\n")
	c.Writer.Flush()

	// 等待安装完成
	timeout := time.NewTimer(60 * time.Minute)
	defer timeout.Stop()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	lastCount := 0

	for {
		select {
		case <-timeout.C:
			// 超时结束
			c.Writer.WriteString("event: done\ndata: timeout|timeout\n\n")
			c.Writer.Flush()
			return

		case <-ticker.C:
			lines := installLog.Lines()
			for i := lastCount; i < len(lines); i++ {
				c.Writer.WriteString(fmt.Sprintf("data: %s\n\n", lines[i]))
				c.Writer.Flush()
			}
			lastCount = len(lines)

			// 检查安装是否完成（不再有进行标志 + 有日志内容）
			if !installInProgress && lastCount > 0 {
				// 再等一下确认
				time.Sleep(500 * time.Millisecond)
				if !installInProgress {
					c.Writer.WriteString("event: done\ndata: done|complete\n\n")
					c.Writer.Flush()
					return
				}
			}
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

// gameInstalled 检测 DST Dedicated Server 是否已安装（检查多个可能的路径）
func (s *Server) gameInstalled() bool {
	candidates := []string{
		filepath.Join(s.cfg.GameInstallDir, "bin64", "dontstarve_dedicated_server_nullrenderer_x64"),
		filepath.Join(s.cfg.GameInstallDir, "bin", "dontstarve_dedicated_server_nullrenderer"),
		filepath.Join(s.cfg.GameInstallDir, "bin", "linux64", "dedicated_server"),
		filepath.Join(s.cfg.GameInstallDir, "linux64", "steamclient.so"),
	}
	for _, p := range candidates {
		if fileExists(p) {
			return true
		}
	}
	return false
}

func cmdExists(name string) bool { _, err := exec.LookPath(name); return err == nil }
func mustInt(s string) int { v, _ := strconv.Atoi(s); return v }
func mustIntOr(s string, defaultVal int) int { v, _ := strconv.Atoi(s); if v == 0 { return defaultVal }; return v }


// ===== 世界参数 API =====

// apiGetWorldParams 返回结构化世界参数（分类+当前值）
func (s *Server) apiGetWorldParams(c *gin.Context) {
    worldID, _ := strconv.ParseInt(c.Param("world_id"), 10, 64)
    if worldID == 0 {
        JSON(c, 400, "无效的世界 ID", nil)
        return
    }

    world, err := s.clusterMgr.GetWorld(worldID)
    if err != nil {
        JSON(c, 400, err.Error(), nil)
        return
    }

    params := cluster.ParseLevelDataToParams(world.LevelData)
    categories := cluster.GetParamCategories(world.Name)

    result := make([]gin.H, 0, len(categories))
    for _, cat := range categories {
        items := make([]gin.H, 0, len(cat.Params))
        for _, p := range cat.Params {
            currentVal := params[p.Key]
            if currentVal == "" {
                currentVal = p.Default
            }
            items = append(items, gin.H{
                "key":      p.Key,
                "label":    p.Label,
                "type":     p.Type,
                "options":  p.Options,
                "value":    currentVal,
                "default":  p.Default,
            })
        }
        result = append(result, gin.H{
            "key":    cat.Key,
            "label":  cat.Label,
            "icon":   cat.Icon,
            "params": items,
        })
    }

    Ok(c, "ok", gin.H{
        "world_id":   world.ID,
        "world_name": world.Name,
        "categories": result,
    })
}

// apiSaveWorldParams 保存世界参数（从结构化数据重建 leveldataoverride.lua）
func (s *Server) apiSaveWorldParams(c *gin.Context) {
    var req struct {
        WorldID  int64             `json:"world_id"`
        Params   map[string]string `json:"params"`
    }
    if err := c.ShouldBindJSON(&req); err != nil {
        JSON(c, 400, "参数格式错误: "+err.Error(), nil)
        return
    }
    if req.WorldID == 0 {
        JSON(c, 400, "缺少 world_id", nil)
        return
    }

    world, err := s.clusterMgr.GetWorld(req.WorldID)
    if err != nil {
        JSON(c, 400, err.Error(), nil)
        return
    }

    levelData := cluster.BuildLevelDataFromParams(world.Name, req.Params)

    // 保存到数据库和磁盘
    if err := s.clusterMgr.UpdateWorldLevel(req.WorldID, levelData); err != nil {
        JSON(c, 500, "保存失败: "+err.Error(), nil)
        return
    }

    Ok(c, "世界参数已保存", nil)
}

// apiGetClusterConnectURL 获取房间连接信息（IP、端口、dst:// 链接）
func (s *Server) apiGetClusterConnectURL(c *gin.Context) {
    clusterID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
    if clusterID == 0 {
        JSON(c, 400, "无效的集群 ID", nil)
        return
    }

    cl, worlds, err := s.clusterMgr.Get(clusterID)
    if err != nil {
        JSON(c, 400, err.Error(), nil)
        return
    }

    // 获取服务器外部 IP
    serverIP := getPublicIP()
    // 也尝试读取 Master 的 server.ini 确认端口
    var masterWorld *cluster.World
    for i := range worlds {
        if worlds[i].IsMaster {
            masterWorld = &worlds[i]
        }
    }

    var serverPort int
    var dstURL string
    if masterWorld != nil {
        serverPort = masterWorld.ServerPort
    } else {
        serverPort = 10999
    }

    // dst:// 格式: dst://<IP>:<PORT>?cluster=<cluster_name>
    dstURL = fmt.Sprintf("dst://%s:%d?cluster=%s", serverIP, serverPort, cl.Name)

    // 同时构建 steam 短连接链接
    steamURL := fmt.Sprintf("steam://connect/%s:%d", serverIP, serverPort)

    token, _ := s.clusterMgr.GetClusterToken(clusterID)
    Ok(c, "ok", gin.H{
        "server_ip":       serverIP,
        "server_port":     serverPort,
        "dst_url":         dstURL,
        "steam_connect":   steamURL,
        "cluster_name":    cl.Name,
        "status":          cl.Status,
        "cluster_token":   token,
    })
}

// getPublicIP 获取服务器外部 IP
func getPublicIP() string {
    // 先尝试本地 eth 网卡 IP（局域网连接场景）
    addrs, err := net.InterfaceAddrs()
    if err == nil {
        for _, addr := range addrs {
            if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
                return ipnet.IP.String()
            }
        }
    }
    return "127.0.0.1"
}
