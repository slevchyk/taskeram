package postgres

import "database/sql"

func UpdateUserStatus(db *sql.DB) (*sql.Stmt, error) {

	return db.Prepare(`
			UPDATE
				users
			SET
				status=$1,
				changed_at=$2,
				changed_by=$3
			WHERE
				tgid=$4 
			`)
}

func UpdateUserData(db *sql.DB) (*sql.Stmt, error) {

	return db.Prepare(`
			UPDATE
				users
			SET
				first_name=$1,
				last_name=$2,
				userpic=$3
			WHERE
				tgid=$4 
			`)
}

func UpdateTaskStatus(db *sql.DB) (*sql.Stmt, error) {

	return db.Prepare(`
		UPDATE
			tasks
		SET
			status=$1,
			changed_at=$2,
			changed_by=$3
		WHERE 
			id=$4`)
}

func UpdateTaskComment(db *sql.DB) (*sql.Stmt, error) {

	return db.Prepare(`
		UPDATE 
			tasks
		SET
			comment=$1,
			commented_at=$2,
			commented_by=$3
		WHERE
			id=$4;`)
}

func UpdateAuth(db *sql.DB) (*sql.Stmt, error) {

	return db.Prepare(`
		UPDATE 
			auth
		SET
			approved=$1
		WHERE
			token=$2;`)
}

func UpdateSessionLastActivityByUuid(db *sql.DB) (*sql.Stmt, error) {

	return db.Prepare(`
		UPDATE 
			sessions
		SET
			last_activity=$1
		WHERE
			uuid=$2;`)
}