package install

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// SystemDepChecker 检查系统依赖
type SystemDepChecker struct {
	Packages []string
}

func NewSystemDepChecker() *SystemDepChecker {
	return &SystemDepChecker{
		Packages: []string{
			"libc6",
			"libstdc++6",
			"libglx-mesa0",
			"libgl1",
			"libglib2.0-0",
			"libgl1-mesa-dri",
			"libgcc-s1",
		},
	}
}

// CheckAndInstall 检查并安装系统依赖
func (s *SystemDepChecker) CheckAndInstall(useSudo bool) error {
	if runtime.GOOS != "linux" {
		log.Println("[deps] 非 Linux 系统，跳过依赖检查")
		return nil
	}

	pkgMgr, err := s.detectPackageManager()
	if err != nil {
		log.Printf("[deps] 未检测到支持的包管理器: %v", err)
		return nil
	}

	log.Printf("[deps] 使用包管理器: %s", pkgMgr.name)

	installed, err := s.checkInstalled(pkgMgr)
	if err != nil {
		log.Printf("[deps] 检查已安装包失败: %v", err)
		return nil
	}

	missing := make([]string, 0)
	for _, pkg := range s.Packages {
		if !installed[pkg] {
			missing = append(missing, pkg)
		}
	}

	if len(missing) == 0 {
		log.Println("[deps] 所有依赖已安装")
		return nil
	}

	log.Printf("[deps] 缺少依赖: %v", missing)

	if useSudo {
		log.Println("[deps] 正在安装缺少的依赖（需要 sudo 权限）...")
		return s.installPackages(pkgMgr, missing)
	}

	return fmt.Errorf("缺少系统依赖 %v，请在服务器上使用 sudo 手动安装", missing)
}

type PackageManager struct {
	name    string
	check   string
	install string
}

func (s *SystemDepChecker) detectPackageManager() (*PackageManager, error) {
	if exec.Command("bash", "-c", "which apt-get >/dev/null 2>&1").Run() == nil {
		return &PackageManager{
			name:    "apt",
			check:   "dpkg -s %s 2>/dev/null | grep -q Status",
			install: "DEBIAN_FRONTEND=noninteractive apt-get install -y %s",
		}, nil
	}
	if exec.Command("bash", "-c", "which dnf >/dev/null 2>&1 || which yum >/dev/null 2>&1").Run() == nil {
		tool := "dnf"
		if exec.Command("which", "dnf").Run() != nil {
			tool = "yum"
		}
		return &PackageManager{
			name:    tool,
			check:   "rpm -q %s >/dev/null 2>&1",
			install: fmt.Sprintf("%s install -y %%s", tool),
		}, nil
	}
	return nil, fmt.Errorf("未检测到支持的包管理器 (apt/dnf/yum)")
}

func (s *SystemDepChecker) checkInstalled(pm *PackageManager) (map[string]bool, error) {
	installed := make(map[string]bool)
	for _, pkg := range s.Packages {
		cmd := exec.Command("bash", "-c", fmt.Sprintf(pm.check, pkg))
		installed[pkg] = cmd.Run() == nil
	}
	return installed, nil
}

func (s *SystemDepChecker) installPackages(pm *PackageManager, missing []string) error {
	if len(missing) == 0 {
		return nil
	}
	pkgArg := strings.Join(missing, " ")
	cmd := exec.Command("sudo", "bash", "-c", fmt.Sprintf(pm.install, pkgArg))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("安装依赖失败: %w", err)
	}
	return nil
}

// ============================================================
//  Steam CDN hosts 加速（国内网络优化）
// ============================================================

var steamCDNHosts = []struct {
	IP      string
	Domain  string
}{
	{"103.46.209.92", "steamcontent-dal-01.cdn.steampipe.steamcontent.com"},
	{"103.46.209.92", "steamcontent-atl-01.cdn.steampipe.steamcontent.com"},
	{"103.46.209.92", "steamcontent-fra-01.cdn.steampipe.steamcontent.com"},
	{"103.46.209.92", "steamcontent-ams-01.cdn.steampipe.steamcontent.com"},
	{"103.46.209.92", "steamcontent-sin-01.cdn.steampipe.steamcontent.com"},
	{"103.46.209.92", "steamcontent-hkg-01.cdn.steampipe.steamcontent.com"},
}

const hostsPath = "/etc/hosts"

// AddSteamCDNHosts 将 Steam CDN 节点加入 /etc/hosts 加速下载
func AddSteamCDNHosts() error {
	hosts, err := os.ReadFile(hostsPath)
	if err != nil {
		return fmt.Errorf("读取 %s 失败: %w", hostsPath, err)
	}
	hostsContent := string(hosts)

	// 检查是否已经存在
	for _, entry := range steamCDNHosts {
		if strings.Contains(hostsContent, entry.Domain) {
			log.Println("[install] Steam CDN hosts 已存在，无需重复添加")
			return nil
		}
	}

	// 追加到 hosts 文件
	lines := make([]string, 0)
	for _, entry := range steamCDNHosts {
		lines = append(lines, entry.IP+"\t"+entry.Domain)
	}

	newContent := "\n# Steam CDN (added by dts-panel)\n" + strings.Join(lines, "\n") + "\n"

	// 用 sudo sh 写入
	cmd := exec.Command("bash", "-c", fmt.Sprintf("echo '%s' >> %s", strings.ReplaceAll(newContent, "'", "'\\''"), hostsPath))
	cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	// 密码注入
	if pwd := os.Getenv("DTS_SUDO_PASSWORD"); pwd != "" {
		cmd.Stdin = strings.NewReader(pwd + "\n")
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Println("[install] 正在添加 Steam CDN hosts 映射...")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("添加 hosts 失败: %w", err)
	}

	log.Println("[install] ✓ Steam CDN hosts 添加完成")
	return nil
}

// RemoveSteamCDNHosts 移除 Steam CDN hosts 映射
func RemoveSteamCDNHosts() error {
	_, err := os.ReadFile(hostsPath)
	if err != nil {
		return fmt.Errorf("读取 %s 失败: %w", hostsPath, err)
	}

	// 用 awk 过滤掉我们的条目
	script := `awk '/# Steam CDN/ || /steamcontent.*cdn.steampipe.steamcontent.com/ {next} {print}'`
	cmd := exec.Command("bash", "-c", fmt.Sprintf("%s %s > /tmp/hosts.tmp && mv /tmp/hosts.tmp %s", script, hostsPath, hostsPath))
	cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	if pwd := os.Getenv("DTS_SUDO_PASSWORD"); pwd != "" {
		cmd.Stdin = strings.NewReader(pwd + "\n")
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("移除 hosts 失败: %w", err)
	}

	log.Println("[install] ✓ Steam CDN hosts 已移除")
	return nil
}

// CheckSteamCDNHosts 检查 Steam CDN hosts 是否存在
func CheckSteamCDNHosts() bool {
	hostsData, err := os.ReadFile(hostsPath)
	if err != nil {
		return false
	}
	return strings.Contains(string(hostsData), "steamcontent")
}
