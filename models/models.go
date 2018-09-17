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
	Time  time.Time
	Valid bool // Valid is true if Time is not NULL
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
	ID         int
	TelegramID int
	FirstName  string
	LastName   string
	Admin      int
	Status     string
	ChangedBy  int
	ChangedAt  NullTime
	Comment    string
	Userpic    string
	Password string
}

type DbTasks struct {
	ID          int
	FromUser    int
	ToUser      int
	Status      string
	ChangedAt   NullTime
	ChangedBy   int
	Title       string
	Description string
	Comment     string
	CommentedAt NullTime
	CommentedBy int
	Images      string
	Documents   string
}

type DbTaskHistory struct {
	ID     int
	TaskID int
	UserID int
	Date   NullTime
	Status string
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
	HDb DbTaskHistory
	TDb DbTasks
	UDb DbUsers
}

type DbComment struct {
	CDb DbTaskComments
	TDb DbTasks
	UDb DbUsers
}

type DbAuth struct {
	ID int
	Token string
	ExpiryDate NullTime
	TelegramID int
	Approved int
}

type DbSessions struct {
	ID int
	UUID string
	TelegramID int
	StartedAt NullTime
	LastActivity NullTime
	IP string
	UserAgent string
}