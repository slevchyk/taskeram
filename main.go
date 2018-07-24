package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/slevchyk/taskeram/buttons"
	"github.com/slevchyk/taskeram/models"
	"gopkg.in/telegram-bot-api.v4"
)

var db *sql.DB
var bot *tgbotapi.BotAPI
var cache map[int]*UserCache
var taskRules map[string]map[string]AllowedActions

const (
	NewUserRequest = "NewUserRequest"
	NewUserCancel  = "NewUserCancel"
	NewUserAccept  = "NewUserAccept"
	NewUserDecline = "NewUserDecline"
)

const (
	ACTION_USERREQUEST = "userRequest"
)

func init() {
	var err error

	db, err = sql.Open("sqlite3", "tasker.sqlite")
	if err != nil {
		log.Fatal("Can't connect to DB ", err.Error())
	}

	token := os.Getenv("TELEGRAM_TASKERAM_TOKEN")

	bot, err = tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Fatal(err)
	}
}

func main() {

	defer db.Close()

	cache = make(map[int]*UserCache)

	initDB()

	taskRules = make(map[string]map[string]AllowedActions)
	inboxRules := make(map[string]AllowedActions)
	sentRules := make(map[string]AllowedActions)

	inboxRules["New"] = AllowedActions{"Start", "Comment", "Complete", "History"}
	inboxRules["Start"] = AllowedActions{"Complete", "Comment", "History"}
	inboxRules["Reject"] = AllowedActions{"Complete", "Comment", "History"}
	inboxRules["Complete"] = AllowedActions{"History"}
	inboxRules["Close"] = AllowedActions{"History"}
	taskRules["Inbox"] = inboxRules

	sentRules["New"] = AllowedActions{"Close", "Comment", "History"}
	sentRules["Start"] = AllowedActions{"Close", "Comment", "History"}
	sentRules["Complete"] = AllowedActions{"Reject", "Comment", "Close", "History"}
	sentRules["Close"] = AllowedActions{"History"}
	taskRules["Sent"] = sentRules

	bot.Debug = false
	log.Printf("Authorized on account %s", bot.Self.UserName)

	// инициализируем канал, куда будут прилетать обновления от API
	var ucfg tgbotapi.UpdateConfig
	var c *UserCache

	ucfg = tgbotapi.NewUpdate(0)
	ucfg.Timeout = 60

	upd, _ := bot.GetUpdatesChan(ucfg)
	// читаем обновления из канала
	for {
		select {
		case update := <-upd:

			var tgid int

			if update.Message != nil {
				tgid = update.Message.From.ID
			} else if update.CallbackQuery != nil {
				tgid = update.CallbackQuery.From.ID
			}

			if tgid == 0 {
				continue
			}

			u := getUserByTelegramID(tgid)

			//Якщо в цього користувача ще не має власних налаштувань сесії, то ініціюємо їх
			if _, ok := cache[tgid]; !ok {
				cache[tgid] = &UserCache{User: u}
			}

			c = cache[tgid]

			if update.CallbackQuery != nil {

				if u.ID == 0 {
					c.User.TelegramID = tgid
					c.User.FirstName = update.CallbackQuery.From.FirstName
					c.User.LastName = update.CallbackQuery.From.LastName
				}

				c.Message = update.CallbackQuery.Message
				c.MessageID = update.CallbackQuery.Message.MessageID
				c.Text = update.CallbackQuery.Message.Text
				c.ChatID = update.CallbackQuery.Message.Chat.ID
				c.CallbackID = update.CallbackQuery.ID
				c.CallbackData = update.CallbackQuery.Data

				go handleCallbackQuery(c)
				continue
			}

			if update.Message != nil {
				//новий користвуач якого немає ще в нас в базі даних
				if u.ID == 0 {
					go serveNewUser(update)
					continue
				}

				if u.Status == models.UserBanned {
					go serveBannedUser(update)
				}

				//користувач надіслав запит на активацію але це ще не активований
				if u.Status != models.UserApprowed {
					go serveNonApprovedUser(update)
					continue
				}
				c.Message = update.Message

				go serveUser(c)
			}
		}
	}
}

