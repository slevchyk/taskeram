package main

import (
	"database/sql/driver"
	"time"

	"gopkg.in/telegram-bot-api.v4"
)

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

type dbUsers struct {
	ID         int
	TelegramID int
	FirstName  string
	LastName   string
	Admin      int
	Status     string
	ChangedBy  int
	ChangedAt  NullTime
}

type dbTasks struct {
	ID          int
	FromUser    int
	ToUser      int
	Status      string
	ChangedAt   NullTime
	ChangedBy   int
	Comments    string
	Title       string
	Description string
	Images      string
	Documents   string
}

type dbTaskHistory struct {
	ID       int
	TaskID   int
	UserID   int
	Date     NullTime
	Status   string
	Comments string
}

type dbNotifications struct {
	ID        int
	TUserID   int
	MessageID int
	ACTION    string
}

type UserCache struct {
	User            dbUsers
	ChatID          int64
	MessageID       int
	Text            string
	Command         string
	Arguments       string
	CallbackID      string
	CallbackData    string
	TaskID          int
	currentMenu     string
	currentMessage  int
	NewTask         *task
	editingUserIndx int
	users           map[int]dbUsers
	editingTaskIndx int
	tasks           map[int]dbTasks
	Message         *tgbotapi.Message
}

type task struct {
	Step        int
	ToUser      *dbUsers
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

type History struct {
	h dbTaskHistory
	t dbTasks
	u dbUsers
}

type Buttons struct {
	Main		tgbotapi.KeyboardButton
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
