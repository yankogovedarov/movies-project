package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"
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

	files, err := scanner.Scan(paths.DiskRoot)
	if err != nil {
		log.Fatalf("disk scan failed: %v", err)
	}
	log.Printf("Found %d video files", len(files))

	r := gin.Default()
	r.GET("/", web.IndexHandler)
	r.Run(":8080")
}
