//go:build windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"unsafe"
)

func TestWin32NotifyIconStructAndTrayIcon(t *testing.T) {
	if got := unsafe.Sizeof(NOTIFYICONDATAW{}); got != 976 {
		t.Fatalf("expected NOTIFYICONDATAW size 976 on amd64, got %d", got)
	}
	if got := unsafe.Sizeof(DRAWITEMSTRUCT{}); got != 64 {
		t.Fatalf("expected DRAWITEMSTRUCT size 64 on amd64, got %d", got)
	}
	icon := createFishTrayIcon(false)
	if icon == 0 {
		t.Fatalf("createFishTrayIcon(false) returned 0")
	}
	pDestroyIcon.Call(icon)
	iconUnread := createFishTrayIcon(true)
	if iconUnread == 0 {
		t.Fatalf("createFishTrayIcon(true) returned 0")
	}
	pDestroyIcon.Call(iconUnread)
}

func TestMenuFlagsAndDropperTipBounds(t *testing.T) {
	if menuFlags(true) != (MF_STRING | MF_CHECKED) {
		t.Fatalf("expected MF_CHECKED when true")
	}
	if menuFlags(false) != MF_STRING {
		t.Fatalf("expected MF_STRING when false")
	}
	st := &eyedropperState{vx: 0, vy: 0, vw: 1920, vh: 1080}
	tx, ty := st.calcTipPos(1900, 1060, 188, 48)
	if tx >= 1900 || ty >= 1060 || tx < 0 || ty < 0 {
		t.Fatalf("expected tip to flip inside bottom-right screen edge, got (%d,%d)", tx, ty)
	}
}

func TestSelfReplaceUpdaterWaitAndUnicodePath(t *testing.T) {
	// Spawn a short-lived dummy process so launchSelfReplaceUpdater exercises WaitForExit + Copy + Delete on Unicode path
	sleeper := exec.Command("cmd.exe", "/C", "ping -n 2 127.0.0.1 >nul")
	if err := sleeper.Start(); err != nil {
		t.Fatalf("start dummy process: %v", err)
	}

	dir := filepath.Join(t.TempDir(), "测试 目录")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	src := filepath.Join(t.TempDir(), "fish-update-stage.exe")
	dst := filepath.Join(dir, "fish.exe")
	if err := os.WriteFile(dst, []byte("old-binary"), 0o644); err != nil {
		t.Fatalf("write old dst: %v", err)
	}
	if err := os.WriteFile(src, []byte("new-binary-v2"), 0o644); err != nil {
		t.Fatalf("write new src: %v", err)
	}

	updCmd, err := launchSelfReplaceUpdater(sleeper.Process.Pid, src, dst, false)
	if err != nil {
		t.Fatalf("launchSelfReplaceUpdater failed: %v", err)
	}
	_ = sleeper.Wait()
	if err := updCmd.Wait(); err != nil {
		t.Fatalf("updater command failed: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil || string(got) != "new-binary-v2" {
		t.Fatalf("expected dst to hold new-binary-v2, got %q err=%v", string(got), err)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatalf("expected staged src file to be removed after update")
	}
}

func TestCaptureScreenRectBGRA(t *testing.T) {
	pix, ok := captureScreenRectBGRA(0, 0, 32, 24)
	if !ok || len(pix) != 32*24 {
		t.Fatalf("expected 32x24 screen capture buffer, ok=%v len=%d", ok, len(pix))
	}
}
