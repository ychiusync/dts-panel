package install

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// ==================== 日志流 ====================

type InstallLog struct {
	mu    sync.Mutex
	lines []string
}

func (l *InstallLog) Write(s string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lines = append(l.lines, s)
	log.Print(s)
}

func (l *InstallLog) Lines() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.lines...)
}

var globalLog = &InstallLog{}

// ==================== 下载源 ====================

var steamCMDSources = []struct {
	URL string
	Ext string
	Cmd string
}{
	{
		URL: "https://steamcdn-a.akamaihd.net/client/installer/steamcmd_linux.tar.gz",
		Ext: ".tar.gz",
		Cmd: "wget -q --timeout=30 --tries=3 -O %s https://steamcdn-a.akamaihd.net/client/installer/steamcmd_linux.tar.gz",
	},
}

type SteamCMDInstaller struct {
	BaseDir      string
	SteamCMDPath string
	GameDir      string
}

func NewSteamCMDInstaller(baseDir, steamCMDPath, gameDir string) *SteamCMDInstaller {
	return &SteamCMDInstaller{BaseDir: baseDir, SteamCMDPath: steamCMDPath, GameDir: gameDir}
}

// ==================== Box64/Box86 安装 ====================

var (
	box64Once sync.Once
	box64Bin  string
	box86Once sync.Once
	box86Bin  string
)

func getBox64Bin() string {
	box64Once.Do(func() {
		if bin, err := exec.LookPath("box64"); err == nil {
			box64Bin = bin
			return
		}
		for _, c := range []string{"/opt/box64/box64", "/usr/local/bin/box64", "/usr/bin/box64"} {
			if _, err := os.Stat(c); err == nil {
				box64Bin = c
				return
			}
		}
	})
	return box64Bin
}

// upgradeBox64IfNeeded 检查 Box64 版本，< 0.3.0 返回 false（需要升级）
func upgradeBox64IfNeeded(bin string) bool {
	out, _ := exec.Command("bash", "-c", bin+" --version 2>&1 | head -1").Output()
	ver := strings.TrimSpace(string(out))
	// 尝试提取版本号 (如 v0.2.6)
	parts := strings.Split(ver, "v")
	if len(parts) < 2 {
		return false // 无法解析版本，假设太旧
	}
	version := parts[1]
	num := strings.Split(version, " ")[0]
	major := 0
	minor := 0
	fmt.Sscanf(num, "%d.%d", &major, &minor)
	if major > 0 || (major == 0 && minor >= 3) {
		globalLog.Write(fmt.Sprintf("[install] ✓ Box64 已安装 (%s)", ver))
		return true
	}
	globalLog.Write(fmt.Sprintf("[install] Box64 版本 %s 过旧（需 ≥ 0.3.0），将升级到最新版", num))
	return false
}

func getBox86Bin() string {
	box86Once.Do(func() {
		if bin, err := exec.LookPath("box86"); err == nil {
			box86Bin = bin
			return
		}
		for _, c := range []string{"/opt/box86/box86", "/usr/local/bin/box86", "/usr/bin/box86"} {
			if _, err := os.Stat(c); err == nil {
				box86Bin = c
				return
			}
		}
	})
	return box86Bin
}

func runBash(pwd, script string) error {
	cmd := exec.Command("bash", "-c", script)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if pwd != "" {
		cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive", "NEEDRESTART_MODE=a")
		cmd.Stdin = strings.NewReader(pwd + "\n")
	}
	return cmd.Run()
}

// extractSemverTag 从 GitHub release HTML 提取版本号
func extractSemverTag(html string) string {
	i := strings.Index(html, "semver:")
	if i < 0 {
		return ""
	}
	start := i + 7
	end := start
	for end < len(html) && (html[end] >= '0' && html[end] <= '9' || html[end] == '.') {
		end++
	}
	if end > start {
		return "v" + html[start:end]
	}
	return ""
}

// downloadAndInstallBin 从 zip 下载解压安装
func downloadAndInstallBin(url, name, destDir string) error {
	tmpZip := "/tmp/" + name + ".zip"
	globalLog.Write(fmt.Sprintf("[install] 下载中... %s", url))
	if err := exec.Command("bash", "-c",
		fmt.Sprintf("curl -fsSL --connect-timeout 20 --retry 3 -o %s %s || wget -q --timeout=20 --tries=3 -O %s %s",
			tmpZip, url, tmpZip, url)).Run(); err != nil {
		return fmt.Errorf("下载失败: %w", err)
	}
	info, err := os.Stat(tmpZip)
	if err != nil || info.Size() < 10000 {
		_ = os.Remove(tmpZip)
		return fmt.Errorf("文件异常 (size=%d)", info.Size())
	}
	globalLog.Write(fmt.Sprintf("[install] 下载完成 (%d bytes)，正在解压...", info.Size()))
	_ = os.MkdirAll(destDir, 0755)
	if err := exec.Command("unzip", "-q", "-o", tmpZip, "-d", destDir).Run(); err != nil {
		_ = os.Remove(tmpZip)
		return fmt.Errorf("解压失败: %w", err)
	}
	_ = os.Remove(tmpZip)
	_ = exec.Command("chmod", "+x", filepath.Join(destDir, name)).Run()
	_ = exec.Command("ln", "-sf", filepath.Join(destDir, name), "/usr/local/bin/"+name).Run()
	globalLog.Write(fmt.Sprintf("[install] %s 安装完成", name))
	return nil
}

