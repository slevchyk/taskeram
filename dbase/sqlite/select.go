package sqlite

import (
	"database/sql"
	"fmt"
	"github.com/slevchyk/taskeram/models"
	"log"
	"strconv"
	"time"
)

func ConntectDB(cfg models.Config) (*sql.DB, error) {

	dbName := fmt.Sprintf("%v.sqlite", cfg.Database.Name)
	db, err := sql.Open("sqlite3", dbName)

	return db, err
}

func InitDB(db *sql.DB, cfg models.Config) {

	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS 'users'(
			'id' INTEGER PRIMARY KEY AUTOINCREMENT,
			'tgid' INTEGER,			
			'first_name' TEXT,
			'last_name' TEXT,
			'admin' INTEGER DEFAULT 0,
			'status' TEXT,
			'changed_at' DATE,
			'changed_by' INTEGER DEFAULT 0,
			'comment' TEXT DEFAULT '',
			'userpic' TEXT DEFAULT '');`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS 'user_history'(
			'id' INTEGER PRIMARY KEY AUTOINCREMENT,
			'userid' INTEGER REFERENCES users,
			'status' TEXT,
			'changed_by' INTEGER DEFAULT 0,
			'changed_at' DATE,
			'admin' INTEGER);`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TRIGGER IF NOT EXISTS update_user_history AFTER UPDATE ON users WHEN (old.status <> new.status)
		BEGIN
			INSERT INTO user_history(status, changed_by, changed_at, admin) values (new.status, new.changed_by, new.changed_at, new.admin);
		END;`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS  'tasks'(
			'id' INTEGER PRIMARY KEY AUTOINCREMENT ,
			'from_user' INTEGER NOT NULL,
			'to_user' INTEGER NOT NULL,
			'status' TEXT NOT NULL,
			'changed_at' DATE NOT NULL,
			'changed_by' INTEGER NOT NULL,			
			'title' TEXT NOT NULL,
			'description' TEXT DEFAULT '',
			'comment' TEXT DEFAULT '',
			'commented_at' DATE,
			'commented_by' INTEGER,
			'images' TEXT DEFAULT '',
			'documents' TEXT DEFAULT '');`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS 'task_history'(
			'id' INTEGER PRIMARY KEY AUTOINCREMENT,
			'taskid' INTEGER REFERENCES tasks,
			'tgid' INTEGER REFERENCES users,
			'date' DATE,
			'status' INTEGER,			
			'comment' TEXT
			);`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TRIGGER IF NOT EXISTS insert_task_history AFTER INSERT ON tasks
		BEGIN
			INSERT INTO task_history(date, status, taskid, tgid) values (new.changed_at, new.status, new.id, new.changed_by);
		END;`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TRIGGER IF NOT EXISTS update_task_history AFTER UPDATE ON tasks WHEN (old.status <> new.status)
		BEGIN
			INSERT INTO task_history(date, status, taskid, tgid) values (new.changed_at, new.status, new.id, new.changed_by);
		END;`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS 'task_comments'(
			'id' INTEGER PRIMARY KEY AUTOINCREMENT,
			'taskid' INTEGER REFERENCES tasks,
			'tgid' INTEGER REFERENCES users,
			'date' DATE,			
			'comment' TEXT
			);`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TRIGGER IF NOT EXISTS update_task_comments AFTER UPDATE ON tasks WHEN (old.comment <> new.comment)
		BEGIN
			INSERT INTO task_comments(taskid, tgid, date, comment) values (new.id, new.commented_by, new.commented_at,  new.comment);
		END;`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS 'sessions' (
				'id' INTEGER PRIMARY KEY AUTOINCREMENT,
				'uuid' TEXT NOT NULL,
				'tgid' INTEGER NOT NULL,
				'started_at' DATE,
				'last_activity' DATE,
				'ip' TEXT DEFAULT '',
				'user_agent' TEXT DEFAULT '');`)

	if cfg.Telegram.AdminID == "" {
		log.Fatal("Telegram Admin ID does not exist in config file")
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS 'auth' (
				'id' INTEGER PRIMARY KEY AUTOINCREMENT,
				'token' TEXT NOT NULL ,
				'expiry_date' DATE NOT NULL,	
				'tgid' INTEGER NOT NULL ,				
				'approved' INTEGER DEFAULT 0);`)

	if cfg.Telegram.AdminID == "" {
		log.Fatal("Telegram Admin ID does not exist in config file")
	}

	tgID, err := strconv.Atoi(cfg.Telegram.AdminID)
	if err != nil {
		log.Fatal(err)
	}

	rows, err := db.Query(`
		SELECT
		u.id
		FROM 
			users u
		WHERE
			u.tgid=?`, tgID)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	if !rows.Next() {
		stmt, err := db.Prepare(`
			INSERT into 'users'(tgid, first_name, last_name, admin, status, changed_at) VALUES (?,?,?,?,?,?)`)
		if err != nil {
			log.Fatal(err)
		}

		_, err = stmt.Exec(tgID, "admin", "admin", 1, models.UserApprowed, time.Now().UTC())
		if err != nil {
			log.Fatal(err)
		}
	}
}

func SelectUsersByTelegramID(db *sql.DB, tgid int) (*sql.Rows, error) {

	return db.Query(`
		SELECT
			u.id,
			u.tgid,			
			u.first_name,
			u.last_name,
			u.admin,
			u.status,
			u.changed_by,
			u.changed_at,
			u.comment,
			u.userpic		
		FROM 
			users u
		WHERE
			tgid=?
		ORDER BY
				u.id`, tgid)
}

func SelectUsersByStatus(db *sql.DB, status string) (*sql.Rows, error) {

	return db.Query(`
		SELECT
			u.id,
			u.tgid,			
			u.first_name,
			u.last_name,
			u.admin,
			u.status,
			u.changed_by,
			u.changed_at,
			u.comment,
			u.userpic			
		FROM 
			users u
		WHERE
			status=?
		ORDER BY
				u.id`, status)
}

func SelectUsersByTelegramIDStatus(db *sql.DB, tgid int, status string) (*sql.Rows, error) {

	return db.Query(`
		SELECT
			u.id,
			u.tgid,			
			u.first_name,
			u.last_name,
			u.admin,
			u.status,
			u.changed_by,
			u.changed_at,
			u.comment,
			u.userpic	
		FROM 
			users u
		WHERE
			tgid=?
			AND status=?
		ORDER BY
				u.id`, tgid, status)
}

func SelectAdminUsers(db *sql.DB) (*sql.Rows, error) {

	return db.Query(`
		SELECT
			u.id,
			u.tgid,			
			u.first_name,
			u.last_name,
			u.admin,
			u.status,
			u.changed_by,
			u.changed_at,
			u.comment,
			u.userpic			
		FROM 
			users u
		WHERE
			admin=1
		ORDER BY
				u.id`)
}

func SelectUsersForBan(db *sql.DB, tgid int) (*sql.Rows, error) {

	return db.Query(`
		SELECT
			u.id,
			u.tgid,			
			u.first_name,
			u.last_name,
			u.admin,
			u.status,
			u.changed_by,
			u.changed_at,
			u.comment,
			u.userpic		
		FROM 
			users u
		WHERE
			(u.status=? OR u.status=?)
			AND u.tgid!=?
		ORDER BY
				u.id`, models.UserRequested, models.UserApprowed, tgid)
}

func SelectUsersForUnban(db *sql.DB, tgid int) (*sql.Rows, error) {

	return db.Query(`
		SELECT
			u.id,
			u.tgid,			
			u.first_name,
			u.last_name,
			u.admin,
			u.status,
			u.changed_by,
			u.changed_at,
			u.comment,
			u.userpic	
		FROM 
			users u
		WHERE
			u.status=?
			AND u.tgid!=?
		ORDER BY
				u.id`, models.UserBanned, tgid)
}

func SelectTasksByID(db *sql.DB, taskID int) (*sql.Rows, error) {

	return db.Query(`
		SELECT
			t.ID,
			t.from_user,
			t.to_user,
			t.status,
			t.changed_at,
			t.changed_by,
			t.title,
			t.description,
			t.comment,
			t.commented_at,
			t.commented_by,
			t.images,
			t.documents			
		FROM tasks t
		WHERE
			t.id=?
		ORDER BY
			t.id`, taskID)
}

func SelectTasksByIDUserTelegramID(db *sql.DB, taskID int, tgid int) (*sql.Rows, error) {

	return db.Query(`
		SELECT
			t.ID,
			t.from_user,
			t.to_user,
			t.status,
			t.changed_at,
			t.changed_by,
			t.title,
			t.description,
			t.comment,
			t.commented_at,
			t.commented_by,
			t.images,
			t.documents		
		FROM tasks t
		WHERE
			t.id=?
			AND (t.from_user=?
			OR t.to_user=?)
		ORDER BY
			t.id`, taskID, tgid, tgid)
}

func SelectInboxTasks(db *sql.DB, tgid int, status string) (*sql.Rows, error) {

	return db.Query(`
		SELECT
			t.ID,
			t.from_user,
			t.to_user,
			t.status,
			t.changed_at,
			t.changed_by,
			t.title,
			t.description,
			t.comment,
			t.commented_at,
			t.commented_by,
			t.images,
			t.documents			
		FROM tasks t
		WHERE
			t.to_user=?
			AND t.status=?
		ORDER BY
			t.id`, tgid, status)
}

func SelectSentTasks(db *sql.DB, tgid int, status string) (*sql.Rows, error) {

	return db.Query(`
		SELECT
			t.ID,
			t.from_user,
			t.to_user,
			t.status,
			t.changed_at,
			t.changed_by,
			t.title,
			t.description,
			t.comment,
			t.commented_at,
			t.commented_by,
			t.images,
			t.documents			
		FROM tasks t
		WHERE
			t.from_user=?
			AND t.status=?
		ORDER BY
			t.id`, tgid, status)
}

func SelectHistory(db *sql.DB, taskID int, tgid int) (*sql.Rows, error) {

	return db.Query(`
		SELECT 
			h.status,
			h.date,
			h.taskid,							
			u.tgid,
			u.first_name,
			u.last_name,
			t.title
		FROM
			task_history h
		LEFT JOIN
			users u
			ON h.tgid = u.tgid
		LEFT JOIN
			tasks t 
			ON h.taskid = t.id
		WHERE
			h.taskid=?	
			AND (t.from_user=?
				OR t.to_user=?)		
		ORDER BY 
			h.date`, taskID, tgid, tgid)
}

func SelectComments(db *sql.DB, taskID int, tgid int) (*sql.Rows, error) {

	return db.Query(`
		SELECT 
			c.comment,
			c.date,
			c.taskid,							
			c.tgid,
			u.first_name,
			u.last_name,
			t.title
		FROM
			task_comments c
		LEFT JOIN
			users u
			ON c.tgid = u.tgid
		LEFT JOIN
			tasks t 
			ON c.taskid = t.id
		WHERE
			c.taskid=?
			AND (t.from_user=?
				OR t.to_user=?)		
		ORDER BY 
			c.date`, taskID, tgid, tgid)
}

func SelectAuthByToken(db *sql.DB, token string) (*sql.Rows, error) {

	return db.Query(`
		SELECT 
			a.id,
			a.token,
			a.expiry_date,
			a.tgid,
			a.approved
		FROM auth a
		WHERE
			token=?`, token)
}

func SelectSessions(db *sql.DB) (*sql.Rows, error) {

	return db.Query(`
		SELECT 
			s.id,
			s.uuid,
			s.tgid,
			s.started_at,
			s.last_activity,
			s.ip,
			s.user_agent
		FROM sessions s`)
}

func SelectUsersBySessionUUID(db *sql.DB, uuid string) (*sql.Rows, error) {

	return db.Query(`
		SELECT 
			u.id,
			u.tgid,			
			u.first_name,
			u.last_name,
			u.admin,
			u.status,
			u.changed_by,
			u.changed_at,
			u.comment,
			u.userpic	
		FROM sessions s
			LEFT JOIN users u
			ON s.tgid = u.tgid
		WHERE s.uuid = ?;`, uuid)
}
