//go:build windows

package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"unicode/utf16"
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

func encodePowerShellUTF16(script string) string {
	u := utf16.Encode([]rune(script))
	b := make([]byte, len(u)*2)
	for i, v := range u {
		b[i*2] = byte(v)
		b[i*2+1] = byte(v >> 8)
	}
	return base64.StdEncoding.EncodeToString(b)
}

func launchSelfReplaceUpdater(pid int, stagePath, targetExe string, relaunch bool) (*exec.Cmd, error) {
	relaunchStmt := ""
	if relaunch {
		relaunchStmt = "\nStart-Process -FilePath $dst -WorkingDirectory (Split-Path -Parent $dst)"
	}
	psScript := fmt.Sprintf(`$ErrorActionPreference = 'Stop'
$src = $env:FISH_UPD_SRC
$dst = $env:FISH_UPD_DST
$p = Get-Process -Id %d -ErrorAction SilentlyContinue
if ($p) { $p.WaitForExit(15000) | Out-Null }
Start-Sleep -Milliseconds 150
for ($i = 0; $i -lt 30; $i++) {
  try {
    Copy-Item -LiteralPath $src -Destination $dst -Force
    Remove-Item -LiteralPath $src -Force -ErrorAction SilentlyContinue
    break
  } catch {
    if ($i -eq 29) { throw }
    Start-Sleep -Milliseconds 250
  }
}%s`, pid, relaunchStmt)

	psExe := filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	if _, err := os.Stat(psExe); err != nil {
		psExe = "powershell.exe"
	}

	cmd := exec.Command(
		psExe,
		"-NoProfile",
		"-NonInteractive",
		"-WindowStyle", "Hidden",
		"-EncodedCommand", encodePowerShellUTF16(psScript),
	)
	cmd.Env = append(os.Environ(),
		"FISH_UPD_SRC="+stagePath,
		"FISH_UPD_DST="+targetExe,
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

func downloadAndApplyFishUpdate(serverURL string, rel FishReleaseDTO) error {
	exe, err := os.Executable()
	if err != nil || exe == "" {
		return errors.New("无法定位当前程序路径")
	}
	stagePath := filepath.Join(os.TempDir(), fmt.Sprintf("fish-update-%d.exe", os.Getpid()))
	if err := downloadFishUpdateToFile(serverURL, rel, stagePath); err != nil {
		return err
	}
	if _, err := launchSelfReplaceUpdater(os.Getpid(), stagePath, exe, true); err != nil {
		_ = os.Remove(stagePath)
		return fmt.Errorf("启动自动更新进程失败: %w", err)
	}
	return nil
}