// installBox64ViaGitHub 从 GitHub Releases 下载最新版 Box64
// 优先用已知稳定的版本（v0.3.5），失败再尝试解析最新 tag
func installBox64ViaGitHub() error {
	// 策略 1: 用已知稳定的静态版本
	staticURL := "https://github.com/ptitSeb/box64/releases/download/v0.3.5/box64-v0.3.5.zip"
	globalLog.Write("[install] 下载 Box64 (v0.3.5) ...")
	err := downloadAndInstallBin(staticURL, "box64", "/usr/local/box64")
	if err == nil {
		return nil
	}
	globalLog.Write(fmt.Sprintf("[install] 静态版本下载失败 (%v)，尝试获取最新 tag...", err))

	// 策略 2: 解析 release 页面获取最新 tag
	globalLog.Write("[install] 正在从 GitHub 获取 Box64 最新版本...")
	cmd := exec.Command("bash", "-c",
		"curl -sL --connect-timeout 20 --retry 2 https://github.com/ptitSeb/box64/releases/latest || wget -qO- --timeout=20 --tries=2 https://github.com/ptitSeb/box64/releases/latest")
	html, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("获取 release 页面失败: %w", err)
	}
	tag := extractSemverTag(string(html))
	if tag == "" {
		return fmt.Errorf("无法提取 tag")
	}
	globalLog.Write(fmt.Sprintf("[install] Box64 最新 tag: %s", tag))
	zipURL := fmt.Sprintf("https://github.com/ptitSeb/box64/releases/download/%s/box64-%s.zip", tag, tag)
	return downloadAndInstallBin(zipURL, "box64", "/usr/local/box64")
}

// InstallBox86And64 检查并安装 Box64 + Box86
func InstallBox86And64() error {
	pwd := os.Getenv("DTS_SUDO_PASSWORD")
	globalLog.Write("[install] 检查 Box64 / Box86...")

	// --- Box64（必需，需 ≥ 0.3.0 才能执行脚本）---
	bin, found := func() (string, bool) {
		if b, err := exec.LookPath("box64"); err == nil {
			return b, true
		}
		for _, c := range []string{"/opt/box64/box64", "/usr/local/bin/box64", "/usr/bin/box64"} {
			if _, err := os.Stat(c); err == nil {
				return c, true
			}
		}
		return "", false
	}()

	if found && upgradeBox64IfNeeded(bin) {
		// 已安装且版本够新
	} else {
		globalLog.Write("[install] Box64 未安装或版本过旧，从 GitHub 下载最新版...")
		if err := installBox64ViaGitHub(); err != nil {
			globalLog.Write(fmt.Sprintf("[install] GitHub 下载失败: %v", err))
			if pwd != "" {
				globalLog.Write("[install] 回退 apt 安装...")
				script := `wget -q --timeout=10 --tries=2 https://ryanfortner.github.io/box64-debs/box64.list -O /etc/apt/sources.list.d/box64.list
wget -qO- --timeout=10 --tries=2 https://ryanfortner.github.io/box64-debs/KEY.gpg | gpg --dearmor --yes -o /etc/apt/trusted.gpg.d/box64-debs-archive-keyring.gpg
apt-get update -qq
DEBIAN_FRONTEND=noninteractive apt-get install -y -q box64`
				if err2 := runBash(pwd, script); err2 != nil {
					return fmt.Errorf("Box64 安装失败（GitHub 和 apt 都不可用），请手动安装")
				}
				globalLog.Write("[install] Box64 apt 安装成功")
			} else {
				return fmt.Errorf("Box64 未安装，请设置 DTS_SUDO_PASSWORD 环境变量或手动安装")
			}
		}
		// 安装成功后重置缓存
		box64Once = sync.Once{}
	}

	// --- Box86（可选，32位）---
	if _, err := exec.LookPath("box86"); err == nil {
		globalLog.Write("[install] ✓ Box86 已安装")
	} else {
		globalLog.Write("[install] Box86 未安装（可忽略，不影响 64 位游戏运行）")
	}

	// 重置缓存
	box64Once = sync.Once{}
	return nil
}

// ==================== SteamCMD 安装 ====================

