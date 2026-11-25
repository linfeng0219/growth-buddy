package main

import (
	"fmt"
	"time"
)

func formatTrayUnreadTip(count int) string {
	if count < 0 {
		count = 0
	}
	return fmt.Sprintf("未读信息%d条", count)
}

// chromaKeyRef 是透明背景模式下用于抠图的色键 RGB(1, 2, 3)。
const chromaKeyRef uint32 = 0x00030201

const conversationGapMs int64 = 3 * 60 * 1000 // 前后消息间隔小于 3 分钟归为同一次对话

func shouldShowConversationTimestamp(prevCreatedAt, curCreatedAt int64, isFirst bool) bool {
	if curCreatedAt <= 0 {
		return false
	}
	if isFirst || prevCreatedAt <= 0 {
		return true
	}
	return curCreatedAt-prevCreatedAt >= conversationGapMs
}

func formatConversationTime(createdAtMs int64, now time.Time) string {
	if createdAtMs <= 0 {
		return ""
	}
	t := time.UnixMilli(createdAtMs).In(now.Location())
	ny, nm, nd := now.Date()
	ty, tm, td := t.Date()
	if ty == ny && tm == nm && td == nd {
		return t.Format("15:04")
	}
	yest := now.AddDate(0, 0, -1)
	yy, ym, yd := yest.Date()
	if ty == yy && tm == ym && td == yd {
		return "昨天 " + t.Format("15:04")
	}
	if ty == ny {
		return t.Format("01-02 15:04")
	}
	return t.Format("2006-01-02 15:04")
}

func isDarkColor(c uint32) bool {
	r := int(c & 0xFF)
	g := int((c >> 8) & 0xFF)
	b := int((c >> 16) & 0xFF)
	return (r*299 + g*587 + b*114) < 128000
}

func colorLuminance(c uint32) int {
	r := int(c & 0xFF)
	g := int((c >> 8) & 0xFF)
	b := int((c >> 16) & 0xFF)
	return (r*299 + g*587 + b*114) / 1000
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// invertTextColorForBg 计算与背景 COLORREF (0x00BBGGRR) 相反的高对比度文字颜色。
func invertTextColorForBg(bg uint32) uint32 {
	r := int(bg & 0xFF)
	g := int((bg >> 8) & 0xFF)
	b := int((bg >> 16) & 0xFF)

	ir := 255 - r
	ig := 255 - g
	ib := 255 - b
	inv := uint32((ib << 16) | (ig << 8) | ir)

	// 灰阶中值区（如 #808080）直接取反会导致灰字配灰底看不清，此时顺向拉开对比度。
	bgLum := colorLuminance(bg)
	if absInt(bgLum-colorLuminance(inv)) < 85 {
		if bgLum < 128 {
			inv = 0x00FFFFFF
		} else {
			inv = 0x00101010
		}
	}

	// 避免刚好命中抠图色键 RGB(1,2,3) 导致文字被透明打洞
	if inv == chromaKeyRef {
		return 0x00000000
	}
	return inv
}

func colorChannelMaxDiff(a, b uint32) int {
	dr := absInt(int(a&0xFF) - int(b&0xFF))
	dg := absInt(int((a>>8)&0xFF) - int((b>>8)&0xFF))
	db := absInt(int((a>>16)&0xFF) - int((b>>16)&0xFF))
	m := dr
	if dg > m {
		m = dg
	}
	if db > m {
		m = db
	}
	return m
}

// invertTextColorStable 在 invertTextColorForBg 基础上增加色差死区与中灰滞回区间，
// 防止透明背景采样轻微抖动或桌面亮度接近 128 时反显文字反复黑白闪烁。
func invertTextColorStable(bg, prevBg, prevFg uint32, hasPrev bool) (uint32, uint32) {
	if hasPrev && prevFg != 0 && colorChannelMaxDiff(bg, prevBg) <= 14 {
		return prevFg, prevBg
	}
	bgLum := colorLuminance(bg)
	inv := invertTextColorForBg(bg)
	if hasPrev && prevFg != 0 {
		prevIsLight := colorLuminance(prevFg) >= 128
		// 中灰过渡区 [104, 152] 内维持当前明暗极性，避免在亮度 128 临界点来回跳变
		if bgLum >= 104 && bgLum <= 152 {
			if absInt(bgLum-colorLuminance(inv)) < 85 || (colorLuminance(inv) >= 128) != prevIsLight {
				if prevIsLight {
					inv = 0x00FFFFFF
				} else {
					inv = 0x00101010
				}
			}
		}
		if inv == prevFg {
			return prevFg, bg
		}
	}
	return inv, bg
}

// brightenColor 返回比指定 COLORREF (0x00BBGGRR) 稍亮一点的颜色（用于输入框外框）。
func brightenColor(c uint32, delta int) uint32 {
	if delta < 0 {
		delta = 0
	}
	clamp := func(v int) int {
		if v > 255 {
			return 255
		}
		if v < 0 {
			return 0
		}
		return v
	}
	r := clamp(int(c&0xFF) + delta)
	g := clamp(int((c>>8)&0xFF) + delta)
	b := clamp(int((c>>16)&0xFF) + delta)
	out := uint32((b << 16) | (g << 8) | r)
	if out == chromaKeyRef {
		return 0x001E1E1E
	}
	return out
}

// darkenColorForButton 计算较暗的按钮底色，确保与亮色文字搭配清晰且不刺眼。
func darkenColorForButton(bg uint32, pressed bool) uint32 {
	r := int(bg & 0xFF)
	g := int((bg >> 8) & 0xFF)
	b := int((bg >> 16) & 0xFF)
	if !isDarkColor(bg) {
		r = r/5 + 6
		g = g/5 + 8
		b = b/5 + 6
	} else {
		r = r - 18
		g = g - 18
		b = b - 18
	}
	if pressed {
		r -= 10
		g -= 10
		b -= 10
	}
	clampMin := func(v int) int {
		if v < 18 {
			return 18
		}
		if v > 72 {
			return 72
		}
		return v
	}
	out := uint32((clampMin(b) << 16) | (clampMin(g) << 8) | clampMin(r))
	if out == chromaKeyRef {
		return 0x001E2420
	}
	return out
}

// averageBGRAStripColor 计算 top-down 32bpp BGRA 像素缓冲区指定行区间的平均色（返回 COLORREF 0x00BBGGRR）。
func averageBGRAStripColor(bgra []uint32, w, h, yStart, yEnd int32) (uint32, bool) {
	if w <= 0 || h <= 0 || len(bgra) < int(w*h) {
		return 0, false
	}
	if yStart < 0 {
		yStart = 0
	}
	if yEnd > h {
		yEnd = h
	}
	if yEnd <= yStart {
		return 0, false
	}

	stepX := int32(3)
	if w < 12 {
		stepX = 1
	}
	stepY := int32(2)
	if yEnd-yStart < 8 {
		stepY = 1
	}

	var sumR, sumG, sumB, count uint64
	for y := yStart; y < yEnd; y += stepY {
		rowOff := int(y) * int(w)
		for x := int32(0); x < w; x += stepX {
			px := bgra[rowOff+int(x)]
			sumB += uint64(px & 0xFF)
			sumG += uint64((px >> 8) & 0xFF)
			sumR += uint64((px >> 16) & 0xFF)
			count++
		}
	}
	if count == 0 {
		return 0, false
	}
	r := uint32(sumR / count)
	g := uint32(sumG / count)
	b := uint32(sumB / count)
	return (b << 16) | (g << 8) | r, true
}
