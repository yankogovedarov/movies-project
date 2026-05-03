package web_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/yankogovedarov/movie-tracker/internal/web"
)

func TestIndexHandlerReturns200(t *testing.T) {
	router := gin.Default()
	router.GET("/", web.IndexHandler)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Movie Tracker")
}