func probeURL(url string) bool {
	resp, err := http.Head(url)
	if err != nil {
		resp, err = http.Get(url)
	}
	if err != nil {
		return false
	}
	if resp.Body != nil {
		resp.Body.Close()
	}
	return resp.StatusCode == 200
}

func (i *SteamCMDInstaller) InstallSteamCMD() error {
	steamDir := filepath.Dir(i.SteamCMDPath)
	if err := os.MkdirAll(steamDir, 0755); err != nil {
		return fmt.Errorf("创建 steamcmd 目录失败: %w", err)
	}
	if _, err := os.Stat(i.SteamCMDPath); err == nil {
		globalLog.Write("[install] SteamCMD 已存在，跳过下载")
		return nil
	}
	globalLog.Write("[install] 下载 SteamCMD...")
	for _, src := range steamCMDSources {
		globalLog.Write(fmt.Sprintf("[install] 探测源: %s", src.URL))
		if probeURL(src.URL) {
			globalLog.Write(fmt.Sprintf("[install] 源可用，下载中..."))
			tmpPath := filepath.Join(steamDir, "steamcmd"+src.Ext)
			cmd := exec.Command("bash", "-c", fmt.Sprintf(src.Cmd, tmpPath))
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				globalLog.Write(fmt.Sprintf("[install] 下载失败: %v", err))
				return fmt.Errorf("下载 steamcmd 失败: %w", err)
			}
			info, _ := os.Stat(tmpPath)
			globalLog.Write(fmt.Sprintf("[install] 下载完成 (%d bytes)，正在解压...", info.Size()))
			if err := extractSteamCMD(tmpPath, steamDir); err != nil {
				return fmt.Errorf("解压 steamcmd 失败: %w", err)
			}
			_ = os.Remove(tmpPath)
			break
		}
	}
	if _, err := os.Stat(i.SteamCMDPath); err != nil {
		globalLog.Write("[install] 直接下载（无探测）...")
		tmpPath := filepath.Join(steamDir, "steamcmd.tar.gz")
		cmd := exec.Command("bash", "-c", fmt.Sprintf(steamCMDSources[0].Cmd, tmpPath))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Run()
		if info, err := os.Stat(tmpPath); err == nil && info.Size() > 100000 {
			_ = extractSteamCMD(tmpPath, steamDir)
			_ = os.Remove(tmpPath)
		}
	}

	_ = os.Chmod(i.SteamCMDPath, 0755)
	globalLog.Write("[install] SteamCMD 安装完成")
	return nil
}

func extractSteamCMD(archivePath, destDir string) error {
	base := filepath.Base(archivePath)
	if strings.HasSuffix(archivePath, ".tar.gz") || base == "steamcmd.tar.gz" {
		if err := exec.Command("tar", "-xzf", archivePath, "-C", destDir).Run(); err != nil {
			return err
		}
	} else if strings.HasSuffix(archivePath, ".zip") {
		if err := exec.Command("unzip", "-q", archivePath, "-d", destDir).Run(); err != nil {
			return err
		}
	}
	// 把子目录移平
	sub := filepath.Join(destDir, "steamcmd")
	entries, _ := os.ReadDir(sub)
	if len(entries) > 0 {
		for _, e := range entries {
			_ = os.Rename(filepath.Join(sub, e.Name()), filepath.Join(destDir, e.Name()))
		}
		_ = os.Remove(sub)
	}
	return nil
}

// ==================== SteamCMD 执行（ARM 下通过 Box64）====================

// buildSteamCMDCommand 构建 SteamCMD 命令，多层 fallback
func (i *SteamCMDInstaller) buildSteamCMDCommand(args []string) *exec.Cmd {
	steamDir := filepath.Dir(i.SteamCMDPath)

	if runtime.GOARCH != "arm64" {
		return exec.Command(filepath.Join(steamDir, "steamcmd.sh"), args...)
	}

	box64 := getBox64Bin()
	if box64 == "" {
		return exec.Command("echo", "无法找到 Box64")
	}

	// 尝试顺序：
	// 1) linux64/steamcmd（64位 ELF，box64 运行最佳）
	// 2) linux32/steamcmd（32位 ELF，box64 新版可能支持）
	// 3) steamcmd.sh（shell 脚本，需 box64 0.3.0+）
	for _, candidate := range []string{
		"linux64/steamcmd",
		"steamcmd.sh",
		"linux32/steamcmd",
	} {
		steamBin := filepath.Join(steamDir, candidate)
		if _, err := os.Stat(steamBin); err == nil {
			globalLog.Write(fmt.Sprintf("[install] 使用 %s", candidate))
			cmd := exec.Command(box64, append([]string{steamBin}, args...)...)
			cmd.Dir = steamDir
			cmd.Env = append(os.Environ(), "HOME="+i.BaseDir, "LD_LIBRARY_PATH="+steamDir+"/linux64:"+steamDir+"/linux32")
			return cmd
		}
	}

	return exec.Command("echo", "无可用 SteamCMD 二进制")
}

