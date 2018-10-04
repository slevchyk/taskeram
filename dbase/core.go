package dbase

import (
	"database/sql"
	"github.com/slevchyk/taskeram/dbase/postgres"
	"github.com/slevchyk/taskeram/dbase/sqlite"
	"github.com/slevchyk/taskeram/models"
)

var db *sql.DB
var dbType string

const (
	Sqlite   = "sqlite"
	Postgres = "postgres"
)

func ConnectDB(cfg models.Config) (*sql.DB, error) {

	var err error
	dbType = cfg.Database.Type

	switch dbType {
	case "sqlite":
		db, err = sqlite.ConntectDB(cfg)
	case "postgres":
		db, err = postgres.ConntectDB(cfg)
	default:
		db, err = sqlite.ConntectDB(cfg)
	}

	return db, err
}

func InitDB(cfg models.Config) {

	switch dbType {
	case "sqlite":
		sqlite.InitDB(db, cfg)
	case "postgres":
		postgres.InitDB(db, cfg)
	default:
		sqlite.InitDB(db, cfg)
	}
}