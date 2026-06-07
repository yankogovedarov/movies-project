package main

import (
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/yankogovedarov/movie-tracker/internal/config"
	"github.com/yankogovedarov/movie-tracker/internal/db"
	"github.com/yankogovedarov/movie-tracker/internal/disk"
	"github.com/yankogovedarov/movie-tracker/internal/scanner"
	"github.com/yankogovedarov/movie-tracker/internal/vlc"
	"github.com/yankogovedarov/movie-tracker/internal/web"
	"gopkg.in/natefinch/lumberjack.v2"
)

func main() {
	paths, err := disk.Discover(os.Executable)
	if err != nil {
		slog.Error("disk discovery failed", "err", err)
		os.Exit(1)
	}

	if err := os.MkdirAll(filepath.Join(paths.AppFolder, "logs"), 0755); err != nil {
		slog.Error("failed to create logs dir", "err", err)
		os.Exit(1)
	}

	logFile := &lumberjack.Logger{
		Filename:   filepath.Join(paths.AppFolder, "logs", "app.log"),
		MaxSize:    10,
		MaxBackups: 3,
		MaxAge:     90,
		Compress:   true,
	}
	logger := slog.New(slog.NewTextHandler(
		io.MultiWriter(os.Stdout, logFile),
		&slog.HandlerOptions{Level: slog.LevelInfo},
	))
	slog.SetDefault(logger)

	slog.Info("starting", "appFolder", paths.AppFolder, "diskRoot", paths.DiskRoot)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config load failed", "err", err)
		os.Exit(1)
	}
	vlcPath, err := vlc.DetectDefault(cfg.VLCPath)
	if err != nil {
		slog.Warn("VLC not found", "err", err)
	} else {
		slog.Info("VLC found", "path", vlcPath)
	}

	dbPath := filepath.Join(paths.AppFolder, "movietracker.db")
	d, err := db.Open(dbPath)
	if err != nil {
		slog.Error("database open failed", "err", err)
		os.Exit(1)
	}
	defer d.Close()

	if err := db.Migrate(d); err != nil {
		slog.Error("database migrate failed", "err", err)
		os.Exit(1)
	}

	excludedDirs := []string{
		"$RECYCLE.BIN",
		"System Volume Information",
		"found.000",
		"Books",
		"Download",
		"Games",
		"Install",
		"LizaWork",
		"Music",
		"Sub",
		"Tatko",
		"Temp",
		"zzz",
	}
	files, err := scanner.Scan(paths.DiskRoot, excludedDirs)
	if err != nil {
		slog.Error("disk scan failed", "err", err)
		os.Exit(1)
	}
	slog.Info("scan complete", "files", len(files))

	if err := db.SyncScanResults(d, files); err != nil {
		slog.Error("sync scan results failed", "err", err)
		os.Exit(1)
	}
	slog.Info("db synced", "files", len(files))

	h := &web.Handlers{DB: d, DiskRoot: paths.DiskRoot, VLCPath: vlcPath, Log: logger}
	r := gin.Default()
	r.GET("/", h.Index)
	r.GET("/tree", h.Tree)
	r.GET("/media/:id", h.MediaDetail)
	r.GET("/media/:id/open-folder", h.OpenFolder)
	r.POST("/media/:id/start", h.StartMedia)
	r.POST("/media/:id/status", h.ChangeStatus)
	r.POST("/media/random-new/start", h.RandomNew)
	r.POST("/media/:id/translation-type", h.SetTranslationType)
	r.POST("/scan", h.Scan)
	r.GET("/static/*filepath", h.StaticFile)

	go openBrowser("http://localhost:8080")
	r.Run(":8080")
}

func openBrowser(url string) {
	_ = exec.Command("cmd", "/c", "start", url).Start()
}
