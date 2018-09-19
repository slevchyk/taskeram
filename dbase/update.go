package dbase

import (
	"database/sql"
	"github.com/slevchyk/taskeram/models"
)

func UpdateTaskStatusExec(stmt *sql.Stmt, t models.DbTasks) (sql.Result, error) {

	return stmt.Exec(t.Status, t.ChangedAt, t.ChangedBy, t.ID)
}
