package dbase

import (
	"database/sql"
	"github.com/slevchyk/taskeram/dbase/sqlite"
	"github.com/slevchyk/taskeram/models"
)

func ConnectDB(cfg models.Config) (*sql.DB, error) {

	switch cfg.Database.Type {
	case "sqlite":
		return sqlite.ConntectDB(cfg)
	default:
		return sqlite.ConntectDB(cfg)
	}
}

func InitDB(cfg models.Config) {

	switch cfg.Database.Type {
	case "sqlite":
		sqlite.InitDB(cfg.DB, cfg)
	default:
		sqlite.InitDB(cfg.DB, cfg)
	}
}