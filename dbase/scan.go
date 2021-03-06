package dbase

import (
	"database/sql"
	"github.com/slevchyk/taskeram/models"
)

func ScanUser(rows *sql.Rows, u *models.DbUsers) error {
	return rows.Scan(&u.ID, &u.TelegramID, &u.FirstName, &u.LastName, &u.Admin, &u.Status, &u.ChangedBy, &u.ChangedAt, &u.Comment, &u.Userpic)
}

func ScanTask(rows *sql.Rows, t *models.DbTasks) error {
	return rows.Scan(&t.ID, &t.FromUser, &t.ToUser, &t.Status, &t.ChangedAt, &t.ChangedBy, &t.Title, &t.Description, &t.Comment, &t.CommentedAt, &t.CommentedBy, &t.Images, &t.Documents)
}

func ScanHistory(rows *sql.Rows, h *models.DbHistory) error {
	return rows.Scan(&h.HDb.Status, &h.HDb.Date, &h.HDb.TaskID, &h.UDb.TelegramID, &h.UDb.FirstName, &h.UDb.LastName, &h.TDb.Title)
}

func ScanComments(rows *sql.Rows, c *models.DbComment) error {
	return rows.Scan(&c.CDb.Comment, &c.CDb.Date, &c.CDb.TaskID, &c.UDb.TelegramID, &c.UDb.FirstName, &c.UDb.LastName, &c.TDb.Title)
}

func ScanAuth(rows *sql.Rows, a *models.DbAuth) error {
	return rows.Scan(&a.ID, &a.Token, &a.ExpiryDate, &a.TelegramID, &a.Approved)
}

func ScanSession(rows *sql.Rows, s *models.DbSessions) error {
	return rows.Scan(&s.ID, &s.UUID, &s.TelegramID, &s.StartedAt, &s.LastActivity, &s.IP, &s.UserAgent)
}