func initDB() {

	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS 'users'(
			'id' INTEGER PRIMARY KEY AUTOINCREMENT,
			'tgid' INTEGER,			
			'first_name' TEXT,
			'last_name' TEXT,
			'admin' INTEGER DEFAULT 0,
			'status' TEXT,
			'changed_by' INTEGER DEFAULT 0,
			'changed_at' DATE,
			'comments' TEXT DEFAULT "");`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS 'user_history'(
			'id' INTEGER PRIMARY KEY AUTOINCREMENT,
			'userid' INTEGER REFERENCES users,
			'status' TEXT,
			'changed_by' INTEGER DEFAULT 0,
			'changed_at' DATE,
			'admin' INTEGER);`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS  'tasks'(
			'id' INTEGER PRIMARY KEY AUTOINCREMENT ,
			'from_user' INTEGER NOT NULL,
			'to_user' INTEGER NOT NULL,
			'status' TEXT NOT NULL,
			'changed_at' DATE NOT NULL,
			'changed_by' INTEGER NOT NULL,			
			'title' TEXT NOT NULL,
			'description' TEXT DEFAULT "",
			'comments' TEXT DEFAULT "",
			'images' TEXT DEFAULT "",
			'documents' TEXT DEFAULT "");`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS 'task_history'(
			'id' INTEGER PRIMARY KEY AUTOINCREMENT,
			'taskid' INTEGER REFERENCES tasks,
			'userid' INTEGER REFERENCES users,
			'date' DATE,
			'status' INTEGER,			
			'comments' TEXT
			);`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TRIGGER IF NOT EXISTS update_task_history AFTER UPDATE ON tasks WHEN (old.status <> new.status OR old.comments <> new.comments)
		BEGIN
			INSERT INTO task_history(date, status, taskid, comments, userid) values (new.changed_at, new.status, new.id, new.comments, new.changed_by);
		END;`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TRIGGER IF NOT EXISTS insert_task_history AFTER INSERT ON tasks
		BEGIN
			INSERT INTO task_history(date, status, taskid, comments, userid) values (new.changed_at, new.status, new.id, new.comments, new.changed_by);
		END;`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TRIGGER IF NOT EXISTS update_user_history AFTER UPDATE ON users WHEN (old.status <> new.status)
		BEGIN
			INSERT INTO user_history(status, changed_by, changed_at, admin) values (new.status, new.changed_by, new.changed_at, new.admin);
		END;`)
	if err != nil {
		log.Fatal(err)
	}

}

//запропонуємо користувачу зробити запит на активацію в програмі
func serveNewUser(update tgbotapi.Update) {

	ut := update.Message.From

	reply := fmt.Sprintf("Hello, %s %s. I can see you are new one here. Would you like to send request to approve your account in Taskeram?\n", ut.FirstName, ut.LastName)

	btnYes := tgbotapi.NewInlineKeyboardButtonData("✓ Yes", NewUserRequest)
	btnNo := tgbotapi.NewInlineKeyboardButtonData("🚫 No", NewUserCancel)
	keyboard := tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(btnYes, btnNo))

	msg := tgbotapi.NewMessage(update.Message.Chat.ID, reply)
	msg.ReplyMarkup = &keyboard

	bot.Send(msg)
}

func serveBannedUser(update tgbotapi.Update) {

	ut := update.Message.From

	reply := fmt.Sprintf("Hello, %s %s. I'm so sorry but yor request was declined.\nAsk admins to restore your account", ut.FirstName, ut.LastName)
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, reply)

	bot.Send(msg)
}

func serveNonApprovedUser(update tgbotapi.Update) {

	ut := update.Message.From

	reply := fmt.Sprintf("Hello, %s %s. Keep calm and wait for approval message!\n", ut.FirstName, ut.LastName)
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, reply)

	bot.Send(msg)
}

func serveUser(c *UserCache) {

	//опрацюємо текстове повідомлення від користувача
	if c.Message != nil {

		c.ChatID = c.Message.Chat.ID
		c.MessageID = c.Message.MessageID
		c.Text = c.Message.Text

		if c.Message.IsCommand() {
			c.Command = c.Message.Command()
			c.Arguments = c.Message.CommandArguments()
			handleCommand(c)
			return
		}

		cm := c.currentMenu
		msg := c.Text

		if cm == models.MenuMain && msg == models.Users {
			handleUsers(c)
		} else if cm == models.MenuUsers && msg == models.View {
			handleUsersView(c)
		} else if cm == models.MenuUsersView && msg == models.Back {
			handleUsers(c)
		} else if cm == models.MenuUsersView && msg == models.All {
			handleUsersViewALl(c)
		} else if cm == models.MenuUsersView && msg == models.Requests {
			handleUsersViewRequests(c)
		} else if cm == models.MenuUsersView && msg == models.Banned {
			handleUsersViewBanned(c)
		} else if cm == models.MenuUsers && msg == models.Edit {
			handleUsersEdit(c)
		} else if cm == models.MenuUsersEdit && msg == models.Back {
			handleUsers(c)
		} else if cm == models.MenuUsersEdit && msg == models.Approve {
			handleUsersEditApprove(c)
		} else if cm == models.MenuUsersEditApprove && msg == models.Back {
			handleUsersEdit(c)
		} else if cm == models.MenuUsersEditApprove {
			handleUsersEditApprove(c)
		} else if cm == models.MenuUsersEdit && msg == models.Ban {
			handleUsersEditBan(c)
		} else if cm == models.MenuUsersEditBan && msg == models.Back {
			handleUsersEdit(c)
		} else if cm == models.MenuUsersEditBan {
			handleUsersEditBan(c)
		} else if cm == models.MenuUsersEdit && msg == models.Unban {
			handleUsersEditUnban(c)
		} else if cm == models.MenuUsersEditUnban && msg == models.Back {
			handleUsersEdit(c)
		} else if cm == models.MenuUsersEditUnban {
			handleUsersEditUnban(c)
		} else if cm == models.MenuMain && msg == models.Inbox {
			handleInbox(c)
		} else if cm == models.MenuInbox && msg == models.Back {
			handleMain(c)
		} else if cm == models.MenuInbox && msg == models.New {
			handleInboxNew(c)
		} else if cm == models.MenuMain && msg == models.Sent {
			handleSent(c)
		} else if cm == models.MenuSent && msg == models.Back {
			handleMain(c)
		} else if cm == models.MenuMain && msg == models.New {
			handleNew(c)
		} else if cm == models.MenuNew {
			handleNew(c)
		} else if cm == models.Comment {
			handleComment(c)
		} else {
			handleMain(c)
		}
	}
}

func handleCommand(c *UserCache) {

	switch c.Command {
	case "task":
		handleCommandTask(c)
		return
	case "history":
		handleHistoryTask(c)
		return
	}

}

func handleCommandTask(c *UserCache) {

	var err error
	if c.Arguments == "" {
		msg := tgbotapi.NewMessage(c.ChatID, fmt.Sprintln(c.Arguments, "you should input task number after /task command "))
		bot.Send(msg)
		return
	}

	c.TaskID, err = strconv.Atoi(c.Arguments)
	if err != nil {
		msg := tgbotapi.NewMessage(c.ChatID, fmt.Sprintln(c.Arguments, " - wrong argument type"))
		bot.Send(msg)
		return
	}

	c.currentMessage = 0

	showTask(c)
}

func handleHistoryTask(c *UserCache) {

	var err error

	if c.Arguments == "" {
		msg := tgbotapi.NewMessage(c.ChatID, fmt.Sprintln(c.Arguments, "you should input task number after /task command "))
		bot.Send(msg)
		return
	}

	c.TaskID, err = strconv.Atoi(c.Arguments)
	if err != nil {
		msg := tgbotapi.NewMessage(c.ChatID, fmt.Sprintln(c.Arguments, " - wrong argument type"))
		bot.Send(msg)
		return
	}

	showHistory(c)
	handleMain(c)
}

func handleMain(c *UserCache) {

	c.currentMenu = models.MenuMain
	chatID := c.Message.Chat.ID

	var kbrd [][]tgbotapi.KeyboardButton

	kbrd = append(kbrd, tgbotapi.NewKeyboardButtonRow(buttons.Inbox, buttons.Sent, buttons.New))

	if c.User.Admin == 1 {
		kbrd = append(kbrd, tgbotapi.NewKeyboardButtonRow(buttons.Users))
	}

	markup := tgbotapi.NewReplyKeyboard(kbrd[:]...)
	markup.Selective = true

	msg := tgbotapi.NewMessage(chatID, "Main menu:")
	msg.ReplyMarkup = markup
	bot.Send(msg)
}

func handleUsers(c *UserCache) {

	if c.User.Admin == 0 {
		c.currentMenu = models.MenuMain
		handleMain(c)
		return
	}

	c.currentMenu = models.MenuUsers

	btnRow1 := tgbotapi.NewKeyboardButtonRow(buttons.Back)
	btnRow2 := tgbotapi.NewKeyboardButtonRow(buttons.View, buttons.Edit)

	markup := tgbotapi.NewReplyKeyboard(btnRow1, btnRow2)
	markup.Selective = true

	msg := tgbotapi.NewMessage(c.ChatID, "Users menu:")
	msg.ReplyMarkup = markup

	bot.Send(msg)
}

func handleUsersView(c *UserCache) {

	if c.User.Admin == 0 {
		c.currentMenu = models.MenuMain
		handleMain(c)
		return
	}

	c.currentMenu = models.MenuUsersView

	btnRow1 := tgbotapi.NewKeyboardButtonRow(buttons.Back)
	btnRow2 := tgbotapi.NewKeyboardButtonRow(buttons.All, buttons.Requests, buttons.Banned)

	markup := tgbotapi.NewReplyKeyboard(btnRow1, btnRow2)
	markup.Selective = true

	msg := tgbotapi.NewMessage(c.ChatID, "View users:")
	msg.ReplyMarkup = markup

	bot.Send(msg)
}

func handleUsersViewALl(c *UserCache) {

	if c.User.Admin == 0 {
		c.currentMenu = models.MenuMain
		handleMain(c)
		return
	}

	var reply string

	rows, err := db.Query(`
		SELECT 
			u.tgid,			
			u.first_name,
			u.last_name			
		FROM users u
		WHERE
			u.status=?
		ORDER BY
			u.id;`, models.UserApprowed)
	if err != nil {
		msg := tgbotapi.NewMessage(c.ChatID, "Internal error. Can't select users from db")
		msg.ReplyToMessageID = c.MessageID
		bot.Send(msg)

		handleUsersView(c)
	}
	defer rows.Close()

	var xs []dbUsers
	var u dbUsers

	for rows.Next() {
		rows.Scan(&u.TelegramID, &u.FirstName, &u.LastName)
		xs = append(xs, u)
	}

	reply = fmt.Sprintf("We have <b>%v</b> approved users:", len(xs))
	msg := tgbotapi.NewMessage(c.ChatID, reply)
	msg.ParseMode = "HTML"
	bot.Send(msg)

	for key, val := range xs {
		reply = fmt.Sprintf(`#%v <a href="tg://user?id=%v">%v %v</a>`, key+1, val.TelegramID, val.FirstName, val.LastName)
		msg := tgbotapi.NewMessage(c.ChatID, reply)
		msg.ParseMode = "HTML"
		bot.Send(msg)
	}
}

func handleUsersViewRequests(c *UserCache) {

	if c.User.Admin == 0 {
		c.currentMenu = models.MenuMain
		handleMain(c)
		return
	}

	var reply string

	rows, err := db.Query(`
		SELECT 
			u.tgid,
			u.first_name,
			u.last_name			
		FROM users u
		WHERE
			u.status=?;`, models.UserRequested)
	if err != nil {
		msg := tgbotapi.NewMessage(c.ChatID, "Internal error. Can't select users for approving from db")
		msg.ReplyToMessageID = c.MessageID
		bot.Send(msg)

		handleUsersView(c)
	}
	defer rows.Close()

	var xs []dbUsers
	var u dbUsers

	for rows.Next() {
		rows.Scan(&u.TelegramID, &u.FirstName, &u.LastName)
		xs = append(xs, u)
	}

	reply = fmt.Sprintf("We have <b>%v</b> users requests:", len(xs))
	msg := tgbotapi.NewMessage(c.ChatID, reply)
	msg.ParseMode = "HTML"
	bot.Send(msg)

	for key, val := range xs {
		reply = fmt.Sprintf(`#%v <a href="tg://user?id=%v">%v %v</a>`, key+1, val.TelegramID, val.FirstName, val.LastName)
		msg := tgbotapi.NewMessage(c.ChatID, reply)
		msg.ParseMode = "HTML"
		bot.Send(msg)
	}
}

