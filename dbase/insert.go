package dbase

import (
	"database/sql"
	"github.com/slevchyk/taskeram/models"
)

func InsertUserExec(stmt *sql.Stmt, u models.DbUsers) (sql.Result, error) {

	return stmt.Exec(u.TelegramID, u.FirstName, u.LastName, u.Admin, u.Status, u.ChangedAt.Time, u.ChangedBy, u.Comment, u.Userpic, u.Password)
}

func InsertTaskExec(stmt *sql.Stmt, t models.DbTasks) (sql.Result, error) {

	return stmt.Exec(t.FromUser, t.ToUser, t.Status, t.ChangedAt.Time, t.ChangedBy, t.Title, t.Description, t.Comment, t.CommentedAt.Time, t.CommentedBy, t.Images, t.Documents)
}

func InsertAuthExec(stmt *sql.Stmt, a models.DbAuth) (sql.Result, error) {

	return stmt.Exec(a.Token, a.ExpiryDate.Time, a.TelegramID, a.Approved)
}

func InsertSessionExec(stmt *sql.Stmt, s models.DbSessions) (sql.Result, error) {

	return stmt.Exec(s.UUID, s.TelegramID, s.StartedAt.Time, s.LastActivity.Time, s.IP, s.UserAgent)
}