package main

import (
	"sort"
	"unicode/utf16"
)

type textPos struct {
	line int
	col  int
}

type visualLine struct {
	paraIdx     int
	isTimestamp bool
	u16         []uint16
	charX       []int32 // len = len(u16)+1 cumulative X offsets from cr.Left
	y           int32   // virtual Y offset (0-based from top of chat content)
	h           int32
	fgColor     uint32
}

func cmpTextPos(a, b textPos) int {
	if a.line < b.line {
		return -1
	}
	if a.line > b.line {
		return 1
	}
	if a.col < b.col {
		return -1
	}
	if a.col > b.col {
		return 1
	}
	return 0
}

func normalizedSelection(anchor, focus textPos) (textPos, textPos, bool) {
	c := cmpTextPos(anchor, focus)
	if c == 0 {
		return anchor, focus, false
	}
	if c < 0 {
		return anchor, focus, true
	}
	return focus, anchor, true
}

func clampTextPos(lines []visualLine, p textPos) textPos {
	if len(lines) == 0 {
		return textPos{}
	}
	if p.line < 0 {
		return textPos{line: 0, col: 0}
	}
	if p.line >= len(lines) {
		last := len(lines) - 1
		return textPos{line: last, col: len(lines[last].u16)}
	}
	maxCol := len(lines[p.line].u16)
	if p.col < 0 {
		p.col = 0
	}
	if p.col > maxCol {
		p.col = maxCol
	}
	return p
}

// isLowSurrogate prevents placing selection boundaries in the middle of a UTF-16 surrogate pair.
func isLowSurrogate(u uint16) bool {
	return u >= 0xDC00 && u <= 0xDFFF
}

func isHighSurrogate(u uint16) bool {
	return u >= 0xD800 && u <= 0xDBFF
}

func hitTestChatLines(lines []visualLine, scrollY, crLeft, crTop, clientX, clientY int32) textPos {
	if len(lines) == 0 {
		return textPos{}
	}
	contentY := clientY - crTop + scrollY
	if contentY <= lines[0].y {
		return textPos{line: 0, col: hitTestLineCol(lines[0], clientX-crLeft)}
	}
	lastIdx := len(lines) - 1
	last := lines[lastIdx]
	if contentY >= last.y+last.h {
		return textPos{line: lastIdx, col: hitTestLineCol(last, clientX-crLeft)}
	}

	idx := sort.Search(len(lines), func(i int) bool {
		return lines[i].y+lines[i].h > contentY
	})
	if idx >= len(lines) {
		idx = lastIdx
	}
	// If contentY landed inside a vertical gap between idx-1 and idx, snap to the nearer line
	if idx > 0 && contentY < lines[idx].y {
		prev := lines[idx-1]
		if contentY-(prev.y+prev.h) < lines[idx].y-contentY {
			idx--
		}
	}
	return textPos{line: idx, col: hitTestLineCol(lines[idx], clientX-crLeft)}
}

func hitTestLineCol(ln visualLine, relX int32) int {
	n := len(ln.u16)
	if n == 0 || len(ln.charX) < n+1 || relX <= 0 {
		return 0
	}
	if relX >= ln.charX[n] {
		return n
	}
	for c := 0; c < n; c++ {
		mid := (ln.charX[c] + ln.charX[c+1]) / 2
		if relX < mid {
			if c > 0 && isLowSurrogate(ln.u16[c]) {
				return c - 1
			}
			return c
		}
	}
	return n
}

func paragraphBoundsAtLine(lines []visualLine, lineIdx int) (textPos, textPos, bool) {
	if lineIdx < 0 || lineIdx >= len(lines) {
		return textPos{}, textPos{}, false
	}
	targetPara := lines[lineIdx].paraIdx
	startLine := lineIdx
	for startLine > 0 && lines[startLine-1].paraIdx == targetPara {
		startLine--
	}
	endLine := lineIdx
	for endLine+1 < len(lines) && lines[endLine+1].paraIdx == targetPara {
		endLine++
	}
	return textPos{line: startLine, col: 0}, textPos{line: endLine, col: len(lines[endLine].u16)}, true
}

func extractSelectedUTF16(lines []visualLine, start, end textPos) []uint16 {
	s := clampTextPos(lines, start)
	e := clampTextPos(lines, end)
	lo, hi, ok := normalizedSelection(s, e)
	if !ok || len(lines) == 0 {
		return nil
	}

	var out []uint16
	prevPara := -1
	wroteAny := false
	for li := lo.line; li <= hi.line; li++ {
		ln := lines[li]
		c0 := 0
		if li == lo.line {
			c0 = lo.col
		}
		c1 := len(ln.u16)
		if li == hi.line {
			c1 = hi.col
		}
		if c0 > c1 {
			continue
		}
		// Only insert CRLF when moving to a different paragraph (wrapped lines of the same message join seamlessly)
		if wroteAny && ln.paraIdx != prevPara {
			out = append(out, '\r', '\n')
		}
		if c1 > c0 {
			out = append(out, ln.u16[c0:c1]...)
			wroteAny = true
			prevPara = ln.paraIdx
		} else if li > lo.line && ln.paraIdx != prevPara {
			prevPara = ln.paraIdx
		}
	}
	return out
}

func utf16ToString(u []uint16) string {
	return string(utf16.Decode(u))
}