func handleUsersViewBanned(c *UserCache) {

	if c.User.Admin == 0 {
		c.currentMenu = models.MenuMain
		handleMain(c)
		return
	}

	var reply string

	rows, err := db.Query(`
		SELECT 
			u.tgid,			
			u.first_name,
			u.last_name			
		FROM users u
		WHERE
			u.status=?;`, models.UserBanned)
	if err != nil {
		msg := tgbotapi.NewMessage(c.ChatID, "Internal error. Can't select banned users from db")
		msg.ReplyToMessageID = c.MessageID
		bot.Send(msg)

		handleUsersView(c)
	}
	defer rows.Close()

	var xs []dbUsers
	var u dbUsers

	for rows.Next() {
		rows.Scan(&u.TelegramID, &u.FirstName, &u.LastName)
		xs = append(xs, u)
	}

	reply = fmt.Sprintf("We have <b>%v</b> banned users:", len(xs))
	msg := tgbotapi.NewMessage(c.ChatID, reply)
	msg.ParseMode = "HTML"
	bot.Send(msg)

	for key, val := range xs {
		reply = fmt.Sprintf(`#%v <a href="tg://user?id=%v">%v %v</a>`, key+1, val.TelegramID, val.FirstName, val.LastName)
		msg := tgbotapi.NewMessage(c.ChatID, reply)
		msg.ParseMode = "HTML"
		bot.Send(msg)
	}
}

func handleUsersEdit(c *UserCache) {

	if c.User.Admin == 0 {
		c.currentMenu = models.MenuMain
		handleMain(c)
		return
	}

	c.currentMenu = models.MenuUsersEdit
	c.editingUserIndx = 0

	btnRow1 := tgbotapi.NewKeyboardButtonRow(buttons.Back)
	btnRow2 := tgbotapi.NewKeyboardButtonRow(buttons.Approve, buttons.Ban, buttons.Unban)

	markup := tgbotapi.NewReplyKeyboard(btnRow1, btnRow2)
	markup.Selective = true

	msg := tgbotapi.NewMessage(c.ChatID, "View users:")
	msg.ReplyMarkup = markup

	bot.Send(msg)
}

func handleUsersEditApprove(c *UserCache) {

	if c.User.Admin == 0 {
		c.currentMenu = models.MenuMain
		handleMain(c)
		return
	}

	c.currentMenu = models.MenuUsersEditApprove

	editingUser := c.editingUserIndx
	users := c.users

	var u dbUsers
	var msgText string

	doAction := true

	if editingUser == 0 {
		doAction = false

		rows, err := db.Query(`
			SELECT
				u.id,
				u.tgid,
				u.first_name,
				u.last_name
			FROM
				users u
			WHERE
				u.status=?
				AND u.tgid!=?	 
			ORDER BY
				u.id`, models.UserRequested, c.User.TelegramID)
		if err != nil {
			msg := tgbotapi.NewMessage(c.ChatID, "Something went wrong while taking approving list :(")
			msg.ReplyToMessageID = c.MessageID
			bot.Send(msg)
			return
		}
		defer rows.Close()

		users = make(map[int]dbUsers)

		i := 1
		for rows.Next() {
			rows.Scan(&u.ID, &u.TelegramID, &u.FirstName, &u.LastName)
			users[i] = u
			i++
		}

		if len(users) == 0 {
			msg := tgbotapi.NewMessage(c.ChatID, "I have no users to approve for now")
			bot.Send(msg)
			handleUsersEdit(c)
			return
		}

		editingUser = 1
		c.users = users
		c.editingUserIndx = editingUser
	}

	if doAction {
		msgText = c.Text
	} else {
		msgText = ""
	}

	switch msgText {
	case models.Approve:

		u := users[editingUser]

		stmt, err := db.Prepare(`
			UPDATE
				users
			SET
				status=?,
				changed_at=?,
				changed_by=?
			WHERE
				tgid=? 
			`)
		if err != nil {
			msg := tgbotapi.NewMessage(c.ChatID, "Something went wrong while approving user :(")
			msg.ReplyToMessageID = c.MessageID
			bot.Send(msg)
			handleUsersEdit(c)
			return
		}

		timeNow := time.Now().UTC()
		_, err = stmt.Exec(models.UserApprowed, timeNow, c.User.TelegramID, u.TelegramID)
		if err != nil {
			msg := tgbotapi.NewMessage(c.ChatID, "Something went wrong while approving user :(")
			msg.ReplyToMessageID = c.MessageID
			bot.Send(msg)
			handleUsersEdit(c)
			return
		}

		reply := fmt.Sprintf(`Your account has been <b>approved</b> by <a href="tg://user?id=%v">%v %v</a> at %v`, c.User.TelegramID, c.User.FirstName, c.User.LastName, timeNow)
		msg := tgbotapi.NewMessage(int64(u.TelegramID), reply)
		msg.ParseMode = "HTML"
		bot.Send(msg)

		reply = fmt.Sprintf("Account %v %v has been <b>approved</b>", u.FirstName, u.LastName)
		msg = tgbotapi.NewMessage(c.ChatID, reply)
		msg.ParseMode = "HTML"
		bot.Send(msg)

		editingUser++

		if editingUser > len(users) {
			c.editingUserIndx = 0
			c.currentMenu = models.MenuUsersEdit

			msg := tgbotapi.NewMessage(c.ChatID, "No more users to approve. It was last one")
			bot.Send(msg)

			handleUsersEdit(c)
			return
		}

		c.editingUserIndx = editingUser

	case models.Next:
		editingUser++

		if editingUser > len(users) {
			c.editingUserIndx = 0
			c.currentMenu = models.MenuUsersEdit

			msg := tgbotapi.NewMessage(c.ChatID, "No more users to approve. It was last one")
			bot.Send(msg)

			handleUsersEdit(c)
			return
		}
		c.editingUserIndx = editingUser
	}

	u = users[editingUser]

	reply := fmt.Sprintf(`You moderating: <a href="tg://user?id=%v">%v %v</a>`, u.TelegramID, u.FirstName, u.LastName)
	msg := tgbotapi.NewMessage(c.ChatID, reply)
	msg.ParseMode = "HTML"

	btnRow1 := tgbotapi.NewKeyboardButtonRow(buttons.Back)
	btnRow2 := tgbotapi.NewKeyboardButtonRow(buttons.Approve, buttons.Next)

	markup := tgbotapi.NewReplyKeyboard(btnRow1, btnRow2)
	markup.Selective = true
	msg.ReplyMarkup = markup

	bot.Send(msg)
}

func handleUsersEditBan(c *UserCache) {

	if c.User.Admin == 0 {
		c.currentMenu = models.MenuMain
		handleMain(c)
		return
	}

	c.currentMenu = models.MenuUsersEditBan

	cache := c
	currentUser := cache.editingUserIndx
	users := cache.users

	var u dbUsers
	var msgText string
	var reply string

	doAction := true

	if currentUser == 0 {
		doAction = false

		rows, err := db.Query(`
			SELECT
				u.id,
				u.tgid,
				u.first_name,
				u.last_name,
				u.status				
			FROM
				users u
			WHERE
				(u.status=? OR u.status=?)
				AND u.tgid!=?
			ORDER BY
				u.id`, models.UserRequested, models.UserApprowed, c.User.TelegramID)
		if err != nil {
			msg := tgbotapi.NewMessage(c.ChatID, "Something went wrong while taking users list :(")
			msg.ReplyToMessageID = c.MessageID
			bot.Send(msg)
			return
		}
		defer rows.Close()

		users = make(map[int]dbUsers)

		i := 1
		for rows.Next() {
			rows.Scan(&u.ID, &u.TelegramID, &u.FirstName, &u.LastName, &u.Status)
			users[i] = u
			i++
		}

		if len(users) == 0 {
			msg := tgbotapi.NewMessage(c.ChatID, "I have no users to ban for now")
			bot.Send(msg)
			handleUsersEdit(c)
			return
		}

		currentUser = 1
		c.users = users
		c.editingUserIndx = currentUser
	}

	if doAction {
		msgText = c.Text
	} else {
		msgText = ""
	}

	switch msgText {
	case models.Ban:

		u := users[currentUser]

		stmt, err := db.Prepare(`
			UPDATE
				users
			SET
				status=?,
				changed_at=?,
				changed_by=?
			WHERE
				tgid=? 
			`)
		if err != nil {
			msg := tgbotapi.NewMessage(c.ChatID, "Something went wrong while banning user :(")
			msg.ReplyToMessageID = c.MessageID
			bot.Send(msg)
			handleUsersEdit(c)
			return
		}

		timeNow := time.Now().UTC()
		_, err = stmt.Exec(models.UserBanned, timeNow, c.User.TelegramID, u.TelegramID)
		if err != nil {
			msg := tgbotapi.NewMessage(c.ChatID, "Something went wrong while banning user :(")
			msg.ReplyToMessageID = c.MessageID
			bot.Send(msg)
			handleUsersEdit(c)
			return
		}

		if u.Status != models.UserApprowed {
			reply = fmt.Sprintf(`Unfortunately your request has been <b>declined</b> by <a href="tg://user?id=%v">%v %v</a> at %v. Try to text to admin`, c.User.TelegramID, c.User.FirstName, c.User.LastName, timeNow)
		} else {
			reply = fmt.Sprintf(`Unfortunately your account has been <b>banned</b> by <a href="tg://user?id=%v">%v %v</a> at %v. Try to text to admin`, c.User.TelegramID, c.User.FirstName, c.User.LastName, timeNow)
		}

		msg := tgbotapi.NewMessage(int64(u.TelegramID), reply)
		msg.ParseMode = "HTML"
		bot.Send(msg)

		reply = fmt.Sprintf("Account %v %v has been <b>banned</b>", u.FirstName, u.LastName)
		msg = tgbotapi.NewMessage(c.ChatID, reply)
		msg.ParseMode = "HTML"
		bot.Send(msg)

		currentUser++

		if currentUser > len(users) {
			c.editingUserIndx = 0
			c.currentMenu = models.MenuUsersEdit

			msg := tgbotapi.NewMessage(c.ChatID, "No more users to approve. It was last one")
			bot.Send(msg)

			handleUsersEdit(c)
			return
		}

		c.editingUserIndx = currentUser

	case models.Next:
		currentUser++

		if currentUser > len(users) {
			c.editingUserIndx = 0
			c.currentMenu = models.MenuUsersEdit

			msg := tgbotapi.NewMessage(c.ChatID, "No more users to ban. It was last one")
			bot.Send(msg)

			handleUsersEdit(c)
			return
		}
		c.editingUserIndx = currentUser
	}

	u = users[currentUser]

	reply = fmt.Sprintf(`You moderating: <a href="tg://user?id=%v">%v %v</a>`, u.TelegramID, u.FirstName, u.LastName)
	msg := tgbotapi.NewMessage(c.ChatID, reply)
	msg.ParseMode = "HTML"

	btnRow1 := tgbotapi.NewKeyboardButtonRow(buttons.Back)
	btnRow2 := tgbotapi.NewKeyboardButtonRow(buttons.Ban, buttons.Next)

	markup := tgbotapi.NewReplyKeyboard(btnRow1, btnRow2)
	markup.Selective = true
	msg.ReplyMarkup = markup

	bot.Send(msg)
}

