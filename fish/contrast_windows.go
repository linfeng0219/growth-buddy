//go:build windows

package main

import (
	"unsafe"
)

type BITMAPINFOHEADER struct {
	BiSize          uint32
	BiWidth         int32
	BiHeight        int32
	BiPlanes        uint16
	BiBitCount      uint16
	BiCompression   uint32
	BiSizeImage     uint32
	BiXPelsPerMeter int32
	BiYPelsPerMeter int32
	BiClrUsed       uint32
	BiClrImportant  uint32
}

type BITMAPINFO struct {
	BmiHeader BITMAPINFOHEADER
	BmiColors [1]uint32
}

const biRGB = 0

// captureScreenRectBGRA captures pixels underneath a WS_EX_LAYERED window (omitting CAPTUREBLT) into top-down BGRA pixels.
func captureScreenRectBGRA(x, y, w, h int32) ([]uint32, bool) {
	if w <= 0 || h <= 0 || pGetDC.Find() != nil || pGetDIBits.Find() != nil {
		return nil, false
	}
	scrDC, _, _ := pGetDC.Call(0)
	if scrDC == 0 {
		return nil, false
	}
	defer pReleaseDC.Call(0, scrDC)

	memDC, _, _ := pCreateCompatibleDC.Call(scrDC)
	if memDC == 0 {
		return nil, false
	}
	defer pDeleteDC.Call(memDC)

	bmp, _, _ := pCreateCompatibleBitmap.Call(scrDC, uintptr(w), uintptr(h))
	if bmp == 0 {
		return nil, false
	}
	defer pDeleteObject.Call(bmp)

	oldBmp, _, _ := pSelectObject.Call(memDC, bmp)
	// Do NOT pass CAPTUREBLT: Windows DWM omits WS_EX_LAYERED windows (including fish itself) so we read what is behind fish.
	pBitBlt.Call(memDC, 0, 0, uintptr(w), uintptr(h), scrDC, uintptr(x), uintptr(y), SRCCOPY)
	pSelectObject.Call(memDC, oldBmp)

	buf := make([]uint32, int(w*h))
	var bi BITMAPINFO
	bi.BmiHeader.BiSize = uint32(unsafe.Sizeof(BITMAPINFOHEADER{}))
	bi.BmiHeader.BiWidth = w
	bi.BmiHeader.BiHeight = -h // negative height = top-down DIB
	bi.BmiHeader.BiPlanes = 1
	bi.BmiHeader.BiBitCount = 32
	bi.BmiHeader.BiCompression = biRGB

	rows, _, _ := pGetDIBits.Call(memDC, bmp, 0, uintptr(h), uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&bi)), 0)
	if rows == 0 {
		return nil, false
	}
	return buf, true
}

func computeBackdropSignature(buf []uint32, w, h int32) uint64 {
	if w <= 0 || h <= 0 || len(buf) < int(w*h) {
		return 0
	}
	var sig uint64 = 14695981039346656037
	stepY := h / 12
	if stepY < 8 {
		stepY = 8
	}
	stepX := w / 12
	if stepX < 8 {
		stepX = 8
	}
	for y := int32(0); y < h; y += stepY {
		row := int(y) * int(w)
		for x := int32(0); x < w; x += stepX {
			px := uint64(buf[row+int(x)] & 0x00F8F8F8) // quantize low 3 bits to ignore subpixel noise
			sig ^= px
			sig *= 1099511628211
		}
	}
	return sig
}
