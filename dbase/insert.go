package dbase

import (
	"database/sql"
	"github.com/slevchyk/taskeram/models"
)

func InsertUserExec(stmt *sql.Stmt, u models.DbUsers) (sql.Result, error) {

	return stmt.Exec(u.TelegramID, u.FirstName, u.LastName, u.Admin, u.Status, u.ChangedAt, u.ChangedBy, u.Comment)
}

func InsertTaskExec(stmt *sql.Stmt, t models.DbTasks) (sql.Result, error) {

	return stmt.Exec(t.FromUser, t.ToUser, t.Status, t.ChangedAt, t.ChangedBy, t.Title, t.Description, t.Comment, t.CommentedAt, t.CommentedBy, t.Images, t.Documents)
}