func handleUsersEditUnban(c *UserCache) {

	if c.User.Admin == 0 {
		c.currentMenu = models.MenuMain
		handleMain(c)
		return
	}

	c.currentMenu = models.MenuUsersEditUnban

	cache := c
	currentUser := cache.editingUserIndx
	users := cache.users

	var u dbUsers
	var msgText string

	doAction := true

	if currentUser == 0 {
		doAction = false

		rows, err := db.Query(`
			SELECT
				u.id,
				u.tgid,
				u.username,
				u.first_name,
				u.last_name
			FROM
				users u
			WHERE
				u.status=?
				AND u.tgid!=?
			ORDER BY
				u.id`, models.UserBanned, c.User.TelegramID)
		if err != nil {
			msg := tgbotapi.NewMessage(c.ChatID, "Something went wrong while taking ban list :(")
			msg.ReplyToMessageID = c.MessageID
			bot.Send(msg)
			return
		}
		defer rows.Close()

		users = make(map[int]dbUsers)

		i := 1
		for rows.Next() {
			rows.Scan(&u.ID, &u.TelegramID, &u.FirstName, &u.LastName)
			users[i] = u
			i++
		}

		if len(users) == 0 {
			msg := tgbotapi.NewMessage(c.ChatID, "I have no users to unban for now")
			bot.Send(msg)
			handleUsersEdit(c)
			return
		}

		currentUser = 1
		c.users = users
		c.editingUserIndx = currentUser
	}

	if doAction {
		msgText = c.Text
	} else {
		msgText = ""
	}

	switch msgText {
	case models.Unban:

		u := users[currentUser]

		stmt, err := db.Prepare(`
			UPDATE
				users
			SET
				status=?,	  
				changed_at=?,
				changed_by=?				
			WHERE
				tgid=? 
			`)
		if err != nil {
			msg := tgbotapi.NewMessage(c.ChatID, "Something went wrong while unbanning user :(")
			msg.ReplyToMessageID = c.MessageID
			bot.Send(msg)
			handleUsersEdit(c)
			return
		}

		timeNow := time.Now().UTC()
		_, err = stmt.Exec(models.UserApprowed, timeNow, c.User.TelegramID, u.TelegramID)
		if err != nil {
			msg := tgbotapi.NewMessage(c.ChatID, "Something went wrong while unbanning user :(")
			msg.ReplyToMessageID = c.MessageID
			bot.Send(msg)
			handleUsersEdit(c)
			return
		}

		reply := fmt.Sprintf(`Your account has been <b>unbanned</b> by <a href="tg://user?id=%v">%v %v</a> at %v. Try to text to admin`, c.User.TelegramID, c.User.FirstName, c.User.LastName, timeNow)
		msg := tgbotapi.NewMessage(int64(u.TelegramID), reply)
		msg.ParseMode = "HTML"
		bot.Send(msg)

		reply = fmt.Sprintf("Account %v %v has been <b>unbanned</b>", u.FirstName, u.LastName)
		msg = tgbotapi.NewMessage(c.ChatID, reply)
		msg.ParseMode = "HTML"
		bot.Send(msg)

		currentUser++

		if currentUser > len(users) {
			c.editingUserIndx = 0
			c.currentMenu = models.MenuUsersEdit

			msg := tgbotapi.NewMessage(c.ChatID, "No more users to unban. It was last one")
			bot.Send(msg)

			handleUsersEdit(c)
			return
		}

		c.editingUserIndx = currentUser

	case models.Next:
		currentUser++

		if currentUser > len(users) {
			c.editingUserIndx = 0
			c.currentMenu = models.MenuUsersEdit

			msg := tgbotapi.NewMessage(c.ChatID, "No more users to unban. It was last one")
			bot.Send(msg)

			handleUsersEdit(c)
			return
		}
		c.editingUserIndx = currentUser
	}

	u = users[currentUser]

	reply := fmt.Sprintf(`You moderating: <a href="tg://user?id=%v">%v %v</a>`, u.TelegramID, u.FirstName, u.LastName)
	msg := tgbotapi.NewMessage(c.ChatID, reply)
	msg.ParseMode = "HTML"

	btnRow1 := tgbotapi.NewKeyboardButtonRow(buttons.Back)
	btnRow2 := tgbotapi.NewKeyboardButtonRow(buttons.Unban, buttons.Next)

	markup := tgbotapi.NewReplyKeyboard(btnRow1, btnRow2)
	markup.Selective = true
	msg.ReplyMarkup = markup

	bot.Send(msg)
}

//func handleTasks(c *UserCache) {
//
//	c.currentMenu = models.MenuToMe
//	c.editingUserIndx = 0
//	c.NewTask = nil
//
//	row1 := tgbotapi.NewKeyboardButtonRow(buttons.Back)
//	row2 := tgbotapi.NewKeyboardButtonRow(buttons.View, buttons.New)
//	markup := tgbotapi.NewReplyKeyboard(row1, row2)
//
//	msg := tgbotapi.NewMessage(c.ChatID, "Inbox:")
//	msg.ReplyMarkup = markup
//	bot.Send(msg)
//
//}

func handleInbox(c *UserCache) {

	c.currentMenu = models.MenuInbox
	c.editingUserIndx = 0
	c.NewTask = nil

	row1 := tgbotapi.NewKeyboardButtonRow(buttons.Back)
	row2 := tgbotapi.NewKeyboardButtonRow(buttons.New, buttons.Uncompleted, buttons.Completed)
	markup := tgbotapi.NewReplyKeyboard(row1, row2)

	msg := tgbotapi.NewMessage(c.ChatID, "Inbox:")
	msg.ReplyMarkup = markup
	bot.Send(msg)

}

func handleSent(c *UserCache) {

	c.currentMenu = models.MenuInbox
	c.editingUserIndx = 0
	c.NewTask = nil

	row1 := tgbotapi.NewKeyboardButtonRow(buttons.Back)
	row2 := tgbotapi.NewKeyboardButtonRow(buttons.New, buttons.Uncompleted, buttons.Completed)
	markup := tgbotapi.NewReplyKeyboard(row1, row2)

	msg := tgbotapi.NewMessage(c.ChatID, "Inbox:")
	msg.ReplyMarkup = markup
	bot.Send(msg)

}

