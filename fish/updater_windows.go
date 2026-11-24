//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

const createNoWindow = 0x08000000

func cleanupPendingUpdateFiles() {
	exe, err := os.Executable()
	if err != nil || exe == "" {
		return
	}
	_ = os.Remove(exe + ".new")
	_ = os.Remove(exe + ".old")
}

func quoteCmdArg(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

func downloadAndApplyFishUpdate(serverURL string, rel FishReleaseDTO) error {
	exe, err := os.Executable()
	if err != nil || exe == "" {
		return errors.New("无法定位当前程序路径")
	}
	stagePath := exe + ".new"
	if err := downloadFishUpdateToFile(serverURL, rel, stagePath); err != nil {
		return err
	}
	script := fmt.Sprintf(
		"ping -n 2 127.0.0.1 >nul & move /y %s %s >nul && start \"\" %s",
		quoteCmdArg(stagePath),
		quoteCmdArg(exe),
		quoteCmdArg(exe),
	)
	cmd := exec.Command("cmd.exe", "/C", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
	if err := cmd.Start(); err != nil {
		_ = os.Remove(stagePath)
		return fmt.Errorf("启动更新替换进程失败: %w", err)
	}
	return nil
}
