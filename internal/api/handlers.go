package api

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"

	"github.com/dts-panel/dts-panel/internal/room"
)

// handleDashboard 面板首页
func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	instances, _ := s.instMgr.List()
	running := 0
	for _, inst := range instances {
		if inst.Status == "running" {
			running++
		}
	}
	s.renderHTML(w, "dashboard", &PageData{
		Title:    "DTS Panel",
		PageID:   "dashboard",
		Instances: instances,
		FlashMsg: r.URL.Query().Get("msg"),
	})
}

// handleSystem 系统环境页面
func (s *Server) handleSystem(w http.ResponseWriter, r *http.Request) {
	s.renderHTML(w, "system", &PageData{
		Title:  "系统环境",
		PageID: "system",
	})
}

// handleInstall 游戏安装页面（直接渲染模板）
func (s *Server) handleInstall(w http.ResponseWriter, r *http.Request) {
	s.renderHTML(w, "install", &PageData{
		Title:  "游戏安装",
		PageID: "install",
	})
}

// handleInstances 实例列表
func (s *Server) handleInstances(w http.ResponseWriter, r *http.Request) {
	instances, _ := s.instMgr.List()
	s.renderHTML(w, "instances", &PageData{
		Title:     "实例管理",
		PageID:    "instances",
		Instances: instances,
	})
}

// handleCreateInstance 创建实例
func (s *Server) handleCreateInstance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.renderHTML(w, "instances", &PageData{Title: "实例管理", PageID: "instances"})
		return
	}

	name := r.FormValue("name")
	masterPort, _ := strconv.Atoi(r.FormValue("master_port"))
	clusterPort, _ := strconv.Atoi(r.FormValue("cluster_port"))
	maxPlayers, _ := strconv.Atoi(r.FormValue("max_players"))
	serverToken := r.FormValue("server_token")

	if name == "" {
		s.renderHTML(w, "instances", &PageData{
			Title:  "实例管理",
			PageID: "instances",
			FlashMsg: "✗ 实例名称不能为空",
		})
		return
	}
	if masterPort == 0 {
		masterPort = s.cfg.DefaultMasterPort
	}
	if clusterPort == 0 {
		clusterPort = s.cfg.DefaultClusterPort
	}
	if maxPlayers == 0 {
		maxPlayers = s.cfg.DefaultMaxPlayers
	}

	_, err := s.instMgr.Create(name, masterPort, clusterPort, maxPlayers, serverToken)
	if err != nil {
		s.renderHTML(w, "instances", &PageData{
			Title:  "实例管理",
			PageID: "instances",
			FlashMsg: "✗ " + err.Error(),
		})
		return
	}

	http.Redirect(w, r, "/instances?msg=✓ 实例 "+name+" 创建成功", http.StatusSeeOther)
}

// handleInstanceAction 实例操作
func (s *Server) handleInstanceAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonResponse(w, 405, map[string]string{"error": "method not allowed"})
		return
	}

	action := r.FormValue("action")
	id, _ := strconv.ParseInt(r.FormValue("id"), 10, 64)

	inst, err := s.instMgr.Get(id)
	if err != nil {
		s.renderHTML(w, "instances", &PageData{
			Title:  "实例管理",
			PageID: "instances",
			FlashMsg: "✗ 实例不存在",
		})
		return
	}

	switch action {
	case "start":
		err = s.instMgr.Start(inst)
	case "stop":
		err = s.instMgr.Stop(inst)
	case "restart":
		err = s.instMgr.Restart(inst)
	case "delete":
		err = s.instMgr.Delete(id)
	default:
		jsonResponse(w, 400, map[string]string{"error": "未知操作: " + action})
		return
	}

	if err != nil {
		s.renderHTML(w, "instances", &PageData{
			Title:  "实例管理",
			PageID: "instances",
			FlashMsg: "✗ " + err.Error(),
		})
		return
	}

	msg := map[string]string{"start": "启动", "stop": "停止", "restart": "重启", "delete": "删除"}
	http.Redirect(w, r, "/instances?msg=✓ 实例 "+inst.Name+" 已"+msg[action], http.StatusSeeOther)
}

