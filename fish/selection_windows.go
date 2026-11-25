//go:build windows

package main

import (
	"unicode/utf16"
	"unsafe"
)

func wrapTextWithFont(hdc, hFont uintptr, text string, maxWidth int32, paraIdx int, isTimestamp bool) []visualLine {
	if maxWidth < 20 {
		maxWidth = 20
	}
	u16s := utf16.Encode([]rune(text))
	if len(u16s) == 0 {
		return nil
	}
	oldFont, _, _ := pSelectObject.Call(hdc, hFont)
	defer pSelectObject.Call(hdc, oldFont)

	var out []visualLine
	offset := 0
	for offset < len(u16s) {
		rem := len(u16s) - offset
		dx := make([]int32, rem)
		var fit int32
		var sz SIZE
		ret, _, _ := pGetTextExtentExPointW.Call(
			hdc,
			uintptr(unsafe.Pointer(&u16s[offset])),
			uintptr(rem),
			uintptr(maxWidth),
			uintptr(unsafe.Pointer(&fit)),
			uintptr(unsafe.Pointer(&dx[0])),
			uintptr(unsafe.Pointer(&sz)),
		)
		if ret == 0 || fit <= 0 {
			fit = 1
		}
		if int(fit) > rem {
			fit = int32(rem)
		}
		// Do not split surrogate pairs at wrap boundary
		if int(fit) < rem && isHighSurrogate(u16s[offset+int(fit)-1]) && int(fit) > 1 {
			fit--
		}
		chunk := u16s[offset : offset+int(fit)]
		charX := make([]int32, len(chunk)+1)
		for k := 0; k < len(chunk); k++ {
			charX[k+1] = dx[k]
		}
		h := sz.Cy
		if h <= 0 {
			h = 16
		}
		out = append(out, visualLine{
			paraIdx:     paraIdx,
			isTimestamp: isTimestamp,
			u16:         chunk,
			charX:       charX,
			h:           h,
		})
		offset += int(fit)
	}
	return out
}

func drawVisualLine(memDC uintptr, ln visualLine, left, screenY, clipTop, clipBottom int32, fgCol uint32, hasSel bool, selLo, selHi textPos, lineIdx int) {
	n := len(ln.u16)
	if n == 0 || len(ln.charX) < n+1 {
		return
	}
	clipRc := RECT{
		Left:   left,
		Top:    clipTop,
		Right:  left + ln.charX[n] + 4,
		Bottom: clipBottom,
	}

	c0, c1 := 0, 0
	if hasSel && lineIdx >= selLo.line && lineIdx <= selHi.line {
		c0 = 0
		if lineIdx == selLo.line {
			c0 = selLo.col
		}
		c1 = n
		if lineIdx == selHi.line {
			c1 = selHi.col
		}
		if c0 < 0 {
			c0 = 0
		}
		if c1 > n {
			c1 = n
		}
	}

	if c1 <= c0 {
		pSetBkMode.Call(memDC, TRANSPARENT)
		pSetTextColor.Call(memDC, uintptr(fgCol))
		pExtTextOutW.Call(memDC, uintptr(left), uintptr(screenY), ETO_CLIPPED, uintptr(unsafe.Pointer(&clipRc)), uintptr(unsafe.Pointer(&ln.u16[0])), uintptr(n), 0)
		return
	}

	// Segment 1: before selection
	if c0 > 0 {
		pSetBkMode.Call(memDC, TRANSPARENT)
		pSetTextColor.Call(memDC, uintptr(fgCol))
		pExtTextOutW.Call(memDC, uintptr(left), uintptr(screenY), ETO_CLIPPED, uintptr(unsafe.Pointer(&clipRc)), uintptr(unsafe.Pointer(&ln.u16[0])), uintptr(c0), 0)
	}

	// Segment 2: selected text with crisp highlight background (non-chroma #0078D7 -> COLORREF 0x00D77800) & white text
	const selBgRef uint32 = 0x00D77800
	hlTop := screenY
	if hlTop < clipTop {
		hlTop = clipTop
	}
	hlBot := screenY + ln.h
	if hlBot > clipBottom {
		hlBot = clipBottom
	}
	selRc := RECT{
		Left:   left + ln.charX[c0],
		Top:    hlTop,
		Right:  left + ln.charX[c1],
		Bottom: hlBot,
	}
	if selRc.Right > selRc.Left && selRc.Bottom > selRc.Top {
		if br, _, _ := pCreateSolidBrush.Call(uintptr(selBgRef)); br != 0 {
			pFillRect.Call(memDC, uintptr(unsafe.Pointer(&selRc)), br)
			pDeleteObject.Call(br)
		}
	}
	pSetBkMode.Call(memDC, TRANSPARENT)
	pSetTextColor.Call(memDC, 0x00FFFFFF)
	pExtTextOutW.Call(memDC, uintptr(left+ln.charX[c0]), uintptr(screenY), ETO_CLIPPED, uintptr(unsafe.Pointer(&clipRc)), uintptr(unsafe.Pointer(&ln.u16[c0])), uintptr(c1-c0), 0)

	// Segment 3: after selection
	if c1 < n {
		pSetBkMode.Call(memDC, TRANSPARENT)
		pSetTextColor.Call(memDC, uintptr(fgCol))
		pExtTextOutW.Call(memDC, uintptr(left+ln.charX[c1]), uintptr(screenY), ETO_CLIPPED, uintptr(unsafe.Pointer(&clipRc)), uintptr(unsafe.Pointer(&ln.u16[c1])), uintptr(n-c1), 0)
	}
}

func copyUTF16ToClipboard(owner uintptr, u []uint16) bool {
	if len(u) == 0 {
		return false
	}
	withNul := make([]uint16, len(u)+1)
	copy(withNul, u)
	byteLen := len(withNul) * 2

	hMem, _, _ := pGlobalAlloc.Call(GMEM_MOVEABLE, uintptr(byteLen))
	if hMem == 0 {
		return false
	}
	ptr, _, _ := pGlobalLock.Call(hMem)
	if ptr == 0 {
		return false
	}
	dst := unsafe.Slice((*uint16)(unsafe.Pointer(ptr)), len(withNul))
	copy(dst, withNul)
	pGlobalUnlock.Call(hMem)

	opened, _, _ := pOpenClipboard.Call(owner)
	if opened == 0 {
		return false
	}
	defer pCloseClipboard.Call()
	pEmptyClipboard.Call()
	res, _, _ := pSetClipboardData.Call(CF_UNICODETEXT, hMem)
	return res != 0
}
