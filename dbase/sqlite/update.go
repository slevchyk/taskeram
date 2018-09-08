package sqlite

import "database/sql"

func UpdateUserStatus(db *sql.DB) (*sql.Stmt, error) {

	return db.Prepare(`
			UPDATE
				users
			SET
				status=?,
				changed_at=?,
				changed_by=?
			WHERE
				tgid=? 
			`)
}

func UpdateTaskStatus(db *sql.DB) (*sql.Stmt, error) {

	return db.Prepare(`
		UPDATE
			tasks
		SET
			status=?,
			changed_at=?,
			changed_by=?
		WHERE 
			id=?`)
}

func UpdateTaskComment(db *sql.DB) (*sql.Stmt, error) {

	return db.Prepare(`
		UPDATE 
			tasks
		SET
			comment=?,
			commented_at=?,
			commented_by=?
		WHERE
			id=?;`)
}
