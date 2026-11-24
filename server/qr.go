package main

import (
	"fmt"
	"strings"
)

// Pure-Go Byte-mode QR Code SVG encoder supporting Versions 1..4 (EC Level L, up to 78 bytes).

var qrExp [512]byte
var qrLog [256]byte

func init() {
	x := 1
	for i := 0; i < 255; i++ {
		qrExp[i] = byte(x)
		qrLog[x] = byte(i)
		x <<= 1
		if x&0x100 != 0 {
			x ^= 0x11d
		}
	}
	for i := 255; i < 512; i++ {
		qrExp[i] = qrExp[i-255]
	}
}

func gfMul(a, b byte) byte {
	if a == 0 || b == 0 {
		return 0
	}
	return qrExp[int(qrLog[a])+int(qrLog[b])]
}

func rsGenerator(ecLen int) []byte {
	g := []byte{1}
	for i := 0; i < ecLen; i++ {
		next := make([]byte, len(g)+1)
		factor := qrExp[i]
		for j, coef := range g {
			next[j] ^= coef
			next[j+1] ^= gfMul(coef, factor)
		}
		g = next
	}
	return g
}

func rsRemainder(data []byte, ecLen int) []byte {
	gen := rsGenerator(ecLen)
	msg := make([]byte, len(data)+ecLen)
	copy(msg, data)
	for i := 0; i < len(data); i++ {
		lead := msg[i]
		if lead != 0 {
			for j := 0; j < len(gen); j++ {
				msg[i+j] ^= gfMul(gen[j], lead)
			}
		}
	}
	return msg[len(data):]
}

type qrVerSpec struct {
	version  int
	size     int
	dataCap  int
	ecLen    int
	alignPos int
}

var qrVersionsL = []qrVerSpec{
	{version: 1, size: 21, dataCap: 19, ecLen: 7, alignPos: 0},
	{version: 2, size: 25, dataCap: 34, ecLen: 10, alignPos: 18},
	{version: 3, size: 29, dataCap: 55, ecLen: 15, alignPos: 22},
	{version: 4, size: 33, dataCap: 80, ecLen: 20, alignPos: 26},
}

