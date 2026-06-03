package app

import (
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/hjyoon/socketing-socket-server/internal/ws"
)

func NewRouter(cfg Config, service *ws.Service) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	origins := cfg.CORSOrigins
	if len(origins) == 0 {
		origins = []string{"*"}
	}
	r.Use(gin.Logger(), gin.Recovery(), cors.New(cors.Config{
		AllowOrigins: origins,
		AllowMethods: []string{"GET", "POST", "OPTIONS"},
		AllowHeaders: []string{"Origin", "Content-Length", "Content-Type", "Authorization"},
		MaxAge:       12 * time.Hour,
	}))
	r.GET("/liveness", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "The server is alive."})
	})
	r.GET("/readiness", func(c *gin.Context) {
		if err := service.Ready(c.Request.Context()); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "The server is ready."})
	})
	r.GET("/", service.HandleHTTP)
	return r
}