func handleInboxNew(c *UserCache) {

	c.currentMenu = models.MenuInboxNew

	editingTask := c.editingTaskIndx
	tasks := c.tasks

	var t dbTasks
	var msgText string

	doAction := true

	if editingTask == 0 {
		doAction = false

		rows, err := db.Query(`
		SELECT
			t.ID,
			t.title,
			t.description,
			t.changed_at			
		FROM tasks t
		WHERE
			t.to_user=?
			AND t.status=?
		ORDER BY
			t.id`, c.User.TelegramID, models.New)
		if err != nil {
			msg := tgbotapi.NewMessage(c.ChatID, "Something went wrong while selecting new tasks")
			msg.ParseMode = "HTML"
			bot.Send(msg)

			c.Text = ""
			handleInbox(c)
			return
		}
		defer rows.Close()

		tasks = make(map[int]dbTasks)

		i := 1
		for rows.Next() {
			rows.Scan(&t.ID, &t.Title, &t.Description, &t.ChangedAt)

			tasks[i] = t
			i++
		}

		if len(tasks) == 0 {
			msg := tgbotapi.NewMessage(c.ChatID, "I have no new tasks")
			bot.Send(msg)
			handleInbox(c)
			return
		}

		editingTask = 1
		c.tasks = tasks
		c.editingTaskIndx = editingTask
	}

	if doAction {
		msgText = c.Text
	} else {
		msgText = ""
		c.TaskID = tasks[editingTask].ID
		showTask(c)
		return
	}

	switch msgText {
	case models.Next:

		editingTask++

		if editingTask > len(tasks) {
			c.editingTaskIndx = 0
			c.currentMenu = models.MenuInbox

			msg := tgbotapi.NewMessage(c.ChatID, "No more new tasks. It was last one")
			bot.Send(msg)

			handleInbox(c)
			return
		}

		c.editingTaskIndx = editingTask
		c.TaskID = tasks[editingTask].ID

		showTask(c)
	}
}

func handleInboxCompleted(c *UserCache) {

}

func handleTasksViewFromMe(c *UserCache) {

}

func handleNew(c *UserCache) {

	c.currentMenu = models.MenuNew
	c.editingUserIndx = 0

	if c.NewTask == nil {
		c.NewTask = &task{}
	}

	switch c.NewTask.Step {
	case models.NewTaskStepUser:
		switch c.Text {
		case models.Cancel:
			c.NewTask = &task{}
			c.currentMenu = models.MenuMain
			handleMain(c)
			return
		case models.New, "":
			rows, err := db.Query(`
			SELECT 
				u.tgid,
				u.first_name,
				u.last_name
			FROM users u
			WHERE
				u.approved=1
				AND u.banned=0`)
			if err != nil {
				c.currentMenu = models.MenuMain
				c.editingUserIndx = 0
				c.NewTask = nil

				reply := "Something went wrong while selecting users. Can't start new task"
				msg := tgbotapi.NewMessage(c.ChatID, reply)
				msg.ReplyToMessageID = c.MessageID
				bot.Send(msg)
				handleMain(c)
				return
			}
			defer rows.Close()

			var u dbUsers
			var users = make(map[int]dbUsers)

			i := 1
			for rows.Next() {
				rows.Scan(&u.TelegramID, &u.FirstName, &u.LastName)

				users[i] = u
				i++
			}

			c.users = users

			var btnRow []tgbotapi.KeyboardButton
			var keyboard [][]tgbotapi.KeyboardButton

			btnRow = append(btnRow, buttons.Cancel)

			for key, val := range users {

				btn := tgbotapi.NewKeyboardButton(fmt.Sprintf("%v | %v %v", key, val.FirstName, val.LastName))
				btnRow = append(btnRow, btn)

				if len(btnRow) == 2 {
					keyboard = append(keyboard, btnRow)
					btnRow = nil
				}
			}

			if len(btnRow) > 0 {
				keyboard = append(keyboard, btnRow)
				btnRow = nil
			}

			markup := tgbotapi.NewReplyKeyboard(keyboard[:]...)
			markup.Selective = true

			reply := "Choose user:"
			msg := tgbotapi.NewMessage(c.ChatID, reply)
			msg.ReplyMarkup = markup
			bot.Send(msg)
			return
		default:
			xs := strings.Split(c.Text, " | ")
			if len(xs) == 0 {
				fmt.Println("0")
				return
			}

			toUserIndx, err := strconv.Atoi(xs[0])
			if err != nil {
				fmt.Println("1")
				return
			}

			users := c.users

			if _, ok := users[toUserIndx]; !ok {
				fmt.Println("1")
				return
			}

			toUser := users[toUserIndx]
			c.NewTask.ToUser = &toUser
			c.NewTask.Step = models.NewTaskStepTitle

			row1 := tgbotapi.NewKeyboardButtonRow(buttons.Back, buttons.Cancel)
			markup := tgbotapi.NewReplyKeyboard(row1)
			markup.Selective = true

			reply := fmt.Sprintf(`<b>New task</b>
			To user: <a href="tg://user?id=%v">%v %v</a>
			
			Enter task title <i>(then press Enter)</i>:`, toUser.TelegramID, toUser.FirstName, toUser.LastName)
			msg := tgbotapi.NewMessage(c.ChatID, reply)
			msg.ParseMode = "HTML"
			msg.ReplyMarkup = markup
			bot.Send(msg)
			return
		}
	case models.NewTaskStepTitle:
		switch c.Text {
		case models.Cancel:
			c.NewTask = &task{}
			c.currentMenu = models.MenuMain
			handleMain(c)
			return
		case models.Back:
			c.NewTask.ToUser = &dbUsers{}
			c.NewTask.Step = models.NewTaskStepUser

			c.Text = ""
			handleNew(c)
			return
		case "":
			toUser := c.NewTask.ToUser

			row1 := tgbotapi.NewKeyboardButtonRow(buttons.Back, buttons.Cancel)
			markup := tgbotapi.NewReplyKeyboard(row1)
			markup.Selective = true

			reply := fmt.Sprintf(`<b>New task</b>
			To user: <a href="tg://user?id=%v">%v %v</a>
			
			Enter task title <i>(then press Enter)</i>:`, toUser.TelegramID, toUser.FirstName, toUser.LastName)
			msg := tgbotapi.NewMessage(c.ChatID, reply)
			msg.ParseMode = "HTML"
			msg.ReplyMarkup = markup
			bot.Send(msg)
			return
		default:
			c.NewTask.Title = c.Text
			c.NewTask.Step = models.NewTaskStepDescription

			toUser := c.NewTask.ToUser

			row1 := tgbotapi.NewKeyboardButtonRow(buttons.Back, buttons.Cancel)
			markup := tgbotapi.NewReplyKeyboard(row1)
			markup.Selective = true

			reply := fmt.Sprintf(`<b>New task</b>
			To user: <a href="tg://user?id=%v">%v %v</a>
			Title: %v
			
			Enter task description:`, toUser.TelegramID, toUser.FirstName, toUser.LastName, c.Text)

			msg := tgbotapi.NewMessage(c.ChatID, reply)
			msg.ParseMode = "HTML"
			msg.ReplyMarkup = markup
			bot.Send(msg)
			return
		}
	case models.NewTaskStepDescription:
		switch c.Text {
		case models.Cancel:
			c.NewTask = &task{}
			c.currentMenu = models.MenuMain
			handleMain(c)
			return
		case models.Back:
			c.NewTask.Title = ""
			c.NewTask.Step = models.NewTaskStepTitle

			c.Text = ""
			handleNew(c)
			return
		case "":
			toUser := c.NewTask.ToUser

			row1 := tgbotapi.NewKeyboardButtonRow(buttons.Back, buttons.Cancel)
			markup := tgbotapi.NewReplyKeyboard(row1)
			markup.Selective = true

			reply := fmt.Sprintf(`<b>New task</b>
			To user: <a href="tg://user?id=%v">%v %v</a>
			Title: %v
			
			Enter task description:`, toUser.TelegramID, toUser.FirstName, toUser.LastName, c.NewTask.Title)

			msg := tgbotapi.NewMessage(c.ChatID, reply)
			msg.ParseMode = "HTML"
			msg.ReplyMarkup = markup
			bot.Send(msg)
			return
		default:
			c.NewTask.Description = c.Text
			c.NewTask.Step = models.NewTaskStepSaveToDB
			toUser := c.NewTask.ToUser

			row1 := tgbotapi.NewKeyboardButtonRow(buttons.Back, buttons.Cancel, buttons.Save)
			markup := tgbotapi.NewReplyKeyboard(row1)
			markup.Selective = true

			reply := fmt.Sprintf(`<b>New task</b>
			To user: <a href="tg://user?id=%v">%v %v</a>
			Title: %v
			Description: %v`, toUser.TelegramID, toUser.FirstName, toUser.LastName, c.NewTask.Title, c.Text)
			msg := tgbotapi.NewMessage(c.ChatID, reply)
			msg.ParseMode = "HTML"
			msg.ReplyMarkup = markup
			bot.Send(msg)
			return
		}
	case models.NewTaskStepSaveToDB:
		switch c.Text {
		case models.Cancel:
			c.NewTask = &task{}
			c.currentMenu = models.MenuMain
			handleMain(c)
			return
		case models.Back:
			c.NewTask.Description = ""
			c.NewTask.Step = models.NewTaskStepDescription

			c.Text = ""
			handleNew(c)
		case models.Save:

			toUser := c.NewTask.ToUser

			createdAt := time.Now().UTC()

			stmt, err := db.Prepare(`
				INSERT INTO 'tasks'(from_user, to_user, status, changed_at, changed_by, title, description) VALUES(?,?,?,?,?,?,?) `)
			if err != nil {
				msg := tgbotapi.NewMessage(c.ChatID, "Something went wrong while saving task")
				bot.Send(msg)

				handleMain(c)
				return
			}

			res, err := stmt.Exec(c.User.TelegramID, toUser.TelegramID, models.TaskStatusNew, createdAt, c.User.ID, c.NewTask.Title, c.NewTask.Description)
			if err != nil {
				msg := tgbotapi.NewMessage(c.ChatID, "Something went wrong while saving task")
				bot.Send(msg)

				handleMain(c)
				return
			}

			taskID, err := res.LastInsertId()
			if err != nil {
				msg := tgbotapi.NewMessage(c.ChatID, "Something went wrong while saving task")
				bot.Send(msg)

				handleMain(c)
				return
			}

			reply := fmt.Sprintf(`<b>Task #%v</b>
				To user: <a href="tg://user?id=%v">%v %v</a>
				Title: %v
				Description: %v`, taskID, toUser.TelegramID, toUser.FirstName, toUser.LastName, c.NewTask.Title, c.NewTask.Description)

			msg := tgbotapi.NewMessage(c.ChatID, reply)
			msg.ParseMode = "HTML"
			bot.Send(msg)

			if c.User.TelegramID != toUser.TelegramID {
				reply = fmt.Sprintf(`<b>You have new task #%v</b>				
				Title: %v
				Description: %v

				Task manager: <a href="tg://user?id=%v">%v %v</a>
				Created at: %v`, taskID, c.NewTask.Title, c.NewTask.Description, c.User.ID, c.User.FirstName, c.User.LastName, createdAt)
				msg = tgbotapi.NewMessage(int64(toUser.TelegramID), reply)
				msg.ParseMode = "HTML"
				_, err := bot.Send(msg)
				if err != nil {
					fmt.Println(err)
				}
			}

			c.NewTask = &task{}
			c.currentMenu = models.MenuMain

			handleMain(c)
			return

		default:
			return
		}
	default:
		return
	}
}