// handleInstanceLogs 读取实例日志
func (s *Server) handleInstanceLogs(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(pathParam(r, "id"), 10, 64)
	inst, err := s.instMgr.Get(id)
	if err != nil {
		jsonResponse(w, 404, map[string]string{"error": "实例不存在"})
		return
	}

	logs, err := s.instMgr.RefreshLogs(inst, 100)
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, map[string]string{"logs": logs})
}

// handleRooms 房间列表
func (s *Server) handleRooms(w http.ResponseWriter, r *http.Request) {
	roomID := r.FormValue("instance_id")

	var rooms []*room.Room
	if roomID != "" {
		instID, _ := strconv.ParseInt(roomID, 10, 64)
		rooms, _ = s.roomMgr.ListByInstance(instID)
	}

	s.renderHTML(w, "rooms", &PageData{
		Title: "房间管理",
		PageID: "rooms",
		Rooms: rooms,
	})
}

// handleRoomAction 房间操作
func (s *Server) handleRoomAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonResponse(w, 405, map[string]string{"error": "method not allowed"})
		return
	}

	action := r.FormValue("action")
	id, _ := strconv.ParseInt(r.FormValue("id"), 10, 64)

	switch action {
	case "create":
		roomID, _ := strconv.ParseInt(r.FormValue("instance_id"), 10, 64)
		room := &room.Room{
			InstanceID:    roomID,
			Name:          r.FormValue("name"),
			WorldName:     r.FormValue("world_name"),
			MaxPlayers:    mustInt(r.FormValue("max_players")),
			Seed:          r.FormValue("seed"),
			Autopause:     r.FormValue("autopause") == "on",
			AllowTransfer: r.FormValue("allow_transfer") == "on",
			Description:   r.FormValue("description"),
		}
		err := s.roomMgr.Create(room)
		if err != nil {
			jsonResponse(w, 500, map[string]string{"error": err.Error()})
			return
		}
		http.Redirect(w, r, "/rooms?msg=✓ 房间 "+room.Name+" 创建成功", http.StatusSeeOther)

	case "delete":
		_, err := s.roomMgr.Get(id)
		if err != nil {
			jsonResponse(w, 404, map[string]string{"error": "房间不存在"})
			return
		}
		err = s.roomMgr.Delete(id)
		if err != nil {
			jsonResponse(w, 500, map[string]string{"error": err.Error()})
			return
		}
		http.Redirect(w, r, "/rooms?msg=✓ 房间已删除", http.StatusSeeOther)
	}
}

// handleMods 模组列表
func (s *Server) handleMods(w http.ResponseWriter, r *http.Request) {
	mods, _ := s.modMgr.List()
	s.renderHTML(w, "mods", &PageData{
		Title: "模组管理",
		PageID: "mods",
		Mods:  mods,
	})
}