func RenderQRSvg(text string, pxSize int) string {
	raw := []byte(text)
	var spec qrVerSpec
	found := false
	for _, s := range qrVersionsL {
		// 4 bits mode (0100) + 8 bits len + 8*N data bits + 4 bits terminator => need 2 + len(raw) bytes max
		if len(raw)+2 <= s.dataCap {
			spec = s
			found = true
			break
		}
	}
	if !found {
		spec = qrVersionsL[len(qrVersionsL)-1]
		if len(raw) > spec.dataCap-2 {
			raw = raw[:spec.dataCap-2]
		}
	}

	// Build bit stream for data codewords (Byte mode)
	bits := make([]int, 0, spec.dataCap*8)
	pushBits := func(val, n int) {
		for i := n - 1; i >= 0; i-- {
			bits = append(bits, (val>>i)&1)
		}
	}
	pushBits(0b0100, 4) // Byte mode
	pushBits(len(raw), 8)
	for _, b := range raw {
		pushBits(int(b), 8)
	}
	rem := spec.dataCap*8 - len(bits)
	if rem > 4 {
		rem = 4
	}
	pushBits(0, rem)
	for len(bits)%8 != 0 {
		bits = append(bits, 0)
	}
	data := make([]byte, spec.dataCap)
	for i := 0; i < len(bits)/8; i++ {
		var b byte
		for k := 0; k < 8; k++ {
			b = (b << 1) | byte(bits[i*8+k])
		}
		data[i] = b
	}
	padBytes := []byte{0xEC, 0x11}
	for i, pIdx := len(bits)/8, 0; i < spec.dataCap; i, pIdx = i+1, pIdx+1 {
		data[i] = padBytes[pIdx%2]
	}

	ec := rsRemainder(data, spec.ecLen)
	codewords := append(data, ec...)

	n := spec.size
	modules := make([][]bool, n)
	reserved := make([][]bool, n)
	for i := range modules {
		modules[i] = make([]bool, n)
		reserved[i] = make([]bool, n)
	}
	setFunc := func(r, c int, dark bool) {
		if r >= 0 && r < n && c >= 0 && c < n {
			modules[r][c] = dark
			reserved[r][c] = true
		}
	}
	drawFinder := func(r0, c0 int) {
		for dr := -1; dr <= 7; dr++ {
			for dc := -1; dc <= 7; dc++ {
				r, c := r0+dr, c0+dc
				if r < 0 || r >= n || c < 0 || c >= n {
					continue
				}
				dark := false
				if dr >= 0 && dr <= 6 && dc >= 0 && dc <= 6 {
					dark = dr == 0 || dr == 6 || dc == 0 || dc == 6 || (dr >= 2 && dr <= 4 && dc >= 2 && dc <= 4)
				}
				setFunc(r, c, dark)
			}
		}
	}
	drawFinder(0, 0)
	drawFinder(0, n-7)
	drawFinder(n-7, 0)

	if spec.alignPos > 0 {
		ap := spec.alignPos
		for dr := -2; dr <= 2; dr++ {
			for dc := -2; dc <= 2; dc++ {
				dark := dr == -2 || dr == 2 || dc == -2 || dc == 2 || (dr == 0 && dc == 0)
				setFunc(ap+dr, ap+dc, dark)
			}
		}
	}

	for i := 8; i < n-8; i++ {
		setFunc(6, i, i%2 == 0)
		setFunc(i, 6, i%2 == 0)
	}
	// Dark module + format reservation
	setFunc(4*spec.version+9, 8, true)
	for i := 0; i < 9; i++ {
		if i != 6 {
			reserved[8][i] = true
			reserved[i][8] = true
		}
	}
	for i := 0; i < 8; i++ {
		reserved[8][n-1-i] = true
		if i < 7 {
			reserved[n-1-i][8] = true
		}
	}

	// Place data bits in zig-zag order
	bitIdx := 0
	totalBits := len(codewords) * 8
	up := true
	for col := n - 1; col >= 1; col -= 2 {
		if col == 6 {
			col = 5
		}
		for step := 0; step < n; step++ {
			r := step
			if up {
				r = n - 1 - step
			}
			for dc := 0; dc < 2; dc++ {
				c := col - dc
				if !reserved[r][c] {
					if bitIdx < totalBits {
						b := (codewords[bitIdx/8] >> uint(7-(bitIdx%8))) & 1
						modules[r][c] = b == 1
						bitIdx++
					}
				}
			}
		}
		up = !up
	}

	applyMask := func(mask int, grid [][]bool) {
		for r := 0; r < n; r++ {
			for c := 0; c < n; c++ {
				grid[r][c] = modules[r][c]
				if !reserved[r][c] {
					inv := false
					switch mask {
					case 0:
						inv = (r+c)%2 == 0
					case 1:
						inv = r%2 == 0
					case 2:
						inv = c%3 == 0
					case 3:
						inv = (r+c)%3 == 0
					case 4:
						inv = (r/2+c/3)%2 == 0
					case 5:
						inv = (r*c)%2+(r*c)%3 == 0
					case 6:
						inv = ((r*c)%2+(r*c)%3)%2 == 0
					case 7:
						inv = ((r+c)%2+(r*c)%3)%2 == 0
					}
					if inv {
						grid[r][c] = !grid[r][c]
					}
				}
			}
		}
		// Format info for EC L (01) + mask
		fmtData := (0b01 << 3) | mask
		d := fmtData << 10
		for i := 14; i >= 10; i-- {
			if (d>>i)&1 != 0 {
				d ^= 0x537 << (i - 10)
			}
		}
		fmtBits := ((fmtData << 10) | d) ^ 0x5412
		for i := 0; i <= 5; i++ {
			grid[8][i] = (fmtBits>>(14-i))&1 != 0
		}
		grid[8][7] = (fmtBits>>8)&1 != 0
		grid[8][8] = (fmtBits>>7)&1 != 0
		grid[7][8] = (fmtBits>>6)&1 != 0
		for i := 0; i <= 5; i++ {
			grid[5-i][8] = (fmtBits>>i)&1 != 0
		}
		for i := 0; i < 8; i++ {
			grid[8][n-1-i] = (fmtBits>>i)&1 != 0
		}
		for i := 0; i < 7; i++ {
			grid[n-1-i][8] = (fmtBits>>(14-i))&1 != 0
		}
	}

	bestMask := 0
	bestPen := 1 << 30
	tmp := make([][]bool, n)
	for i := range tmp {
		tmp[i] = make([]bool, n)
	}
	for m := 0; m < 8; m++ {
		applyMask(m, tmp)
		pen := 0
		for r := 0; r < n; r++ {
			run := 1
			for c := 1; c < n; c++ {
				if tmp[r][c] == tmp[r][c-1] {
					run++
				} else {
					if run >= 5 {
						pen += 3 + (run - 5)
					}
					run = 1
				}
			}
			if run >= 5 {
				pen += 3 + (run - 5)
			}
		}
		for c := 0; c < n; c++ {
			run := 1
			for r := 1; r < n; r++ {
				if tmp[r][c] == tmp[r-1][c] {
					run++
				} else {
					if run >= 5 {
						pen += 3 + (run - 5)
					}
					run = 1
				}
			}
			if run >= 5 {
				pen += 3 + (run - 5)
			}
		}
		if pen < bestPen {
			bestPen = pen
			bestMask = m
		}
	}
	applyMask(bestMask, tmp)

	quiet := 2
	vb := n + quiet*2
	if pxSize <= 0 {
		pxSize = 220
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d" shape-rendering="crispEdges">`, pxSize, pxSize, vb, vb)
	fmt.Fprintf(&sb, `<rect width="%d" height="%d" fill="#ffffff"/>`, vb, vb)
	sb.WriteString(`<path fill="#1f2d24" d="`)
	for r := 0; r < n; r++ {
		for c := 0; c < n; c++ {
			if tmp[r][c] {
				fmt.Fprintf(&sb, "M%d %dh1v1h-1z", c+quiet, r+quiet)
			}
		}
	}
	sb.WriteString(`"/></svg>`)
	return sb.String()
}
