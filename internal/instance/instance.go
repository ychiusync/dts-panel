package instance

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

// Instance 代表一个 DST 游戏实例
type Instance struct {
	ID            int64     `json:"id"`
	Name          string    `json:"name"`
	Status        string    `json:"status"` // stopped, starting, running, stopped, error
	MasterPort    int       `json:"master_port"`
	ClusterPort   int       `json:"cluster_port"`
	MaxPlayers    int       `json:"max_players"`
	ServerToken   string    `json:"-"`
	ConfigDir     string    `json:"config_dir"`
	LogDir        string    `json:"log_dir"`
	GameRoot      string    `json:"game_root"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`

	// 运行时
	Process *os.Process `json:"-"`
}

// Manager 实例管理器
type Manager struct {
	db         *sql.DB
	instanceRoot string
	gameRoot   string
}

func NewManager(db *sql.DB, instanceRoot, gameRoot string) *Manager {
	return &Manager{
		db:           db,
		instanceRoot: instanceRoot,
		gameRoot:     gameRoot,
	}
}

// Create 创建新实例
func (m *Manager) Create(name string, masterPort, clusterPort, maxPlayers int, serverToken string) (*Instance, error) {
	// 检查是否已存在
	exists, err := m.exists(name)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, fmt.Errorf("实例 '%s' 已存在", name)
	}

	configDir := filepath.Join(m.instanceRoot, name, "config")
	logDir := filepath.Join(m.instanceRoot, name, "logs")
	worldDir := filepath.Join(m.instanceRoot, name, "worlds")

	for _, dir := range []string{configDir, logDir, worldDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("创建目录 %s 失败: %w", dir, err)
		}
	}

	// 写入初始 master.ini 模板
	masterIni := fmt.Sprintf(`[NETWORK]
server_port = %d

[SHARD]
shard = Master
max_players = %d
ipc_master_server_port = %d

[ACCOUNT]
encode_user_path = true
`, masterPort, maxPlayers, clusterPort)

	if err := os.WriteFile(filepath.Join(configDir, "master.ini"), []byte(masterIni), 0644); err != nil {
		return nil, fmt.Errorf("写入 master.ini 失败: %w", err)
	}

	// 写入 cluster.ini
	clusterIni := fmt.Sprintf(`[NETWORK]
cluster_name = DST Server %s
cluster_description = A new DST cluster
cluster_intention = survival
cluster_max_players = %d

[MISC]
console_enabled = true
`, name, maxPlayers)

	if err := os.WriteFile(filepath.Join(configDir, "cluster.ini"), []byte(clusterIni), 0644); err != nil {
		return nil, fmt.Errorf("写入 cluster.ini 失败: %w", err)
	}

	// 写入 server_name.txt
	if err := os.WriteFile(filepath.Join(configDir, "server_name.txt"), []byte(name), 0644); err != nil {
		return nil, fmt.Errorf("写入 server_name.txt 失败: %w", err)
	}

	// 插入数据库
	result, err := m.db.Exec(
		`INSERT INTO instances (name, status, master_port, cluster_port, max_players, server_token, config_dir, log_dir)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		name, "stopped", masterPort, clusterPort, maxPlayers, serverToken, configDir, logDir,
	)
	if err != nil {
		return nil, err
	}

	id, _ := result.LastInsertId()
	log.Printf("[instance] 创建实例: %s (id=%d)", name, id)

	return m.Get(id)
}

// Get 按 ID 获取实例
func (m *Manager) Get(id int64) (*Instance, error) {
	var inst Instance
	var createdAt, updatedAt string
	err := m.db.QueryRow(
		`SELECT id, name, status, master_port, cluster_port, max_players, server_token, config_dir, log_dir, created_at, updated_at
		 FROM instances WHERE id = ?`, id).Scan(
		&inst.ID, &inst.Name, &inst.Status, &inst.MasterPort, &inst.ClusterPort,
		&inst.MaxPlayers, &inst.ServerToken, &inst.ConfigDir, &inst.LogDir,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	inst.GameRoot = m.gameRoot
	inst.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	inst.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
	return &inst, nil
}

// List 列出所有实例
func (m *Manager) List() ([]*Instance, error) {
	rows, err := m.db.Query(
		`SELECT id, name, status, master_port, cluster_port, max_players, config_dir, log_dir, created_at
		 FROM instances ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var instances []*Instance
	for rows.Next() {
		var inst Instance
		var createdAt string
		if err := rows.Scan(&inst.ID, &inst.Name, &inst.Status, &inst.MasterPort, &inst.ClusterPort,
			&inst.MaxPlayers, &inst.ConfigDir, &inst.LogDir, &createdAt); err != nil {
			return nil, err
		}
		inst.GameRoot = m.gameRoot
		inst.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		// 刷新实际状态
		inst.Status = m.checkStatus(inst.Name)
		instances = append(instances, &inst)
	}
	return instances, nil
}

// Delete 删除实例
func (m *Manager) Delete(id int64) error {
	inst, err := m.Get(id)
	if err != nil {
		return err
	}

	// 如果正在运行，先停止
	if inst.Status == "running" {
		_ = m.Stop(inst)
	}

	// 删除目录
	_ = os.RemoveAll(filepath.Join(m.instanceRoot, inst.Name))

	// 删除数据库记录
	_, err = m.db.Exec("DELETE FROM instances WHERE id = ?", id)
	if err != nil {
		return err
	}
	log.Printf("[instance] 删除实例: %s (id=%d)", inst.Name, id)
	return nil
}

