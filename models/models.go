package models

import (
	"database/sql"
	"database/sql/driver"
	"gopkg.in/telegram-bot-api.v4"
	"time"
)

type Config struct {
	Telegram struct {
		Token   string `json:"token"`
		AdminID string `json:"admin_id"`
	} `json:"telegram"`
	Database struct {
		Type     string `json:"type"`
		Name     string `json:"name"`
		User     string `json:"user"`
		Password string `json:"password"`
	} `json:"database"`
	DB *sql.DB
}

//NullTime special type for scan sql rows with Null data for time type variables
type NullTime struct {
	Time  time.Time `json:"time"`
	Valid bool      `json:"valid"` // Valid is true if Time is not NULL
}

// Scan implements the Scanner interface.
func (nt *NullTime) Scan(value interface{}) error {
	nt.Time, nt.Valid = value.(time.Time)
	return nil
}

// Value implements the driver Valuer interface.
func (nt NullTime) Value() (driver.Value, error) {
	if !nt.Valid {
		return nil, nil
	}
	return nt.Time, nil
}

type DbUsers struct {
	ID         int      `json:"id"`
	TelegramID int      `json:"telegram_id"`
	FirstName  string   `json:"first_name"`
	LastName   string   `json:"last_name"`
	Admin      int      `json:"admin"`
	Status     string   `json:"status"`
	ChangedBy  int      `json:"changed_by"`
	ChangedAt  NullTime `json:"changed_at"`
	Comment    string   `json:"comment"`
	Userpic    string   `json:"userpic"`
}

type DbTasks struct {
	ID          int      `json:"id"`
	FromUser    int      `json:"from_user"`
	ToUser      int      `json:"to_user"`
	Status      string   `json:"status"`
	ChangedAt   NullTime `json:"changed_at"`
	ChangedBy   int      `json:"changed_by"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Comment     string   `json:"comment"`
	CommentedAt NullTime `json:"commented_at"`
	CommentedBy int      `json:"commented_by"`
	Images      string   `json:"images"`
	Documents   string   `json:"documents"`
}

type DbTaskHistory struct {
	ID     int      `json:"id"`
	TaskID int      `json:"task_id"`
	UserID int      `json:"user_id"`
	Date   NullTime `json:"date"`
	Status string   `json:"status"`
}

type DbTaskComments struct {
	ID      int
	TaskID  int
	UserID  int
	Date    NullTime
	Comment string
}

type userSlider struct {
	EditingUserIndx int
	Users           map[int]DbUsers
}

type taskSlider struct {
	EditingTaskIndx int
	Tasks           map[int]DbTasks
}

type UserCache struct {
	User           DbUsers
	ChatID         int64
	MessageID      int
	Text           string
	Command        string
	Arguments      string
	CallbackID     string
	CallbackData   string
	TaskID         int
	CurrentMenu    string
	CurrentMessage int
	NewTask        *Task
	Users          map[int]DbUsers
	UserSlider     userSlider
	TaskSlider     taskSlider
	Message        *tgbotapi.Message
}

type Task struct {
	Step        int
	ToUser      *DbUsers
	Title       string
	Description string
}

type AllowedActions []string

func (aa AllowedActions) Contains(s string) bool {

	for _, val := range aa {
		if val == s {
			return true
		}
	}

	return false
}

type Buttons struct {
	Main      tgbotapi.KeyboardButton
	Next      tgbotapi.KeyboardButton
	Users     tgbotapi.KeyboardButton
	Back      tgbotapi.KeyboardButton
	View      tgbotapi.KeyboardButton
	All       tgbotapi.KeyboardButton
	Requests  tgbotapi.KeyboardButton
	Banned    tgbotapi.KeyboardButton
	Edit      tgbotapi.KeyboardButton
	Approve   tgbotapi.KeyboardButton
	Ban       tgbotapi.KeyboardButton
	Unban     tgbotapi.KeyboardButton
	Inbox     tgbotapi.KeyboardButton
	Sent      tgbotapi.KeyboardButton
	New       tgbotapi.KeyboardButton
	Started   tgbotapi.KeyboardButton
	Rejected  tgbotapi.KeyboardButton
	Completed tgbotapi.KeyboardButton
	Closed    tgbotapi.KeyboardButton
	Save      tgbotapi.KeyboardButton
	Cancel    tgbotapi.KeyboardButton
	Start     tgbotapi.KeyboardButton
	Complete  tgbotapi.KeyboardButton
	History   tgbotapi.KeyboardButton
	Close     tgbotapi.KeyboardButton
	Reject    tgbotapi.KeyboardButton
}

type DbHistory struct {
	HDb DbTaskHistory `json:"h_db"`
	TDb DbTasks       `json:"t_db"`
	UDb DbUsers       `json:"u_db"`
}

type DbComment struct {
	CDb DbTaskComments
	TDb DbTasks
	UDb DbUsers
}

type DbAuth struct {
	ID         int
	Token      string
	ExpiryDate NullTime
	TelegramID int
	Approved   int
}

type DbSessions struct {
	ID           int
	UUID         string
	TelegramID   int
	StartedAt    NullTime
	LastActivity NullTime
	IP           string
	UserAgent    string
}
