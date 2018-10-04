package dbase

import (
	"database/sql"
	"github.com/slevchyk/taskeram/models"
)

func ExecUpdateUserStatus(stmt *sql.Stmt, u models.DbUsers) (sql.Result, error) {

	return stmt.Exec(u.Status, u.ChangedAt, u.ChangedBy, u.TelegramID)

}

func ExecUpdateUserData(stmt *sql.Stmt, u models.DbUsers) (sql.Result, error) {

	return stmt.Exec(u.FirstName, u.LastName, u.Userpic, u.TelegramID)

}

func ExecUpdateTaskStatus(stmt *sql.Stmt, t models.DbTasks) (sql.Result, error) {

	return stmt.Exec(t.Status, t.ChangedAt, t.ChangedBy, t.ID)
}

func ExecUpdateTaskComment(stmt *sql.Stmt, t models.DbTasks) (sql.Result, error) {

	return stmt.Exec(t.Comment, t.CommentedAt, t.CommentedBy, t.ID)
}

func ExecUpdateAuth(stmt *sql.Stmt, a models.DbAuth) (sql.Result, error) {

	return stmt.Exec(a.Approved, a.Token)

}

func ExecUpdateSessionLastActivityByUuid(stmt *sql.Stmt, s models.DbSessions) (sql.Result, error) {

	return stmt.Exec(s.LastActivity, s.UUID)

}