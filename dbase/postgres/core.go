package postgres

import (
	"database/sql"
	"fmt"
	"github.com/slevchyk/taskeram/models"
	"log"
	"strconv"
	"time"
)

func ConntectDB(cfg models.Config) (*sql.DB, error) {

	dbName := fmt.Sprintf("postgres://%v:%v@localhost/%v?sslmode=disable", cfg.Database.User, cfg.Database.Password, cfg.Database.Name)
	db, err := sql.Open("postgres", dbName)

	return db, err
}

func InitDB(db *sql.DB, cfg models.Config) {

	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			tgid INT,
			first_name TEXT,
			last_name TEXT,
			admin INT DEFAULT 0,
			status TEXT,
			changed_at TIMESTAMP WITH TIME ZONE,
			changed_by INT DEFAULT 0,
			comment TEXT DEFAULT '',
			userpic TEXT DEFAULT '');`)
	if err != nil {
		log.Fatal(err)
	}


	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS user_history (
			id SERIAL PRIMARY KEY,
			userid INT REFERENCES users(id),
			status TEXT,
			changed_by INT DEFAULT 0,
			changed_at TIMESTAMP WITH TIME ZONE,
			admin INT DEFAULT 0);`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		DROP TRIGGER IF EXISTS update_user_history on public.users;`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE OR REPLACE FUNCTION update_user_history()
		RETURNS trigger AS
		$BODY$
		BEGIN
			IF NEW.status <> OLD.status THEN
				INSERT INTO user_history(status, changed_by, changed_at, admin)
				VALUES (NEW.status, NEW.changed_by, NEW.changed_at, NEW.admin);
			END IF;
		END;
		$BODY$
 		LANGUAGE plpgsql;`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TRIGGER update_user_history
  		AFTER UPDATE
		ON users
  		FOR EACH ROW
		EXECUTE PROCEDURE update_user_history();`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS  tasks (
			id SERIAL PRIMARY KEY,
			from_user INT NOT NULL,
			to_user INT NOT NULL,
			status TEXT NOT NULL,
			changed_at TIMESTAMP WITH TIME ZONE NOT NULL,
			changed_by INT NOT NULL,
			title TEXT NOT NULL,
			description TEXT DEFAULT '',
			comment TEXT DEFAULT '',
			commented_at TIMESTAMP WITH TIME ZONE,
			commented_by INT,
			images TEXT DEFAULT '',
			documents TEXT DEFAULT '');`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS task_history (
			id SERIAL PRIMARY KEY,
			taskid INT REFERENCES tasks(id),
			tgid INT,
			date TIMESTAMP WITH TIME ZONE,
			status INT,
			comment TEXT);`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		DROP TRIGGER IF EXISTS insert_task_history on public.tasks;`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE OR REPLACE FUNCTION insert_task_history()
		RETURNS trigger AS
		$BODY$
		BEGIN
			INSERT INTO task_history(date, status, taskid, tgid)
			VALUES (NEW.changed_at, NEW.status, NEW.id, NEW.changed_by);
		END;
		$BODY$
 		LANGUAGE plpgsql;`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TRIGGER insert_task_history
  		AFTER INSERT 
		ON tasks
  		FOR EACH ROW
		EXECUTE PROCEDURE insert_task_history();`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		DROP TRIGGER IF EXISTS update_task_history on public.tasks;`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE OR REPLACE FUNCTION update_task_history()
		RETURNS trigger AS
		$BODY$
		BEGIN
			IF NEW.status <> OLD.status THEN
				INSERT INTO task_history(date, status, taskid, tgid)
				VALUES (NEW.changed_at, NEW.status, NEW.id, NEW.changed_by);
			END IF;
		END;
		$BODY$
 		LANGUAGE plpgsql;`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TRIGGER update_task_history
  		AFTER INSERT 
		ON tasks
  		FOR EACH ROW
		EXECUTE PROCEDURE update_task_history();`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS task_comments(
			id SERIAL PRIMARY KEY,
			taskid INT REFERENCES tasks(id),
			tgid INT,
			date TIMESTAMP WITH TIME ZONE,
			comment TEXT);`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		DROP TRIGGER IF EXISTS update_task_comments on public.tasks;`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE OR REPLACE FUNCTION update_task_comments()
		RETURNS trigger AS
		$BODY$
		BEGIN
			IF NEW.comment <> OLD.comment THEN
				INSERT INTO task_comments(taskid, tgid, date, comment)
				VALUES (NEW.id, NEW.commented_by, NEW.commented_at,  NEW.comment);
			END IF;
		END;
		$BODY$
 		LANGUAGE plpgsql;`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TRIGGER update_task_comments
  		AFTER INSERT 
		ON tasks
  		FOR EACH ROW
		EXECUTE PROCEDURE update_task_comments();`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS sessions (
				id SERIAL PRIMARY KEY,
				uuid TEXT NOT NULL,
				tgid INT NOT NULL,
				started_at TIMESTAMP WITH TIME ZONE,
				last_activity TIMESTAMP WITH TIME ZONE,
				ip TEXT DEFAULT '',
				user_agent TEXT DEFAULT '');`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS auth (
				id SERIAL PRIMARY KEY,
				token TEXT NOT NULL ,
				expiry_date TIMESTAMP WITH TIME ZONE NOT NULL,
				tgid INT NOT NULL ,
				approved INT DEFAULT 0);`)
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
			u.tgid=$1`, tgID)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	if !rows.Next() {
		stmt, err := db.Prepare(`
			INSERT INTO users (tgid, first_name, last_name, admin, status, changed_at) VALUES ($1, $2, $3, $4, $5, $6)`)
		if err != nil {
			log.Fatal(err)
		}

		_, err = stmt.Exec(tgID, "admin", "admin", 1, models.UserApprowed, time.Now().UTC())
		if err != nil {
			log.Fatal(err)
		}
	}
}