func handleComment(c *UserCache) {

	stmt, err := db.Prepare(`
		UPDATE 
			tasks
		SET
			comments=?
		WHERE
			id=?;`)
	if err != nil {
		msg := tgbotapi.NewMessage(c.ChatID, "Something went wrong while updating task user :(")
		msg.ReplyToMessageID = c.MessageID
		bot.Send(msg)
		handleUsersEdit(c)
		return
	}

	_, err = stmt.Exec(c.Text, c.TaskID)
	if err != nil {
		msg := tgbotapi.NewMessage(c.ChatID, "Something went wrong while updating task user :(")
		msg.ReplyToMessageID = c.MessageID
		bot.Send(msg)
		handleUsersEdit(c)
		return
	}

	handleMain(c)
}

func getUserByTelegramID(userid int) dbUsers {

	var u dbUsers

	rows, err := db.Query(`
		SELECT
			u.id,
			u.tgid,			
			u.first_name,
			u.last_name,
			u.admin,
			u.status			
		FROM users u
		WHERE
			tgid=?`, userid)
	if err != nil {
		log.Fatal(err.Error())
	}
	defer rows.Close()

	if rows.Next() {
		rows.Scan(&u.ID, &u.TelegramID, &u.FirstName, &u.LastName, &u.Admin, &u.Status)
	}

	return u
}

func handleCallbackQuery(c *UserCache) {

	var err error

	command := c.CallbackData

	if command == "" {
		return
	}

	xs := strings.Split(command, "|")
	do := xs[0]

	switch do {
	case NewUserCancel:
		newUserCancel(c)
	case NewUserRequest:
		newUserAdd(c)
	case NewUserDecline:
		if len(xs) == 2 {
			newUserDecline(c, xs[1])
		}
	case NewUserAccept:
		if len(xs) == 2 {
			newUserAccpet(c, xs[1])
		}
	case models.Start:
		if len(xs) == 2 {
			c.TaskID, err = strconv.Atoi(xs[1])
			if err != nil {
				return
			}
			changeStatus(c, do)
		}
	case models.Complete:
		if len(xs) == 2 {
			c.TaskID, err = strconv.Atoi(xs[1])
			if err != nil {
				return
			}
			changeStatus(c, do)
		}
	case models.Reject:
		if len(xs) == 2 {
			c.TaskID, err = strconv.Atoi(xs[1])
			if err != nil {
				return
			}
			changeStatus(c, do)
		}
	case models.Close:
		if len(xs) == 2 {
			c.TaskID, err = strconv.Atoi(xs[1])
			if err != nil {
				return
			}
			changeStatus(c, do)
		}
	case models.History:
		if len(xs) == 2 {
			c.TaskID, err = strconv.Atoi(xs[1])
			if err != nil {
				return
			}
			showHistory(c)
		}
	case models.Comment:
		if len(xs) == 2 {
			c.TaskID, err = strconv.Atoi(xs[1])
			if err != nil {
				return
			}
			addComment(c)
		}
	}
}

func newUserCancel(c *UserCache) {

	var cbConfig tgbotapi.CallbackConfig

	cbConfig.CallbackQueryID = c.CallbackID
	cbConfig.ShowAlert = true
	cbConfig.Text = "You canceled activation"

	bot.AnswerCallbackQuery(cbConfig)

	reply := fmt.Sprintf("Dear, %s %s. See you next time\nBye!", c.User.FirstName, c.User.LastName)
	updMsg := tgbotapi.NewEditMessageText(c.ChatID, c.MessageID, reply)
	bot.Send(updMsg)
}

func newUserAdd(c *UserCache) {

	var cbConfig tgbotapi.CallbackConfig

	cbConfig.CallbackQueryID = c.CallbackID

	rows, err := db.Query(`
		SELECT 
			u.id
		FROM users u
		WHERE
			u.tgid=?`, c.User.TelegramID)
	if err != nil {
		cbConfig.Text = fmt.Sprintf("Dear, %s %s. sorry, somethings went wrong. Try make request later", c.User.FirstName, c.User.LastName)
		bot.AnswerCallbackQuery(cbConfig)
		return
	}

	if rows.Next() {
		cbConfig.Text = "You have already made request"
		cbConfig.ShowAlert = true
		bot.AnswerCallbackQuery(cbConfig)

		updMsg := tgbotapi.NewEditMessageText(c.ChatID, c.MessageID, "Keep calm and wait for approval message")
		bot.Send(updMsg)
		return
	}
	rows.Close()

	stmt, err := db.Prepare(`INSERT INTO users(tgid, first_name, last_name, status, changed_by, changed_at) values(?, ?, ?, ?, ?, ?)`)
	if err != nil {
		cbConfig.Text = fmt.Sprintf("Dear, %s %s. sorry, somethings went wrong. Try make request later", c.User.FirstName, c.User.LastName)
		bot.AnswerCallbackQuery(cbConfig)
		return
	}

	res, err := stmt.Exec(c.User.TelegramID, c.User.FirstName, c.User.LastName, models.UserRequested, c.User.TelegramID, time.Now().UTC())
	if err != nil {
		cbConfig.Text = fmt.Sprintf("Dear, %s %s. sorry, somethings went wrong. Try make request later", c.User.FirstName, c.User.LastName)
		bot.AnswerCallbackQuery(cbConfig)
		return
	}

	_, err = res.LastInsertId()
	if err != nil {
		cbConfig.Text = fmt.Sprintf("Dear, %s %s. sorry, somethings went wrong. Try make request later", c.User.FirstName, c.User.LastName)
		bot.AnswerCallbackQuery(cbConfig)
		return
	}

	//notifyNewUserRequest(int(userID))

	cbConfig.Text = ""
	bot.AnswerCallbackQuery(cbConfig)

	var reply string

	rows, err = db.Query(`
		SELECT
			u.tgid
		FROM users u
		WHERE
			u.admin=1`)
	if err != nil {
		reply = fmt.Sprintf("Dear, %s %s. Ok, wait for approval message!", c.User.FirstName, c.User.LastName)
	} else {
		reply = fmt.Sprintf("Dear, %s %s. I made request to Taskeram admins to approve your account\nWait for approval message!", c.User.FirstName, c.User.LastName)
	}
	defer rows.Close()

	var u dbUsers
	for rows.Next() {
		rows.Scan(&u.TelegramID)

		btnAccept := tgbotapi.NewInlineKeyboardButtonData("Accept", fmt.Sprintf("%v|%v", NewUserAccept, c.User.TelegramID))
		btnDecline := tgbotapi.NewInlineKeyboardButtonData("Decline", fmt.Sprintf("%v|%v", NewUserDecline, c.User.TelegramID))
		btnRow1 := tgbotapi.NewInlineKeyboardRow(btnAccept, btnDecline)
		markup := tgbotapi.NewInlineKeyboardMarkup(btnRow1)

		text := fmt.Sprintf(`User: <a href="tg://user?id=%v">%v %v</a> made approve request`, c.User.TelegramID, c.User.FirstName, c.User.LastName)
		msg := tgbotapi.NewMessage(int64(u.TelegramID), text)
		msg.ParseMode = "HTML"
		msg.ReplyMarkup = markup
		bot.Send(msg)
	}

	updMsg := tgbotapi.NewEditMessageText(c.ChatID, c.MessageID, reply)
	bot.Send(updMsg)
}

