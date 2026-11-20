package main

import (
	"bufio"
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// loadDotEnv loads key=value pairs from .env if present (without overriding existing env vars).
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.Trim(strings.TrimSpace(v), `"'`)
		if k != "" && os.Getenv(k) == "" {
			_ = os.Setenv(k, v)
		}
	}
}

func main() {
	loadDotEnv(".env")

	host := os.Getenv("HOST")
	if host == "" {
		host = "0.0.0.0"
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./data/growth_buddy.db"
	}
	releasesDir := os.Getenv("RELEASES_DIR")
	if releasesDir == "" {
		releasesDir = filepath.Join(filepath.Dir(dbPath), "releases")
	}
	publicDir := os.Getenv("PUBLIC_DIR")

	store, err := NewStore(dbPath, releasesDir)
	if err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}
	defer store.Close()

	app := NewAppServer(store, publicDir)
	srv := &http.Server{
		Addr:              host + ":" + port,
		Handler:           app,
		ReadHeaderTimeout: 15 * time.Second,
		WriteTimeout:      5 * time.Minute, // 允许上传/下载大体积 APK
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Printf("Growth Buddy (Go + SQLite) listening on http://%s:%s (db=%s)", host, port, dbPath)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Println("收到退出信号，正在优雅关闭服务...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}