// handleModAction 模组操作
func (s *Server) handleModAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonResponse(w, 405, map[string]string{"error": "method not allowed"})
		return
	}

	action := r.FormValue("action")
	modID := r.FormValue("mod_id")

	switch action {
	case "add":
		_, err := s.modMgr.Add(modID, r.FormValue("mod_name"), r.FormValue("mod_url"))
		if err != nil {
			http.Redirect(w, r, "/mods?msg=✗ "+err.Error(), http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/mods?msg=✓ 模组 "+r.FormValue("mod_name")+" 添加成功", http.StatusSeeOther)

	case "enable":
		_ = s.modMgr.Enable(modID, true)
		http.Redirect(w, r, "/mods?msg=✓ 模组已启用", http.StatusSeeOther)

	case "disable":
		_ = s.modMgr.Enable(modID, false)
		http.Redirect(w, r, "/mods?msg=✓ 模组已禁用", http.StatusSeeOther)

	case "delete":
		err := s.modMgr.Delete(modID)
		if err != nil {
			http.Redirect(w, r, "/mods?msg=✗ "+err.Error(), http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/mods?msg=✓ 模组已删除", http.StatusSeeOther)

	case "link":
		instID, _ := strconv.ParseInt(r.FormValue("instance_id"), 10, 64)
		err := s.modMgr.LinkToInstance(instID, modID, true)
		if err != nil {
			http.Redirect(w, r, "/mods?msg=✗ "+err.Error(), http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/mods?msg=✓ 模组已绑定到实例", http.StatusSeeOther)

	case "unlink":
		instID, _ := strconv.ParseInt(r.FormValue("instance_id"), 10, 64)
		_ = s.modMgr.UnlinkFromInstance(instID, modID)
		http.Redirect(w, r, "/mods?msg=✓ 模组已解绑", http.StatusSeeOther)

	case "download":
		_ = s.modMgr.DownloadMod(s.cfg.SteamCMDPath, modID)
		http.Redirect(w, r, "/mods?msg=✓ 模组下载完成", http.StatusSeeOther)

	default:
		jsonResponse(w, 400, map[string]string{"error": "未知操作"})
	}
}

// handleInstallSteamCMD 安装 SteamCMD
func (s *Server) handleInstallSteamCMD(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonResponse(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	err := s.installer.InstallSteamCMD()
	if err != nil {
		http.Redirect(w, r, "/install?msg=✗ "+err.Error(), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/install?msg=✓ SteamCMD 安装完成", http.StatusSeeOther)
}

// handleInstallGame 安装游戏
func (s *Server) handleInstallGame(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonResponse(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	err := s.installer.InstallDSTServer()
	if err != nil {
		http.Redirect(w, r, "/install?msg=✗ "+err.Error(), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/install?msg=✓ DST Dedicated Server 安装完成", http.StatusSeeOther)
}

// handleCheckDeps 检查系统依赖
func (s *Server) handleCheckDeps(w http.ResponseWriter, r *http.Request) {
	info := s.collectSystemInfo()
	hasSteamCMD := false
	if _, err := os.Stat(s.cfg.SteamCMDPath); err == nil {
		hasSteamCMD = true
	}
	hasGame := false
	bootPath := fmt.Sprintf("%s/bin/linux64/dedicated_server", s.cfg.GameInstallDir)
	if _, err := os.Stat(bootPath); err == nil {
		hasGame = true
	}
	jsonResponse(w, 200, map[string]interface{}{
		"OS":         info.OS,
		"Arch":       info.Arch,
		"CPU":        info.CPU,
		"IsARM64":    info.Arch == "arm64",
		"HasSteamCMD": hasSteamCMD,
		"HasGame":     hasGame,
	})
}

// API 端点
func (s *Server) apiListInstances(w http.ResponseWriter, r *http.Request) {
	instances, err := s.instMgr.List()
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, instances)
}

func (s *Server) apiListMods(w http.ResponseWriter, r *http.Request) {
	mods, err := s.modMgr.List()
	if err != nil {
		jsonResponse(w, 500, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(w, 200, mods)
}

func (s *Server) apiSystemStatus(w http.ResponseWriter, r *http.Request) {
	info := s.collectSystemInfo()
	steamCMDOK := false
	if _, err := os.Stat(s.cfg.SteamCMDPath); err == nil {
		steamCMDOK = true
	}
	gameOK := false
	bootPath := fmt.Sprintf("%s/bin/linux64/dedicated_server", s.cfg.GameInstallDir)
	if _, err := os.Stat(bootPath); err == nil {
		gameOK = true
	}
	jsonResponse(w, 200, map[string]interface{}{
		"os":        info.OS,
		"arch":      info.Arch,
		"cpu":       info.CPU,
		"cpu_count": info.CPUCount,
		"is_arm64":  info.Arch == "arm64",
		"steamcmd":  steamCMDOK,
		"game":      gameOK,
		"data_dir":  s.cfg.DataDir,
		"game_dir":  s.cfg.GameInstallDir,
	})
}

// collectSystemInfo
func (s *Server) collectSystemInfo() *struct {
	OS       string
	Arch     string
	CPU      string
	CPUCount int
} {
	cpuCount := runtime.NumCPU()
	arch := runtime.GOARCH
	os := runtime.GOOS
	cpuInfo := "Unknown"
	if runtime.GOOS == "linux" {
		cmd := exec.Command("bash", "-c", "lscpu 2>/dev/null | grep 'Model name' | head -1 | cut -d: -f2 | xargs")
		output, _ := cmd.Output()
		if len(output) > 0 {
			cpuInfo = string(output)
		}
	}
	return &struct {
		OS       string
		Arch     string
		CPU      string
		CPUCount int
	}{os, arch, cpuInfo, cpuCount}
}

func mustInt(s string) int {
	v, _ := strconv.Atoi(s)
	return v
}
