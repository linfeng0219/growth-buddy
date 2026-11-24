//go:build windows

package main

import (
	"testing"
	"unsafe"
)

func TestWin32NotifyIconStructAndTrayIcon(t *testing.T) {
	if got := unsafe.Sizeof(NOTIFYICONDATAW{}); got != 976 {
		t.Fatalf("expected NOTIFYICONDATAW size 976 on amd64, got %d", got)
	}
	icon := createFishTrayIcon()
	if icon == 0 {
		t.Fatalf("createFishTrayIcon returned 0")
	}
	pDestroyIcon.Call(icon)
}
