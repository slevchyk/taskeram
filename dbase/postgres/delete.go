package postgres

import "database/sql"

func DeleteAuthByToken(db *sql.DB) (*sql.Stmt, error)  {

	return db.Prepare(`
		DELETE 
		FROM auth
		WHERE
			token=$1;`)
}

func DeleteSessionByUUID(db *sql.DB) (*sql.Stmt, error)  {

	return db.Prepare(`
		DELETE 
		FROM sessions
		WHERE
			uuid=$1;`)
}
