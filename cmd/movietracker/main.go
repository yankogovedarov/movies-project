package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/yankogovedarov/movie-tracker/internal/db"
	"github.com/yankogovedarov/movie-tracker/internal/disk"
	"github.com/yankogovedarov/movie-tracker/internal/scanner"
	"github.com/yankogovedarov/movie-tracker/internal/web"
)

func main() {
	paths, err := disk.Discover(os.Executable)
	if err != nil {
		log.Fatalf("disk discovery failed: %v", err)
	}
	log.Printf("AppFolder: %s", paths.AppFolder)
	log.Printf("DiskRoot:  %s", paths.DiskRoot)

	dbPath := filepath.Join(paths.AppFolder, "movietracker.db")
	d, err := db.Open(dbPath)
	if err != nil {
		log.Fatalf("database open failed: %v", err)
	}
	defer d.Close()

	if err := db.Migrate(d); err != nil {
		log.Fatalf("database migrate failed: %v", err)
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
		log.Fatalf("disk scan failed: %v", err)
	}
	log.Printf("Found %d video files", len(files))

	r := gin.Default()
	r.GET("/", web.IndexHandler)
	r.Run(":8080")
}
