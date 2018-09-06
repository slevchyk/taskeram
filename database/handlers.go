package database

import (
	"database/sql"
	"github.com/slevchyk/taskeram/database/sqlite"
	"github.com/slevchyk/taskeram/models"
	"log"
)

func GetUserByTelegramID(userid int, db *sql.DB) models.DbUsers {

	switch cfg.Database.Type {
	case "sqlite":
		return sqlite.GetUserByTelegramID(userid, db)
	default:
		return sqlite.GetUserByTelegramID(userid, db)
	}
}
