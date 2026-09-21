package chat

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/davidmuller5273-boop/safe/internal/config"
)

func Run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	r := gin.Default()
	r.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok", "service": "chat"}) })
	return r.Run(fmt.Sprintf(":%d", cfg.ChatPort))
}