// ==================== DST 安装 ====================

func (i *SteamCMDInstaller) InstallDSTServer() error {
	if _, err := os.Stat(i.SteamCMDPath); err != nil {
		return fmt.Errorf("SteamCMD 未安装，请先调用 InstallSteamCMD")
	}

	globalLog.Write("[install] 开始安装 DST Dedicated Server...")
	arch := runtime.GOARCH
	globalLog.Write(fmt.Sprintf("[install] 当前架构: %s", arch))

	// ARM64: 确保 Box64 已安装
	if arch == "arm64" {
		if err := InstallBox86And64(); err != nil {
			globalLog.Write(fmt.Sprintf("[install] Box 安装警告: %v", err))
		}
	}

	steamDir := filepath.Dir(i.SteamCMDPath)

	// 第一步：SteamCMD 自更新（生成 linux64/steamcmd）
	// 确保 linux64/steamcmd 存在（SteamCMD 自更新后才会生成）
	steam64Bin := filepath.Join(steamDir, "linux64", "steamcmd")
	if _, err := os.Stat(steam64Bin); err != nil {
		globalLog.Write("[install] 第 1 步: SteamCMD 自更新（生成 linux64/steamcmd）...")
		box64Bin := getBox64Bin()
		if box64Bin == "" {
			globalLog.Write("[install] 无 Box64，无法自更新 SteamCMD")
		} else {
			// 新版 box64 (0.3+) 支持执行 steamcmd.sh 脚本
			steamSh := filepath.Join(steamDir, "steamcmd.sh")
			selfUpdateCmd := exec.Command(box64Bin, steamSh, "+force_install_dir", steamDir, "+login", "anonymous", "+quit")
			selfUpdateCmd.Dir = steamDir
			selfUpdateCmd.Env = append(os.Environ(), "HOME="+i.BaseDir)
			selfUpdateCmd.Stdout = os.Stdout
			selfUpdateCmd.Stderr = os.Stderr
			if err := selfUpdateCmd.Run(); err != nil {
				globalLog.Write(fmt.Sprintf("[install] SteamCMD 自更新失败: %v", err))
			} else {
				globalLog.Write("[install] SteamCMD 自更新完成")
			}
			// 再次检查
			if _, err := os.Stat(steam64Bin); err != nil {
				globalLog.Write("[install] SteamCMD 自更新后仍未生成 linux64/steamcmd")
			} else {
				globalLog.Write("[install] ✓ linux64/steamcmd 已生成")
			}
		}
	} else {
		globalLog.Write("[install] ✓ linux64/steamcmd 已存在，跳过自更新")
	}

	// 第二步：下载 DST
	globalLog.Write("[install] 第 2 步: 通过 SteamCMD 下载 DST...")
	installArgs := []string{
		"+force_install_dir", i.GameDir,
		"+login", "anonymous",
		"+app_update", "343050", "validate",
		"+quit",
	}
	for attempt := 1; attempt <= 2; attempt++ {
		globalLog.Write(fmt.Sprintf("[install] 第 %d 次尝试下载 DST...", attempt))
		cmd := i.buildSteamCMDCommand(installArgs)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			globalLog.Write(fmt.Sprintf("[install] 第 %d 次失败: %v", attempt, err))
			if attempt == 2 {
				return fmt.Errorf("DST 安装失败")
			}
		} else {
			globalLog.Write(fmt.Sprintf("[install] DST 下载完成（第 %d 次）", attempt))
			break
		}
	}

	// 清理损坏的 acf
	_ = os.Remove(filepath.Join(i.GameDir, "steamapps", "appmanifest_108730.acf"))

	bootPath := filepath.Join(i.GameDir, "bin", "linux64", "dedicated_server")
	if _, err := os.Stat(bootPath); err == nil {
		globalLog.Write("[install] ✓ DST Dedicated Server 安装完成")
		return nil
	}
	return fmt.Errorf("DST 安装完成但文件不完整")
}

func (i *SteamCMDInstaller) UpdateDSTServer() error {
	globalLog.Write("[install] 更新 DST...")
	installArgs := []string{
		"+force_install_dir", i.GameDir,
		"+login", "anonymous",
		"+app_update", "343050", "validate",
		"+quit",
	}
	cmd := i.buildSteamCMDCommand(installArgs)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (i *SteamCMDInstaller) Verify() error {
	bootPath := filepath.Join(i.GameDir, "bin", "linux64", "dedicated_server")
	if _, err := os.Stat(bootPath); err != nil {
		return fmt.Errorf("DST 未安装: %w", err)
	}
	globalLog.Write("[install] ✓ 验证通过")
	return nil
}
