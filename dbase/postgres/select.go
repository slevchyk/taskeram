package postgres

import (
	"database/sql"
	"github.com/slevchyk/taskeram/models"
)

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
			tgid=$1
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
			status=$1
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
			tgid=$1
			AND status=$2
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
			(u.status=$1 OR u.status=$2)
			AND u.tgid!=$3
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
			u.status=$1
			AND u.tgid!=$2
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
			t.id=$1
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
			t.id=$1
			AND (t.from_user=$2
			OR t.to_user=$3)
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
			t.to_user=$1
			AND t.status=$2
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
			t.from_user=$1
			AND t.status=$2
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
			h.taskid=$1	
			AND (t.from_user=$2
				OR t.to_user=$3)		
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
			c.taskid=$1
			AND (t.from_user=$2
				OR t.to_user=$3)		
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
			token=$1`, token)
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
		WHERE s.uuid = $1;`, uuid)
}
