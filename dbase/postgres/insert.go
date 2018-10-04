package postgres

import "database/sql"

func InsertUser(db *sql.DB) (*sql.Stmt, error)  {

	return db.Prepare(`
		INSERT INTO
			users (
				tgid,
				first_name,
				last_name,
				admin,
				status,				 
				changed_at,
				changed_by,
				comment,
				userpic)
		VALUES ($1, $2, $3, $4, $5, $6. $7, $8, $9);`)
}

func InsertTask(db *sql.DB) (*sql.Stmt, error) {

	return db.Prepare(`
		INSERT INTO
			tasks(
				from_user,
				to_user,
				status,
				changed_at,
				changed_by,
				title,
				description,
				comment,
				commented_at,
				commented_by,
				images,
				documents)
		VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12);`)
}

func InsertAuth(db *sql.DB) (*sql.Stmt, error) {

	return db.Prepare(`
		INSERT INTO 
			auth (
				token,
				expiry_date,
				tgid,
				approved)
		VALUES ($1, $2, $3, $4);`)
}

func InsertSession(db *sql.DB) (*sql.Stmt, error) {

	return db.Prepare(`
		INSERT INTO 
			sessions (
				uuid,
				tgid,
				started_at,
				last_activity,
				ip,
				user_agent)
		VALUES ($1, $2, $3, $4, $5, $6);`)
}