// Start 启动实例
func (m *Manager) Start(inst *Instance) error {
	if inst.Status == "running" {
		return fmt.Errorf("实例 '%s' 已在运行", inst.Name)
	}

	m.updateStatus(inst.ID, "starting")

	// 构建启动命令
	// DST 在 Linux 上的启动方式:
	//  ./bin/linux64/dedicated_server mod_index.dat cluster_dir/
	binary := filepath.Join(m.gameRoot, "bin", "linux64", "dedicated_server")
	clusterDir := filepath.Join(m.instanceRoot, inst.Name)

	// 使用 master 配置
	cmd := exec.Command(binary,
		"-shard", "Master",
		"-cluster", inst.Name,
		"-console",
	)

	// 设置环境变量
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("DST_CLUSTER_ROOT=%s", clusterDir),
		fmt.Sprintf("HOME=%s", m.instanceRoot),
		"DEDICATED_SERVER_PORT="+fmt.Sprintf("%d", inst.MasterPort),
		"DEDICATED_CLUSTER_PORT="+fmt.Sprintf("%d", inst.ClusterPort),
	)

	// 重定向日志
	logFile := filepath.Join(inst.LogDir, "server.log")
	file, _ := os.Create(logFile)
	cmd.Stdout = file
	cmd.Stderr = file

	if err := cmd.Start(); err != nil {
		m.updateStatus(inst.ID, "error")
		return fmt.Errorf("启动失败: %w", err)
	}

	inst.Process = cmd.Process

	// 后台等待终止信号
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()

		select {
		case err := <-done:
			if err != nil {
				log.Printf("[instance] %s 进程退出: %v", inst.Name, err)
			} else {
				log.Printf("[instance] %s 进程正常退出", inst.Name)
			}
			m.updateStatus(inst.ID, "stopped")
		case sig := <-sigChan:
			log.Printf("[instance] %s 收到信号 %v", inst.Name, sig)
			_ = cmd.Process.Signal(syscall.SIGTERM)
			<-done
			m.updateStatus(inst.ID, "stopped")
		}
	}()

	// 给几秒检查进程是否存活
	time.Sleep(2 * time.Second)
	if cmd.ProcessState == nil {
		m.updateStatus(inst.ID, "running")
		log.Printf("[instance] 启动实例: %s (master:%d, cluster:%d)", inst.Name, inst.MasterPort, inst.ClusterPort)
		return nil
	}

	m.updateStatus(inst.ID, "error")
	return fmt.Errorf("实例 '%s' 启动后立即退出", inst.Name)
}

// Stop 停止实例
func (m *Manager) Stop(inst *Instance) error {
	if inst.Status != "running" {
		return fmt.Errorf("实例 '%s' 未在运行", inst.Name)
	}

	// 查找进程
	processes, _ := m.findInstanceProcesses(inst.Name)
	for _, p := range processes {
		_ = p.Signal(syscall.SIGTERM)
		time.Sleep(200 * time.Millisecond)
		// 强制 kill
		_, _ = p.Wait()
	}

	m.updateStatus(inst.ID, "stopped")
	log.Printf("[instance] 停止实例: %s", inst.Name)
	return nil
}

// Restart 重启实例
func (m *Manager) Restart(inst *Instance) error {
	if inst.Status == "running" {
		_ = m.Stop(inst)
	}
	return m.Start(inst)
}

// RefreshLogs 读取最近日志
func (m *Manager) RefreshLogs(inst *Instance, tail int) (string, error) {
	logFile := filepath.Join(inst.LogDir, "server.log")
	if _, err := os.Stat(logFile); err != nil {
		return "", nil
	}

	cmd := exec.Command("tail", "-n", fmt.Sprintf("%d", tail), logFile)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

// 辅助方法
func (m *Manager) exists(name string) (bool, error) {
	var count int
	err := m.db.QueryRow("SELECT COUNT(*) FROM instances WHERE name = ?", name).Scan(&count)
	return count > 0, err
}

func (m *Manager) updateStatus(id int64, status string) {
	_, _ = m.db.Exec("UPDATE instances SET status = ?, updated_at = datetime('now') WHERE id = ?", status, id)
}

func (m *Manager) checkStatus(name string) string {
	processes, _ := m.findInstanceProcesses(name)
	if len(processes) > 0 {
		return "running"
	}
	return "stopped"
}

func (m *Manager) findInstanceProcesses(name string) ([]*os.Process, error) {
	// 用 pgrep 查找
	cmd := exec.Command("bash", "-c", fmt.Sprintf("pgrep -f 'dedicated_server.*%s'", name))
	output, err := cmd.Output()
	if err != nil {
		return nil, nil
	}

	var procs []*os.Process
	lines := filepath.SplitList(string(output))
	for _, line := range lines {
		if pid := parseInt(line); pid > 0 {
			p, _ := os.FindProcess(pid)
			procs = append(procs, p)
		}
	}
	return procs, nil
}

func parseInt(s string) int {
	var n int
	_, _ = fmt.Sscanf(s, "%d", &n)
	return n
}
