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
			'tgid' INTEGER,
			'date' DATE,
			'status' INTEGER,			
			'comment' TEXT);`)
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
			'tgid' INTEGER,
			'date' DATE,			
			'comment' TEXT);`)
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
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS 'auth' (
				'id' INTEGER PRIMARY KEY AUTOINCREMENT,
				'token' TEXT NOT NULL ,
				'expiry_date' DATE NOT NULL,	
				'tgid' INTEGER NOT NULL ,				
				'approved' INTEGER DEFAULT 0);`)
	if err != nil {
		log.Fatal(err)
	}

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
