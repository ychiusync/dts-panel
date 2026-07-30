package install

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

// SteamCMDInstaller 负责 SteamCMD 和游戏安装
type SteamCMDInstaller struct {
	BaseDir      string
	SteamCMDPath string
	GameDir      string
}

func NewSteamCMDInstaller(baseDir, steamCMDPath, gameDir string) *SteamCMDInstaller {
	return &SteamCMDInstaller{
		BaseDir:      baseDir,
		SteamCMDPath: steamCMDPath,
		GameDir:      gameDir,
	}
}

// InstallSteamCMD 下载并安装 SteamCMD (aarch64)
func (i *SteamCMDInstaller) InstallSteamCMD() error {
	steamDir := filepath.Dir(i.SteamCMDPath)
	if err := os.MkdirAll(steamDir, 0755); err != nil {
		return fmt.Errorf("创建 steamcmd 目录失败: %w", err)
	}

	// 如果已存在，跳过
	if _, err := os.Stat(i.SteamCMDPath); err == nil {
		log.Println("[install] SteamCMD 已存在，跳过下载")
		return nil
	}

	// 下载 SteamCMD Linux (aarch64 兼容 x86 版本)
	// Valve 官方提供 steamcmd.zip，在 aarch64 上也可运行
	tarPath := filepath.Join(steamDir, "steamcmd.zip")
	log.Println("[install] 下载 SteamCMD...")

	dlCmd := exec.Command("bash", "-c",
		fmt.Sprintf("curl -sL -m 30 -o %s https://cdn.cloudflare.steamstatic.com/steamcmd/linux/steamcmd.zip", tarPath))
	if err := dlCmd.Run(); err != nil {
		return fmt.Errorf("下载 steamcmd 失败: %w", err)
	}

	log.Println("[install] 解压 SteamCMD...")
	extractCmd := exec.Command("unzip", "-q", tarPath, "-d", steamDir)
	if err := extractCmd.Run(); err != nil {
		return fmt.Errorf("解压 steamcmd 失败: %w", err)
	}

	// 清理
	_ = os.Remove(tarPath)
	log.Println("[install] SteamCMD 安装完成")

	// 给脚本可执行权限
	_ = os.Chmod(i.SteamCMDPath, 0755)
	return nil
}

// InstallDSTServer 通过 SteamCMD 安装 DST Dedicated Server
// AppID 108730 = Don't Starve Together
func (i *SteamCMDInstaller) InstallDSTServer() error {
	if _, err := os.Stat(i.SteamCMDPath); err != nil {
		return fmt.Errorf("SteamCMD 未安装，请先调用 InstallSteamCMD: %w", err)
	}

	// 检查是否已安装
	bootPath := filepath.Join(i.GameDir, "bin", "linux64", "dedicated_server")
	if _, err := os.Stat(bootPath); err == nil {
		log.Println("[install] DST Dedicated Server 已安装")
		return nil
	}

	log.Println("[install] 安装 DST Dedicated Server...")

	cmd := exec.Command(
		i.SteamCMDPath,
		"+force_install_dir", i.GameDir,
		"+app_update", "108730", "-beta", "public", "validate",
		"+quit",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// 设置必要的 SteamCMD 环境变量（匿名录入）
	cmd.Env = append(os.Environ(),
		"STEAMCMD_APPID=108730",
		"STEAMCMD_MEDIA_PATH="+i.GameDir,
	)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("安装 DST Dedicated Server 失败: %w", err)
	}

	log.Println("[install] DST Dedicated Server 安装完成")
	return nil
}

// UpdateDSTServer 增量更新游戏
func (i *SteamCMDInstaller) UpdateDSTServer() error {
	log.Println("[install] 检查并更新 DST Dedicated Server...")
	cmd := exec.Command(
		i.SteamCMDPath,
		"+force_install_dir", i.GameDir,
		"+app_update", "108730", "validate",
		"+quit",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Verify 验证安装是否完整
func (i *SteamCMDInstaller) Verify() error {
	bootPath := filepath.Join(i.GameDir, "bin", "linux64", "dedicated_server")
	if _, err := os.Stat(bootPath); err != nil {
		return fmt.Errorf("DST Dedicated Server 未安装: %w", err)
	}
	log.Println("[install] 验证通过: DST Dedicated Server 已就绪")
	return nil
}
