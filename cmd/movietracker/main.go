package main

import (
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yankogovedarov/movie-tracker/internal/config"
	"github.com/yankogovedarov/movie-tracker/internal/db"
	"github.com/yankogovedarov/movie-tracker/internal/disk"
	"github.com/yankogovedarov/movie-tracker/internal/scanner"
	"github.com/yankogovedarov/movie-tracker/internal/vlc"
	"github.com/yankogovedarov/movie-tracker/internal/web"
	"gopkg.in/natefinch/lumberjack.v2"
)

// version and buildNumber are injected at build time via -ldflags -X.
var (
	version     = "dev"
	buildNumber = "0"
)

func main() {
	for _, arg := range os.Args[1:] {
		if arg == "--version" || arg == "-v" || arg == "version" {
			fmt.Printf("Movie Tracker %s (build %s)\n", version, buildNumber)
			return
		}
	}

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
	slog.Info("Movie Tracker", "version", version, "build", buildNumber)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config load failed", "err", err)
		os.Exit(1)
	}

	// Bug 34: take the port before touching the database. A second instance
	// started while the first one is still scanning or syncing exits here,
	// before it rotates a backup or writes the same database, and a busy port
	// stops the program with a log line instead of failing silently. Browser
	// connections wait in the listen backlog until the server starts serving
	// after the sync.
	const addr, siteURL = ":8080", "http://localhost:8080"
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		slog.Error("http listen failed", "addr", addr, "err", err)
		os.Exit(1)
	}
	slog.Info("http port acquired", "addr", addr)

	vlcPath, err := vlc.DetectDefault(cfg.VLCPath)
	if err != nil {
		slog.Warn("VLC not found", "err", err)
	} else {
		slog.Info("VLC found", "path", vlcPath)
	}

	dbPath := filepath.Join(paths.AppFolder, "movietracker.db")

	// Bug 34: checked before the backup, which copies only the .db file.
	if size, found := db.LeftoverJournal(dbPath); found {
		slog.Warn("leftover database journal found: the previous run was interrupted during a write, or another instance is writing",
			"path", dbPath+"-journal", "size", size,
			"note", "SQLite rolls the journal back when the database is opened; the backup of this start may be incomplete")
	}

	// Bug 34: every startup step logs how long it took, so a slow external
	// disk shows up in the log instead of a silent hang.
	start := time.Now()
	backupPath, err := db.Backup(dbPath, time.Now(), db.BackupKeep)
	if err != nil {
		slog.Error("database backup failed", "err", err)
		os.Exit(1)
	}
	if backupPath != "" {
		slog.Info("database backup created", "path", backupPath, "took", time.Since(start))
	} else {
		slog.Info("no database to back up (first run)")
	}

	start = time.Now()
	d, err := db.Open(dbPath)
	if err != nil {
		slog.Error("database open failed", "err", err)
		os.Exit(1)
	}
	defer d.Close()
	slog.Info("database opened", "path", dbPath, "took", time.Since(start))

	start = time.Now()
	if err := db.Migrate(d); err != nil {
		slog.Error("database migrate failed", "err", err)
		os.Exit(1)
	}
	slog.Info("database migrated", "took", time.Since(start))

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
	start = time.Now()
	files, err := scanner.Scan(paths.DiskRoot, excludedDirs)
	if err != nil {
		slog.Error("disk scan failed", "err", err)
		os.Exit(1)
	}
	slog.Info("scan complete", "files", len(files), "took", time.Since(start))

	if err = db.SyncScanResults(d, files, logger); err != nil {
		slog.Error("sync scan results failed", "err", err)
		os.Exit(1)
	}

	// Bug 24: MOVIETRACKER_NO_LAUNCH=1 keeps the binary from opening VLC,
	// Explorer or the browser, so the e2e suite leaves no windows behind.
	noLaunch := web.NoLaunch()
	launcher := web.Launcher(web.ExecLauncher)
	if noLaunch {
		launcher = web.NoopLauncher
		logger.Info("external program launching disabled (MOVIETRACKER_NO_LAUNCH=1)")
	}

	h := &web.Handlers{DB: d, DiskRoot: paths.DiskRoot, VLCPath: vlcPath, Log: logger, Launcher: launcher}
	r := gin.Default()
	r.GET("/", h.Index)
	r.GET("/tree", h.Tree)
	r.GET("/media/:id/open-folder", h.OpenFolder)
	r.POST("/media/:id/start", h.StartMedia)
	r.POST("/media/:id/status", h.ChangeStatus)
	r.POST("/media/random-new/start", h.RandomNew)
	r.POST("/media/:id/for-dnd", h.SetForDnd)
	r.POST("/media/:id/for-deletion", h.SetForDeletion)
	r.POST("/media/:id/wrong-subs", h.SetWrongSubs)
	r.POST("/scan", h.Scan)
	r.GET("/static/*filepath", h.StaticFile)

	// Bug 34: the browser opens only once the server is about to serve.
	slog.Info("http server listening", "url", siteURL)
	if !noLaunch {
		go openBrowser(siteURL)
	}
	if err := r.RunListener(ln); err != nil {
		slog.Error("http server failed", "err", err)
		os.Exit(1)
	}
}

func openBrowser(url string) {
	if err := exec.Command("cmd", "/c", "start", url).Start(); err != nil {
		slog.Warn("browser open failed", "url", url, "err", err)
		return
	}
	slog.Info("browser launch requested", "url", url)
}
