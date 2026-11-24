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
