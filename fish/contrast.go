package main

// chromaKeyRef 是透明背景模式下用于抠图的色键 RGB(1, 2, 3)。
const chromaKeyRef uint32 = 0x00030201

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
