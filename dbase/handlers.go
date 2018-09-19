package dbase

import (
	"database/sql"
	"github.com/slevchyk/taskeram/dbase/sqlite"
	"github.com/slevchyk/taskeram/models"
	"log"
)

func GetUserByTelegramID(cfg models.Config, tgid int) models.DbUsers {

	var (
		rows *sql.Rows
		u models.DbUsers
		err error
	)

	switch cfg.Database.Type {
	case "sqlite":
		rows, err = sqlite.SelectUsersByTelegramID(cfg.DB, tgid)
	default:
		rows, err = sqlite.SelectUsersByTelegramID(cfg.DB, tgid)
	}

	if err != nil {
		log.Fatal(err.Error())
	}

	if rows.Next() {
		ScanUser(rows, &u)
	}
	rows.Close()

	return u
}

func SelectUsersByTelegramID(cfg models.Config, tgid int) (*sql.Rows, error) {

	switch cfg.Database.Type {
	case "sqlite":
		return  sqlite.SelectUsersByTelegramID(cfg.DB, tgid)
	default:
		return  sqlite.SelectUsersByTelegramID(cfg.DB, tgid)
	}
}

func SelectUsersByStatus(cfg models.Config, status string) (*sql.Rows, error) {

	switch cfg.Database.Type {
	case "sqlite":
		return  sqlite.SelectUsersByStatus(cfg.DB, status)
	default:
		return  sqlite.SelectUsersByStatus(cfg.DB, status)
	}
}

func SelectUsersByTelegramIDStatus(cfg models.Config, tgid int, status string) (*sql.Rows, error) {

	switch cfg.Database.Type {
	case "sqlite":
		return  sqlite.SelectUsersByTelegramIDStatus(cfg.DB, tgid, status)
	default:
		return  sqlite.SelectUsersByTelegramIDStatus(cfg.DB, tgid, status)
	}
}

func SelectAdminUsers(cfg models.Config) (*sql.Rows, error) {

	switch cfg.Database.Type {
	case "sqlite":
		return  sqlite.SelectAdminUsers(cfg.DB)
	default:
		return  sqlite.SelectAdminUsers(cfg.DB)
	}
}

func SelectUsersForBan(cfg models.Config, tgid int) (*sql.Rows, error) {

	switch cfg.Database.Type {
	case "sqlite":
		return  sqlite.SelectUsersForBan(cfg.DB, tgid)
	default:
		return  sqlite.SelectUsersForBan(cfg.DB, tgid)
	}
}

func SelectUsersForUnban(cfg models.Config, tgid int) (*sql.Rows, error) {

	switch cfg.Database.Type {
	case "sqlite":
		return  sqlite.SelectUsersForUnban(cfg.DB, tgid)
	default:
		return  sqlite.SelectUsersForUnban(cfg.DB, tgid)
	}
}

func SelectTasksByID(cfg models.Config, taskID int) (*sql.Rows, error) {

	switch cfg.Database.Type {
	case "sqlite":
		return  sqlite.SelectTasksByID(cfg.DB, taskID)
	default:
		return  sqlite.SelectTasksByID(cfg.DB, taskID)
	}
}

func SelectTasksByIDUserTelegramID(cfg models.Config, taskID int, tgid int) (*sql.Rows, error) {

	switch cfg.Database.Type {
	case "sqlite":
		return  sqlite.SelectTasksByIDUserTelegramID(cfg.DB, taskID, tgid)
	default:
		return  sqlite.SelectTasksByIDUserTelegramID(cfg.DB, taskID, tgid)
	}
}

func SelectInboxTasks(cfg models.Config, tgid int, status string) (*sql.Rows, error) {

	switch cfg.Database.Type {
	case "sqlite":
		return  sqlite.SelectInboxTasks(cfg.DB, tgid, status)
	default:
		return  sqlite.SelectInboxTasks(cfg.DB, tgid, status)
	}
}

func SelectSentTasks(cfg models.Config, tgid int, status string) (*sql.Rows, error) {

	switch cfg.Database.Type {
	case "sqlite":
		return  sqlite.SelectSentTasks(cfg.DB, tgid, status)
	default:
		return  sqlite.SelectSentTasks(cfg.DB, tgid, status)
	}
}

func SelectHistory(cfg models.Config, taskID int, tgid int) (*sql.Rows, error) {

	switch cfg.Database.Type {
	case "sqlite":
		return  sqlite.SelectHistory(cfg.DB, taskID, tgid)
	default:
		return  sqlite.SelectHistory(cfg.DB, taskID, tgid)
	}
}

func SelectComments(cfg models.Config, taskID int, tgid int) (*sql.Rows, error) {

	switch cfg.Database.Type {
	case "sqlite":
		return  sqlite.SelectComments(cfg.DB, taskID, tgid)
	default:
		return  sqlite.SelectComments(cfg.DB, taskID, tgid)
	}
}

