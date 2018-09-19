package sqlite

import "database/sql"

func DeleteAuthByToken(db *sql.DB) (*sql.Stmt, error)  {

	return db.Prepare(`
		DELETE 
		FROM auth
		WHERE
			token=?;`)
}

func DeleteSessionByUUID(db *sql.DB) (*sql.Stmt, error)  {

	return db.Prepare(`
		DELETE 
		FROM sessions
		WHERE
			uuid=?;`)
}