func newUserDecline(c *UserCache, ID string) {

	var cbConfig tgbotapi.CallbackConfig

	userID, err := strconv.Atoi(ID)
	if err != nil {
		msg := tgbotapi.NewMessage(c.ChatID, "Sorry, something went wrong")
		bot.Send(msg)
		return
	}

	rows, err := db.Query(`
		SELECT 
			u.tgid,
			u.first_name,
			u.last_name,
			u.status,
			u.changed_at
		FROM users u
		WHERE
			u.tgid=?
			AND u.status!=?`, userID, models.UserRequested)
	if err != nil {
		msg := tgbotapi.NewMessage(c.ChatID, "Sorry, something went wrong")
		bot.Send(msg)
		return
	}
	defer rows.Close()

	if rows.Next() {
		var u dbUsers

		rows.Scan(&u.TelegramID, &u.FirstName, &u.LastName, &u.Status, &u.ChangedAt)

		reply := fmt.Sprintf(`User: <a href="tg://user?id=%v">%v %v</a>
		<i>current status:</i> %v
		<i>changed at:</i> %v`, u.TelegramID, u.FirstName, u.LastName, u.Status, u.ChangedAt)

		msg := tgbotapi.NewEditMessageText(c.ChatID, c.MessageID, reply)
		msg.ParseMode = "HTML"
		bot.Send(msg)

		cbConfig.CallbackQueryID = c.CallbackID
		cbConfig.Text = ""
		bot.AnswerCallbackQuery(cbConfig)
		return
	}

	stmt, err := db.Prepare(`
		UPDATE 
			users
		SET 
			status=?,
			changed_by=?,
			changed_at=?			
		WHERE
			tgid=?`)

	if err != nil {
		return
	}

	timeNow := time.Now().UTC()
	_, err = stmt.Exec(models.UserBanned, c.User.TelegramID, timeNow, userID)

	reply := fmt.Sprintf(`Unfortunately your request has been <b>declined</b> by <a href="tg://user?id=%v">%v %v</a> at %v. Try to text to admin`, c.User.TelegramID, c.User.FirstName, c.User.LastName, timeNow)
	msg := tgbotapi.NewMessage(int64(userID), "Sorry, your request was denied")
	bot.Send(msg)

	reply = fmt.Sprintf(`<a href="tg://user?id=%v">User</a> has been <b>banned</b>`, userID)
	msgEdited := tgbotapi.NewEditMessageText(c.ChatID, c.MessageID, reply)
	msgEdited.ParseMode = "HTML"
	_, err = bot.Send(msgEdited)
	if err != nil {
		log.Println(err)
	}

	cbConfig.CallbackQueryID = c.CallbackID
	cbConfig.Text = ""
	bot.AnswerCallbackQuery(cbConfig)
}

func newUserAccpet(c *UserCache, ID string) {

	var cbConfig tgbotapi.CallbackConfig

	userID, err := strconv.Atoi(ID)
	if err != nil {
		msg := tgbotapi.NewMessage(c.ChatID, "Sorry, something went wrong")
		bot.Send(msg)
		return
	}

	rows, err := db.Query(`
		SELECT 
			u.tgid,
			u.first_name,
			u.last_name,
			u.status,
			u.changed_at
		FROM users u
		WHERE
			u.tgid=?
			AND u.status!=?`, userID, models.UserRequested)
	if err != nil {
		msg := tgbotapi.NewMessage(c.ChatID, "Sorry, something went wrong")
		bot.Send(msg)
		return
	}
	defer rows.Close()

	if rows.Next() {
		var u dbUsers

		rows.Scan(&u.TelegramID, &u.FirstName, &u.LastName, &u.Status, &u.ChangedAt)

		reply := fmt.Sprintf(`User: <a href="tg://user?id=%v">%v %v</a>
		<i>current status:</i> %v
		<i>changed at:</i> %v`, u.TelegramID, u.FirstName, u.LastName, u.Status, u.ChangedAt)

		msg := tgbotapi.NewEditMessageText(c.ChatID, c.MessageID, reply)
		msg.ParseMode = "HTML"
		bot.Send(msg)

		cbConfig.CallbackQueryID = c.CallbackID
		cbConfig.Text = ""
		bot.AnswerCallbackQuery(cbConfig)
		return
	}

	stmt, err := db.Prepare(`
		UPDATE 
			users
		SET 
			status=?,
			changed_by=?,
			changed_at=?			
		WHERE
			tgid=?`)

	if err != nil {
		return
	}

	timeNow := time.Now().UTC()
	_, err = stmt.Exec(models.UserApprowed, c.User.TelegramID, timeNow, userID)

	reply := fmt.Sprintf(`Your account has been <b>approved</b> by <a href="tg://user?id=%v">%v %v</a> at %v`, c.User.TelegramID, c.User.FirstName, c.User.LastName, timeNow)
	msg := tgbotapi.NewMessage(int64(userID), reply)
	msg.ParseMode = "HTML"
	bot.Send(msg)

	reply = fmt.Sprintf(`<a href="tg://user?id=%v">User</a> has been <b>approved</b>`, userID)
	msgEdited := tgbotapi.NewEditMessageText(c.ChatID, c.MessageID, reply)
	msgEdited.ParseMode = "HTML"
	_, err = bot.Send(msgEdited)
	if err != nil {
		log.Println(err)
	}

	cbConfig.CallbackQueryID = c.CallbackID
	cbConfig.Text = ""
	bot.AnswerCallbackQuery(cbConfig)
}

func changeStatus(c *UserCache, newStatus string) {

	var t dbTasks
	var cbConfig tgbotapi.CallbackConfig

	tguID := c.User.TelegramID

	rows, err := db.Query(`
		SELECT			
			t.id,
			t.status,
			t.to_user,
			t.from_user
		FROM tasks t
		WHERE 
			t.id=?
			AND (t.to_user=? OR t.from_user=?)`, c.TaskID, tguID, tguID)
	if err != nil {
		cbConfig.Text = "Something went wrong while checking current task status"
		cbConfig.ShowAlert = true
		cbConfig.CallbackQueryID = c.CallbackID
		bot.AnswerCallbackQuery(cbConfig)
		return
	}

	if rows.Next() {
		rows.Scan(&t.ID, &t.Status, &t.ToUser, &t.FromUser)
	} else {
		return
	}
	rows.Close()

	var taskType string

	if t.ToUser == tguID {
		taskType = "Inbox"
	} else {
		taskType = "Sent"
	}

	rule := taskRules[taskType][t.Status]

	if !rule.Contains(newStatus) {
		cbConfig.Text = fmt.Sprintf("It isn't allowed to change the status to %v for Task #%v", newStatus, t.ID)
		cbConfig.ShowAlert = true
		cbConfig.CallbackQueryID = c.CallbackID
		bot.AnswerCallbackQuery(cbConfig)

		updateTaskInlineKeyboard(c.ChatID, c.MessageID, c.TaskID, taskType, t.Status)
		return
	}

	stmt, err := db.Prepare(`
		UPDATE
			tasks
		SET
			status=?,
			changed_at=?,
			changed_by=?
		WHERE 
			id=?`)
	if err != nil {
		cbConfig.Text = "Something went wrong while updating task status"
		cbConfig.ShowAlert = true
		cbConfig.CallbackQueryID = c.CallbackID
		bot.AnswerCallbackQuery(cbConfig)
		return
	}

	_, err = stmt.Exec(newStatus, time.Now().UTC(), c.User.TelegramID, t.ID)
	if err != nil {
		cbConfig.Text = "Something went wrong while updating task status"
		cbConfig.ShowAlert = true
		cbConfig.CallbackQueryID = c.CallbackID
		bot.AnswerCallbackQuery(cbConfig)
		return
	}

	cbConfig.Text = fmt.Sprintf("Status has been changed to %v for Task %v", newStatus, t.ID)
	cbConfig.CallbackQueryID = c.CallbackID
	bot.AnswerCallbackQuery(cbConfig)

	updateTaskInlineKeyboard(c.ChatID, c.MessageID, t.ID, taskType, newStatus)
}

