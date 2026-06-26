package server

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
)

func TestProvideHTTPServerAppliesGlobalRequestBodyLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.POST("/upload", func(c *gin.Context) {
		_, err := io.ReadAll(c.Request.Body)
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			c.Status(http.StatusRequestEntityTooLarge)
			return
		}
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusOK)
	})

	srv := ProvideHTTPServer(&config.Config{
		Server: config.ServerConfig{
			Host:               "127.0.0.1",
			Port:               0,
			MaxRequestBodySize: 4,
		},
		Gateway: config.GatewayConfig{
			MaxBodySize: 1024,
		},
	}, router)

	req := httptest.NewRequest(http.MethodPost, "/upload", strings.NewReader("12345"))
	rec := httptest.NewRecorder()

	srv.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}
}
