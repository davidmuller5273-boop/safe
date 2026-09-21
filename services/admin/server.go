package admin

import (
	"fmt"
	"github.com/davidmuller5273-boop/safe/internal/config"
	"github.com/davidmuller5273-boop/safe/internal/platform/database"
	"github.com/davidmuller5273-boop/safe/internal/service"
	httptransport "github.com/davidmuller5273-boop/safe/internal/transport/http"
	"time"
)

func Run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	db, err := database.Open(cfg.MySQLHost, cfg.MySQLPort, cfg.MySQLUser, cfg.MySQLPassword, cfg.MySQLDatabase)
	if err != nil {
		return err
	}
	auth := service.Auth{DB: db, Secret: []byte(cfg.JWTSecret), Expire: time.Duration(cfg.ExpireHours) * time.Hour}
	return httptransport.Router(db, auth).Run(fmt.Sprintf(":%d", cfg.AdminPort))
}
