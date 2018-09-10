package sqlite

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
				comment)
		VALUES (?, ?, ?, ?, ?, ?. ?, ?)`)
}

func InsertTask(db *sql.DB) (*sql.Stmt, error) {

	return db.Prepare(`
		INSERT INTO
			'tasks'(
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
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?) `)
}