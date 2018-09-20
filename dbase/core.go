package dbase

import (
	"database/sql"
	"github.com/slevchyk/taskeram/dbase/sqlite"
	"github.com/slevchyk/taskeram/models"
)

var db *sql.DB
var dbType string

func ConnectDB(cfg models.Config) (*sql.DB, error) {

	var err error
	dbType = cfg.Database.Type

	switch dbType {
	case "sqlite":
		db, err = sqlite.ConntectDB(cfg)
	default:
		db, err = sqlite.ConntectDB(cfg)
	}

	return db, err
}

func InitDB(cfg models.Config) {

	switch dbType {
	case "sqlite":
		sqlite.InitDB(db, cfg)
	default:
		sqlite.InitDB(db, cfg)
	}
}