func SelectAuthByToken(cfg models.Config, token string) (*sql.Rows, error) {

	switch cfg.Database.Type {
	case "sqlite":
		return  sqlite.SelectAuthByToken(cfg.DB, token)
	default:
		return  sqlite.SelectAuthByToken(cfg.DB, token)
	}
}

func SelectSessions(cfg models.Config) (*sql.Rows, error) {

	switch cfg.Database.Type {
	case "sqlite":
		return  sqlite.SelectSessions(cfg.DB)
	default:
		return  sqlite.SelectSessions(cfg.DB)
	}
}

func SelectUsersBySessionUUID(cfg models.Config, uuid string) (*sql.Rows, error) {

	switch cfg.Database.Type {
	case "sqlite":
		return  sqlite.SelectUsersBySessionUUID(cfg.DB, uuid)
	default:
		return  sqlite.SelectUsersBySessionUUID(cfg.DB, uuid)
	}
}



//UpdateUserStatus - for changing user status. Uses 4 params
//1. New user status
//2. When status was changed
//3. Who changed user status
//4. User Telegram ID
func UpdateUserStatus(cfg models.Config) (*sql.Stmt, error) {

	switch cfg.Database.Type {
	case "sqlite":
		return  sqlite.UpdateUserStatus(cfg.DB)
	default:
		return  sqlite.UpdateUserStatus(cfg.DB)
	}
}

//UpdateTaskStatus is for changing task status. Uses 4 params
//1. New status
//2. When status was add
//3. Who change status
//4. Task ID
func UpdateTaskStatus(cfg models.Config) (*sql.Stmt, error) {

	switch cfg.Database.Type {
	case "sqlite":
		return  sqlite.UpdateTaskStatus(cfg.DB)
	default:
		return  sqlite.UpdateTaskStatus(cfg.DB)
	}
}

//UpdateTaskComment is for add new comment to task. Uses 4 params
//1. New comment
//2. When comment was add
//3. Who add comment
//4. Task ID
func UpdateTaskComment(cfg models.Config) (*sql.Stmt, error) {

	switch cfg.Database.Type {
	case "sqlite":
		return  sqlite.UpdateTaskComment(cfg.DB)
	default:
		return  sqlite.UpdateTaskComment(cfg.DB)
	}
}

//UpdateAuth is for add new comment to task. Uses 4 params
//1. Approved (0 - false, 1 - true)
//2. Token
func UpdateAuth(cfg models.Config) (*sql.Stmt, error) {

	switch cfg.Database.Type {
	case "sqlite":
		return  sqlite.UpdateAuth(cfg.DB)
	default:
		return  sqlite.UpdateAuth(cfg.DB)
	}
}

//UpdateAuth is for add new comment to task. Uses 4 params
//1. Last activity (time.Time)
//2. session id
func UpdateSessionLastActivityByUuid(cfg models.Config) (*sql.Stmt, error) {

	switch cfg.Database.Type {
	case "sqlite":
		return  sqlite.UpdateSessionLastActivityByUuid(cfg.DB)
	default:
		return  sqlite.UpdateSessionLastActivityByUuid(cfg.DB)
	}
}

func InsertUser(cfg models.Config) (*sql.Stmt, error) {

	switch cfg.Database.Type {
	case "sqlite":
		return  sqlite.InsertUser(cfg.DB)
	default:
		return  sqlite.InsertUser(cfg.DB)
	}
}

func InsertTask(cfg models.Config) (*sql.Stmt, error) {

	switch cfg.Database.Type {
	case "sqlite":
		return  sqlite.InsertTask(cfg.DB)
	default:
		return  sqlite.InsertTask(cfg.DB)
	}
}

func InsertAuth(cfg models.Config) (*sql.Stmt, error) {

	switch cfg.Database.Type {
	case "sqlite":
		return  sqlite.InsertAuth(cfg.DB)
	default:
		return  sqlite.InsertAuth(cfg.DB)
	}
}

func InsertSession(cfg models.Config) (*sql.Stmt, error) {

	switch cfg.Database.Type {
	case "sqlite":
		return  sqlite.InsertSession(cfg.DB)
	default:
		return  sqlite.InsertSession(cfg.DB)
	}
}

func DeleteAuthByToken(cfg models.Config) (*sql.Stmt, error) {

	switch cfg.Database.Type {
	case "sqlite":
		return  sqlite.DeleteAuthByToken(cfg.DB)
	default:
		return  sqlite.DeleteAuthByToken(cfg.DB)
	}
}

func DeleteSessionByUUID(cfg models.Config) (*sql.Stmt, error) {

	switch cfg.Database.Type {
	case "sqlite":
		return  sqlite.DeleteSessionByUUID(cfg.DB)
	default:
		return  sqlite.DeleteSessionByUUID(cfg.DB)
	}
}