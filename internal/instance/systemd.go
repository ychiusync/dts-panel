package instance

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/dts-panel/dts-panel/internal/db"
)

func (m *Manager) GenerateSystemdUnit(inst *db.Instance) (string, error) {
	binary := filepath.Join(m.gameRoot, "bin", "linux64", "dedicated_server")
	clusterDir := filepath.Join(m.instanceRoot, inst.Name)
	serviceName := fmt.Sprintf("dts-%s", inst.Name)

	content := fmt.Sprintf(`[Unit]
Description=DST Dedicated Server - %s
After=network.target

[Service]
Type=simple
User=root
ExecStart=%s -shard Master -cluster %s -console
WorkingDirectory=%s
Environment="DST_CLUSTER_ROOT=%s"
Environment="HOME=%s"
Environment="DEDICATED_SERVER_PORT=%d"
Environment="DEDICATED_CLUSTER_PORT=%d"
Restart=on-failure
RestartSec=10
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
`,
		inst.Name,
		binary,
		inst.Name,
		clusterDir,
		clusterDir,
		m.instanceRoot,
		inst.MasterPort,
		inst.ClusterPort,
	)

	unitPath := filepath.Join("/etc/systemd/system", serviceName+".service")
	if err := os.WriteFile(unitPath, []byte(content), 0644); err != nil {
		altPath := filepath.Join(m.instanceRoot, inst.Name, serviceName+".service")
		log.Printf("[systemd] 无法写入 /etc，保存至 %s", altPath)
		_ = os.MkdirAll(filepath.Dir(altPath), 0755)
		_ = os.WriteFile(altPath, []byte(content), 0644)
		return content, nil
	}

	_ = exec.Command("systemctl", "daemon-reload").Run()
	log.Printf("[systemd] 生成 systemd 单元: %s.service", serviceName)
	return content, nil
}

func (m *Manager) EnableSystemd(inst *db.Instance) error {
	serviceName := fmt.Sprintf("dts-%s", inst.Name)
	_ = exec.Command("systemctl", "daemon-reload").Run()
	_ = exec.Command("systemctl", "enable", serviceName+".service").Run()
	_ = exec.Command("systemctl", "start", serviceName+".service").Run()
	return nil
}

func (m *Manager) DisableSystemd(inst *db.Instance) error {
	serviceName := fmt.Sprintf("dts-%s", inst.Name)
	_ = exec.Command("systemctl", "stop", serviceName+".service").Run()
	_ = exec.Command("systemctl", "disable", serviceName+".service").Run()
	return nil
}