func updateTaskInlineKeyboard(chatID int64, msgID int, taskID int, taskType string, status string) {
	var btnRow []tgbotapi.InlineKeyboardButton

	rule := taskRules[taskType][status]
	for _, val := range rule {
		btnRow = append(btnRow, tgbotapi.NewInlineKeyboardButtonData(val, fmt.Sprintf("%v|%v", val, taskID)))
	}
	markup := tgbotapi.NewInlineKeyboardMarkup(btnRow)

	inlKbrd := tgbotapi.NewEditMessageReplyMarkup(chatID, msgID, markup)
	_, err := bot.Send(inlKbrd)
	if err != nil {
		log.Println(err)
	}
}

func showTask(c *UserCache) {

	rows, err := db.Query(`
		SELECT
			t.id,
			t.from_user,
			t.to_user,
			t.status,
			t.changed_at,
			t.changed_by,
			t.comments,			
			t.title,
			t.description,			
			t.images,
			t.documents	
		FROM tasks t
		WHERE
			t.id=?
			AND (t.to_user=? OR t.from_user=?)`, c.TaskID, c.User.TelegramID, c.User.TelegramID)
	if err != nil {
		msg := tgbotapi.NewMessage(c.ChatID, "Something went wrong while selecting task info")
		bot.Send(msg)
		return
	}
	defer rows.Close()

	var t dbTasks

	if rows.Next() {
		err = rows.Scan(&t.ID, &t.FromUser, &t.ToUser, &t.Status, &t.ChangedAt, &t.ChangedBy, &t.Comments, &t.Title, &t.Description, &t.Images, &t.Documents)
		if err != nil {
			log.Println(err)
		}
	} else {
		msg := tgbotapi.NewMessage(c.ChatID, fmt.Sprintln("Can't find any task with ID: ", c.TaskID))
		bot.Send(msg)
		return
	}

	if t.ToUser != c.User.TelegramID && t.FromUser != c.User.TelegramID {
		msg := tgbotapi.NewMessage(c.ChatID, "Access denided")
		bot.Send(msg)
		return
	}

	reply := fmt.Sprintf(`<strong>Task #%v</strong>
	<i>status:</i> <b>%v</b> (%v)
	
	<i>title:</i> %v
	<i>description:</i> %v`, t.ID, t.Status, t.ChangedAt.Time, t.Title, t.Description)

	var taskType string
	if t.ToUser == c.User.TelegramID {
		taskType = "Inbox"
	} else if t.FromUser == c.User.TelegramID {
		taskType = "Sent"
	}

	rule := taskRules[taskType][t.Status]

	var btnRow []tgbotapi.InlineKeyboardButton
	for _, val := range rule {
		btnRow = append(btnRow, tgbotapi.NewInlineKeyboardButtonData(val, fmt.Sprintf("%v|%v", val, t.ID)))
	}

	//btnStart := tgbotapi.NewInlineKeyboardButtonData("Start 🚀", fmt.Sprintf("Start|%v", t.ID))
	//btnComplete := tgbotapi.NewInlineKeyboardButtonData("Complete 🏁", fmt.Sprintf("Complete|%v", t.ID))
	//btnHistory := tgbotapi.NewInlineKeyboardButtonData("History 📖", fmt.Sprintf("History|%v", t.ID))
	//btnRow := tgbotapi.NewInlineKeyboardRow(btnStart, btnComplete, btnHistory)
	replyMarkup := tgbotapi.NewInlineKeyboardMarkup(btnRow)

	var msgSent tgbotapi.Message
	if c.currentMessage == 0 {
		msg := tgbotapi.NewMessage(c.ChatID, reply)
		msg.ReplyMarkup = replyMarkup
		msg.ParseMode = "HTML"
		msgSent, err = bot.Send(msg)
	} else {
		msg := tgbotapi.NewEditMessageText(c.ChatID, c.currentMessage, reply)
		//msg.ReplyMarkup = replyMarkup
		msg.ParseMode = "HTML"
		msgSent, err = bot.Send(msg)
	}

	if err != nil {
		log.Println(err)
		c.currentMessage = 0
		return
	}

	c.currentMessage = msgSent.MessageID
}

func showHistory(c *UserCache) {

	var cbConfig tgbotapi.CallbackConfig
	var row History
	var xs []History

	rows, err := db.Query(`
		SELECT 
			h.status,
			h.date,
			h.comments,
			h.taskid,							
			u.tgid,
			u.first_name,
			u.last_name,
			t.title
		FROM
			task_history h
		LEFT JOIN
			users u
			ON h.userid = u.id
		LEFT JOIN
			tasks t 
			ON h.taskid = t.id
		WHERE
			h.taskid=?			
		ORDER BY 
			h.date`, c.TaskID, c.User.TelegramID, c.User.TelegramID)
	if err != nil {
		cbConfig.Text = "Something went wrong while selecting task history"
		cbConfig.ShowAlert = true
		cbConfig.CallbackQueryID = c.CallbackID
		bot.AnswerCallbackQuery(cbConfig)
		return
	}

	for rows.Next() {
		rows.Scan(&row.h.Status, &row.h.Date, &row.h.Comments, &row.h.TaskID, &row.u.TelegramID, &row.u.FirstName, &row.u.LastName, &row.t.Title)
		xs = append(xs, row)
	}
	rows.Close()

	if len(xs) == 0 {
		cbConfig.Text = "There is no history for this task"
		cbConfig.ShowAlert = true
		cbConfig.CallbackQueryID = c.CallbackID
		bot.AnswerCallbackQuery(cbConfig)
		return
	}

	reply := fmt.Sprintf(`<strong>Task #%v</strong>
	<i>Title</i> %v

	`, xs[0].h.TaskID, xs[0].t.Title)

	for key, val := range xs {
		reply += fmt.Sprintf(`%v. <b>%v</b> by <a href="tg://user?id=%v">%v %v</a> at %v`, key+1, val.h.Status, val.u.TelegramID, val.u.FirstName, val.u.LastName, val.h.Date.Time)
		if val.h.Comments != "" {
			reply += fmt.Sprintf(`<v>Comment:</i> %v`, val.h.Comments)
		}
		reply += `

		`
	}

	if c.CallbackID == "" {
		msg := tgbotapi.NewMessage(c.ChatID, reply)
		msg.ParseMode = "HTML"
		_, err = bot.Send(msg)
	} else {
		msgEdited := tgbotapi.NewEditMessageText(c.ChatID, c.MessageID, reply)
		msgEdited.ParseMode = "HTML"
		_, err = bot.Send(msgEdited)
	}

	if err != nil {
		log.Println(err)
	}
}

func addComment(c *UserCache) {

	var t dbTasks
	var cbConfig tgbotapi.CallbackConfig

	tguID := c.User.TelegramID

	rows, err := db.Query(`
		SELECT			
			t.id,
			t.status,
			t.to_user,
			t.from_user
		FROM tasks t
		WHERE 
			t.id=?
			AND (t.to_user=? OR t.from_user=?)`, c.TaskID, tguID, tguID)
	if err != nil {
		cbConfig.Text = "Something went wrong while checking current task status"
		cbConfig.ShowAlert = true
		cbConfig.CallbackQueryID = c.CallbackID
		bot.AnswerCallbackQuery(cbConfig)
		return
	}

	if rows.Next() {
		rows.Scan(&t.ID, &t.Status, &t.ToUser, &t.FromUser)
	} else {
		return
	}
	rows.Close()

	var taskType string

	if t.ToUser == tguID {
		taskType = "Inbox"
	} else {
		taskType = "Sent"
	}

	rule := taskRules[taskType][t.Status]

	if !rule.Contains(models.Comment) {
		cbConfig.Text = fmt.Sprintf("It isn't allowed to comment Task #%v", t.ID)
		cbConfig.ShowAlert = true
		cbConfig.CallbackQueryID = c.CallbackID
		bot.AnswerCallbackQuery(cbConfig)

		updateTaskInlineKeyboard(c.ChatID, c.MessageID, c.TaskID, taskType, t.Status)
		return
	}

	c.currentMenu = models.MenuComment
	reply := fmt.Sprintf(`<strong>Task #%v</strong>
	Enter comment:`)

	msg := tgbotapi.NewMessage(c.ChatID, reply)
	msg.ParseMode = "HTML"
	bot.Send(msg)
}
