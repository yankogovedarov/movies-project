package main

import (
	"github.com/gin-gonic/gin"
	"github.com/yankogovedarov/movie-tracker/internal/web"
)

func main() {
	r := gin.Default()
	r.GET("/", web.IndexHandler)
	r.Run(":8080")
}
