package database

import (
	"database/sql"
	"github.com/slevchyk/taskeram/database/sqlite"
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

func InitDB(db *sql.DB, cfg models.Config) {

	switch cfg.Database.Type {
	case "sqlite":
		sqlite.InitDB(db, cfg)
	default:
		sqlite.InitDB(db, cfg)
	}

}