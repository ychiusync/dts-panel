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
