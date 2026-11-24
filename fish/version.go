package main

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// 构建时可通过 -ldflags "-X main.FishVersionName=2026.11.24-1620 -X main.FishVersionCode=611241620" 注入。
var (
	FishVersionName = ""
	FishVersionCode = ""
)

var resolvedFishVerCode int

func fishVersionCodeFromTime(t time.Time) int {
	loc := time.FixedZone("CST", 8*3600)
	lt := t.In(loc)
	return (lt.Year()-2020)*100_000_000 + int(lt.Month())*1_000_000 + lt.Day()*10_000 + lt.Hour()*100 + lt.Minute()
}

func autoFishVersionCode(vName string) int {
	loc := time.FixedZone("CST", 8*3600)
	for _, layout := range []string{"2006.01.02-1504", "2006-01-02 15:04", "200601021504"} {
		if t, err := time.ParseInLocation(layout, strings.TrimSpace(vName), loc); err == nil {
			return fishVersionCodeFromTime(t)
		}
	}
	return fishVersionCodeFromTime(time.Now())
}

func init() {
	if strings.TrimSpace(FishVersionName) == "" {
		loc := time.FixedZone("CST", 8*3600)
		t := time.Now().In(loc)
		if exe, err := os.Executable(); err == nil {
			if info, err := os.Stat(exe); err == nil {
				t = info.ModTime().In(loc)
			}
		}
		FishVersionName = t.Format("2006.01.02-1504")
	}
	code, _ := strconv.Atoi(strings.TrimSpace(FishVersionCode))
	if code <= 0 {
		code = autoFishVersionCode(FishVersionName)
	}
	resolvedFishVerCode = code
	if strings.TrimSpace(FishVersionCode) == "" {
		FishVersionCode = strconv.Itoa(code)
	}
}

func currentFishVersion() (string, int) {
	return FishVersionName, resolvedFishVerCode
}
