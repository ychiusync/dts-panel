package instance

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"gorm.io/gorm"
	"github.com/dts-panel/dts-panel/internal/db"
)

type Manager struct {
	db           *gorm.DB
	instanceRoot string
	gameRoot     string
}

func NewManager(gormDB *gorm.DB, instanceRoot, gameRoot string) *Manager {
	return &Manager{db: gormDB, instanceRoot: instanceRoot, gameRoot: gameRoot}
}

func (m *Manager) Create(name string, masterPort, clusterPort, maxPlayers int, serverToken string) (*db.Instance, error) {
	var inst db.Instance
	if err := m.db.Where("name = ?", name).First(&inst).Error; err == nil {
		return nil, fmt.Errorf("实例 '%s' 已存在", name)
	}

	inst = db.Instance{
		Name:        name,
		Status:      "stopped",
		MasterPort:  masterPort,
		ClusterPort: clusterPort,
		MaxPlayers:  maxPlayers,
		ServerToken: serverToken,
		ConfigDir:   filepath.Join(m.instanceRoot, name, "config"),
		LogDir:      filepath.Join(m.instanceRoot, name, "logs"),
		GameRoot:    m.gameRoot,
	}

	for _, dir := range []string{inst.ConfigDir, inst.LogDir, filepath.Join(m.instanceRoot, name, "worlds")} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, err
		}
	}

	// 写入配置
	ini := fmt.Sprintf("[NETWORK]\nserver_port = %d\n\n[SHARD]\nshard = Master\nmax_players = %d\nipc_master_server_port = %d\n\n[ACCOUNT]\nencode_user_path = true\n",
		inst.MasterPort, inst.MaxPlayers, inst.ClusterPort)
	_ = os.WriteFile(filepath.Join(inst.ConfigDir, "master.ini"), []byte(ini), 0644)

	clusterIni := fmt.Sprintf("[NETWORK]\ncluster_name = DST Server %s\ncluster_description = A new DST cluster\ncluster_intention = survival\ncluster_max_players = %d\n\n[MISC]\nconsole_enabled = true\n",
		inst.Name, inst.MaxPlayers)
	_ = os.WriteFile(filepath.Join(inst.ConfigDir, "cluster.ini"), []byte(clusterIni), 0644)
	_ = os.WriteFile(filepath.Join(inst.ConfigDir, "server_name.txt"), []byte(name), 0644)

	if err := m.db.Create(&inst).Error; err != nil {
		return nil, err
	}
	log.Printf("[instance] 创建实例: %s (id=%d)", name, inst.ID)
	return &inst, nil
}

func (m *Manager) Get(id int64) (*db.Instance, error) {
	var inst db.Instance
	if err := m.db.First(&inst, id).Error; err != nil {
		return nil, err
	}
	inst.GameRoot = m.gameRoot
	inst.Status = m.checkStatus(inst.Name)
	return &inst, nil
}

func (m *Manager) List() ([]*db.Instance, error) {
	var insts []*db.Instance
	m.db.Order("created_at DESC").Find(&insts)
	for _, i := range insts {
		i.GameRoot = m.gameRoot
		i.Status = m.checkStatus(i.Name)
	}
	return insts, nil
}

func (m *Manager) Delete(id int64) error {
	inst, _ := m.Get(id)
	if inst == nil {
		return fmt.Errorf("实例不存在")
	}
	if inst.Status == "running" {
		_ = m.Stop(inst)
	}
	_ = os.RemoveAll(filepath.Join(m.instanceRoot, inst.Name))
	m.db.Where("id = ?", id).Delete(&db.Instance{})
	_ = m.db.Where("instance_id = ?", id).Delete(&db.Room{}).Error
	log.Printf("[instance] 删除实例: %s (id=%d)", inst.Name, id)
	return nil
}

func (m *Manager) Start(inst *db.Instance) error {
	if inst.Status == "running" {
		return fmt.Errorf("实例 '%s' 已在运行", inst.Name)
	}
	m.db.Model(inst).Update("status", "starting")

	binary := filepath.Join(m.gameRoot, "bin", "linux64", "dedicated_server")
	clusterDir := filepath.Join(m.instanceRoot, inst.Name)

	cmd := exec.Command(binary, "-shard", "Master", "-cluster", inst.Name, "-console")
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("DST_CLUSTER_ROOT=%s", clusterDir),
		"HOME="+m.instanceRoot,
		"DEDICATED_SERVER_PORT="+strconv.FormatInt(int64(inst.MasterPort), 10),
		"DEDICATED_CLUSTER_PORT="+strconv.FormatInt(int64(inst.ClusterPort), 10),
	)

	logFile := filepath.Join(inst.LogDir, "server.log")
	file, _ := os.Create(logFile)
	cmd.Stdout = file
	cmd.Stderr = file

	if err := cmd.Start(); err != nil {
		m.db.Model(inst).Update("status", "error")
		return err
	}

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		select {
		case err := <-done:
			if err != nil {
				log.Printf("[instance] %s 进程退出: %v", inst.Name, err)
			}
			m.db.Model(inst).Update("status", "stopped")
		case <-sigChan:
			_ = cmd.Process.Signal(syscall.SIGTERM)
			<-done
			m.db.Model(inst).Update("status", "stopped")
		}
	}()

	time.Sleep(2 * time.Second)
	if cmd.ProcessState == nil {
		m.db.Model(inst).Update("status", "running")
		log.Printf("[instance] 启动实例: %s (master:%d, cluster:%d)", inst.Name, inst.MasterPort, inst.ClusterPort)
		return nil
	}
	m.db.Model(inst).Update("status", "error")
	return fmt.Errorf("实例 '%s' 启动后立即退出", inst.Name)
}

func (m *Manager) Stop(inst *db.Instance) error {
	procs, _ := m.findProcesses(inst.Name)
	for _, p := range procs {
		_ = p.Signal(syscall.SIGTERM)
		time.Sleep(200 * time.Millisecond)
		_, _ = p.Wait()
	}
	m.db.Model(inst).Update("status", "stopped")
	return nil
}

func (m *Manager) Restart(inst *db.Instance) error {
	if inst.Status == "running" {
		_ = m.Stop(inst)
	}
	return m.Start(inst)
}

func (m *Manager) RefreshLogs(inst *db.Instance, tail int) (string, error) {
	logFile := filepath.Join(inst.LogDir, "server.log")
	if _, err := os.Stat(logFile); err != nil {
		return "", nil
	}
	cmd := exec.Command("tail", "-n", strconv.Itoa(tail), logFile)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func (m *Manager) checkStatus(name string) string {
	procs, _ := m.findProcesses(name)
	if len(procs) > 0 {
		return "running"
	}
	return "stopped"
}

func (m *Manager) findProcesses(name string) ([]*os.Process, error) {
	cmd := exec.Command("bash", "-c", fmt.Sprintf("pgrep -f 'dedicated_server.*%s'", name))
	out, _ := cmd.Output()
	var procs []*os.Process
	for _, line := range filepath.SplitList(string(out)) {
		pid, _ := strconv.Atoi(line)
		if pid > 0 {
			p, _ := os.FindProcess(pid)
			procs = append(procs, p)
		}
	}
	return procs, nil
}
