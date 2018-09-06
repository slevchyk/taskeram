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
	"github.com/slevchyk/taskeram/models"
	"gopkg.in/telegram-bot-api.v4"
)

var (
	db           *sql.DB
	bot          *tgbotapi.BotAPI
	cache        map[int]*models.UserCache
	taskRules    map[string]map[string]models.AllowedActions
	actionStatus map[string]string
	buttons      models.Buttons
)

func init() {
	var err error

	db, err = sql.Open("sqlite3", "tasker.sqlite")
	if err != nil {
		log.Fatal("Can't connect to DB ", err.Error())
	}

	token := os.Getenv("TELEGRAM_TASKERAM_TOKEN")

	if token == "" {
		log.Fatal("Env variable TELEGRAM_TASKERAM_TOKEN does not exist")
	}

	bot, err = tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Fatal(err)
	}
}

func main() {

	defer db.Close()

	initialization()

	bot.Debug = false
	log.Printf("Authorized on account %s", bot.Self.UserName)

	// инициализируем канал, куда будут прилетать обновления от API
	ucfg := tgbotapi.NewUpdate(0)
	ucfg.Timeout = 60

	upd, _ := bot.GetUpdatesChan(ucfg)
	// читаем обновления из канала
	for {

		update := <-upd

		var tgid int

		if update.Message != nil {
			tgid = update.Message.From.ID
		} else if update.CallbackQuery != nil {
			tgid = update.CallbackQuery.From.ID
		}

		//ми не отримали ID користувача, не можемо його ідентифікувати і на дати доступ для роботи далі
		if tgid == 0 {
			continue
		}

		u := getUserByTelegramID(tgid)

		//Якщо в цього користувача ще не має власних налаштувань сесії, то ініціалізуємо їх
		if _, ok := cache[tgid]; !ok {
			cache[tgid] = &models.UserCache{User: u}
		}

		c := cache[tgid]

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

			//користувач забанений але шось пише боту
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

func initialization() {
	initDB()
	initData()
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
			'comment' TEXT DEFAULT '');`)
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
		CREATE TRIGGER IF NOT EXISTS update_user_history AFTER UPDATE ON users WHEN (old.status <> new.status)
		BEGIN
			INSERT INTO user_history(status, changed_by, changed_at, admin) values (new.status, new.changed_by, new.changed_at, new.admin);
		END;`)
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
			'description' TEXT DEFAULT '',
			'comment' TEXT DEFAULT '',
			'commented_at' DATE,
			'commented_by' INTEGER,
			'images' TEXT DEFAULT '',
			'documents' TEXT DEFAULT '');`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS 'task_history'(
			'id' INTEGER PRIMARY KEY AUTOINCREMENT,
			'taskid' INTEGER REFERENCES tasks,
			'tgid' INTEGER REFERENCES users,
			'date' DATE,
			'status' INTEGER,			
			'comment' TEXT
			);`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TRIGGER IF NOT EXISTS insert_task_history AFTER INSERT ON tasks
		BEGIN
			INSERT INTO task_history(date, status, taskid, tgid) values (new.changed_at, new.status, new.id, new.changed_by);
		END;`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TRIGGER IF NOT EXISTS update_task_history AFTER UPDATE ON tasks WHEN (old.status <> new.status)
		BEGIN
			INSERT INTO task_history(date, status, taskid, tgid) values (new.changed_at, new.status, new.id, new.changed_by);
		END;`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS 'task_comments'(
			'id' INTEGER PRIMARY KEY AUTOINCREMENT,
			'taskid' INTEGER REFERENCES tasks,
			'tgid' INTEGER REFERENCES users,
			'date' DATE,			
			'comment' TEXT
			);`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TRIGGER IF NOT EXISTS update_task_comments AFTER UPDATE ON tasks WHEN (old.comment <> new.comment)
		BEGIN
			INSERT INTO task_comments(taskid, tgid, date, comment) values (new.id, new.commented_by, new.commented_at,  new.comment);
		END;`)
	if err != nil {
		log.Fatal(err)
	}

	envID := os.Getenv("TELEGRAM_TASKERAM_ADMIN")
	if envID == "" {
		log.Fatal("Env variable TELEGRAM_TASKERAM_ADMIN does not exist")
	}

	tgID, err := strconv.Atoi(envID)
	if err != nil {
		log.Fatal(err)
	}

	rows, err := db.Query(`
		SELECT
		u.id
		FROM 
			users u
		WHERE
			u.tgid=?`, tgID)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	if !rows.Next() {
		stmt, err := db.Prepare(`
			INSERT into 'users'(tgid, first_name, last_name, admin, status, changed_at) VALUES (?,?,?,?,?,?)`)
		if err != nil {
			log.Fatal(err)
		}

		_, err = stmt.Exec(tgID, "admin", "admin", 1, models.UserApprowed, time.Now().UTC())
		if err != nil {
			log.Fatal(err)
		}
	}
}

func initData() {

	cache = make(map[int]*models.UserCache)

	taskRules = make(map[string]map[string]models.AllowedActions)
	inboxRules := make(map[string]models.AllowedActions)
	sentRules := make(map[string]models.AllowedActions)

	inboxRules[models.TaskStatusNew] = models.AllowedActions{models.Start, models.Complete, models.Comment, models.History}
	inboxRules[models.TaskStatusStarted] = models.AllowedActions{models.Complete, models.Comment, models.History}
	inboxRules[models.TaskStatusRejected] = models.AllowedActions{models.Complete, models.Comment, models.History}
	inboxRules[models.TaskStatusCompleted] = models.AllowedActions{models.History}
	inboxRules[models.TaskStatusClosed] = models.AllowedActions{models.History}
	taskRules["Inbox"] = inboxRules

	sentRules[models.TaskStatusNew] = models.AllowedActions{models.Close, models.Comment, models.History}
	sentRules[models.TaskStatusStarted] = models.AllowedActions{models.Close, models.Comment, models.History}
	sentRules[models.TaskStatusRejected] = models.AllowedActions{models.Close, models.Comment, models.History}
	sentRules[models.TaskStatusCompleted] = models.AllowedActions{models.Reject, models.Comment, models.Close, models.History}
	sentRules[models.TaskStatusClosed] = models.AllowedActions{models.History}
	taskRules["Sent"] = sentRules

	actionStatus = make(map[string]string)
	actionStatus[models.Start] = models.TaskStatusStarted
	actionStatus[models.Complete] = models.TaskStatusCompleted
	actionStatus[models.Reject] = models.TaskStatusRejected
	actionStatus[models.Close] = models.TaskStatusClosed

	buttons.Main = tgbotapi.NewKeyboardButton(models.Main)
	buttons.Next = tgbotapi.NewKeyboardButton(models.Next)
	buttons.Users = tgbotapi.NewKeyboardButton(models.Users)
	buttons.Back = tgbotapi.NewKeyboardButton(models.Back)
	buttons.View = tgbotapi.NewKeyboardButton(models.View)
	buttons.All = tgbotapi.NewKeyboardButton(models.All)
	buttons.Requests = tgbotapi.NewKeyboardButton(models.Requests)
	buttons.Banned = tgbotapi.NewKeyboardButton(models.Banned)
	buttons.Edit = tgbotapi.NewKeyboardButton(models.Edit)
	buttons.Approve = tgbotapi.NewKeyboardButton(models.Approve)
	buttons.Ban = tgbotapi.NewKeyboardButton(models.Ban)
	buttons.Unban = tgbotapi.NewKeyboardButton(models.Unban)
	buttons.Inbox = tgbotapi.NewKeyboardButton(models.Inbox)
	buttons.Sent = tgbotapi.NewKeyboardButton(models.Sent)
	buttons.New = tgbotapi.NewKeyboardButton(models.New)
	buttons.Started = tgbotapi.NewKeyboardButton(models.TaskStatusStarted)
	buttons.Rejected = tgbotapi.NewKeyboardButton(models.TaskStatusRejected)
	buttons.Completed = tgbotapi.NewKeyboardButton(models.TaskStatusCompleted)
	buttons.Closed = tgbotapi.NewKeyboardButton(models.TaskStatusClosed)
	buttons.Save = tgbotapi.NewKeyboardButton(models.Save)
	buttons.Cancel = tgbotapi.NewKeyboardButton(models.Cancel)
	buttons.Start = tgbotapi.NewKeyboardButton(models.Start)
	buttons.Complete = tgbotapi.NewKeyboardButton(models.Complete)
	buttons.History = tgbotapi.NewKeyboardButton(models.History)
	buttons.Close = tgbotapi.NewKeyboardButton(models.Close)
	buttons.Reject = tgbotapi.NewKeyboardButton(models.Reject)
}

//запропонуємо користувачу зробити запит на активацію в програмі
func serveNewUser(update tgbotapi.Update) {

	ut := update.Message.From

	reply := fmt.Sprintf("Hello, %s %s. I can see you are new one here. Would you like to send request to approve your account in Taskeram?\n", ut.FirstName, ut.LastName)

	btnYes := tgbotapi.NewInlineKeyboardButtonData("✓ Yes", models.NewUserRequest)
	btnNo := tgbotapi.NewInlineKeyboardButtonData("🚫 No", models.NewUserCancel)
	keyboard := tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(btnYes, btnNo))

	msg := tgbotapi.NewMessage(update.Message.Chat.ID, reply)
	msg.ReplyMarkup = &keyboard

	_, err := bot.Send(msg)
	if err != nil {
		log.Println(err)
	}
}

func serveBannedUser(update tgbotapi.Update) {

	ut := update.Message.From

	reply := fmt.Sprintf("Hello, %s %s. I'm so sorry but yor request was declined.\nAsk admins to restore your account", ut.FirstName, ut.LastName)
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, reply)

	_, err := bot.Send(msg)
	if err != nil {
		log.Println(err)
	}
}

func serveNonApprovedUser(update tgbotapi.Update) {

	ut := update.Message.From

	reply := fmt.Sprintf("Hello, %s %s. Keep calm and wait for approval message!\n", ut.FirstName, ut.LastName)
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, reply)

	_, err := bot.Send(msg)
	if err != nil {
		log.Println(err)
	}
}

func serveUser(c *models.UserCache) {

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

		cm := c.CurrentMenu
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
		} else if cm == models.MenuInbox && msg == models.TaskStatusNew {
			handleInboxTasks(c, models.MenuInboxNew, msg)
		} else if cm == models.MenuInbox && msg == models.TaskStatusStarted {
			handleInboxTasks(c, models.MenuInboxStarted, msg)
		} else if cm == models.MenuInbox && msg == models.TaskStatusRejected {
			handleInboxTasks(c, models.MenuInboxRejected, msg)
		} else if cm == models.MenuInbox && msg == models.TaskStatusCompleted {
			handleInboxTasks(c, models.MenuInboxCompleted, msg)
		} else if cm == models.MenuInbox && msg == models.TaskStatusClosed {
			handleInboxTasks(c, models.MenuInboxClosed, msg)
		} else if (cm == models.MenuInboxNew ||
			cm == models.MenuInboxStarted ||
			cm == models.MenuInboxRejected ||
			cm == models.MenuInboxCompleted ||
			cm == models.MenuInboxClosed) &&
			msg == models.Back {
			handleInbox(c)
		} else if cm == models.MenuInboxNew {
			handleInboxTasks(c, cm, models.TaskStatusNew)
		} else if cm == models.MenuInboxStarted {
			handleInboxTasks(c, cm, models.TaskStatusStarted)
		} else if cm == models.MenuInboxRejected {
			handleInboxTasks(c, cm, models.TaskStatusRejected)
		} else if cm == models.MenuInboxCompleted {
			handleInboxTasks(c, cm, models.TaskStatusCompleted)
		} else if cm == models.MenuInboxClosed {
			handleInboxTasks(c, cm, models.TaskStatusClosed)
		} else if cm == models.MenuMain && msg == models.Sent {
			handleSent(c)
		} else if cm == models.MenuSent && msg == models.Back {
			handleMain(c)
		} else if cm == models.MenuSent && msg == models.TaskStatusNew {
			handleSentTasks(c, models.MenuSentNew, msg)
		} else if cm == models.MenuSent && msg == models.TaskStatusStarted {
			handleSentTasks(c, models.MenuSentStarted, msg)
		} else if cm == models.MenuSent && msg == models.TaskStatusRejected {
			handleSentTasks(c, models.MenuSentRejected, msg)
		} else if cm == models.MenuSent && msg == models.TaskStatusCompleted {
			handleSentTasks(c, models.MenuSentCompleted, msg)
		} else if cm == models.MenuSent && msg == models.TaskStatusClosed {
			handleSentTasks(c, models.MenuSentClosed, msg)
		} else if (cm == models.MenuSentNew ||
			cm == models.MenuSentStarted ||
			cm == models.MenuSentRejected ||
			cm == models.MenuSentCompleted ||
			cm == models.MenuSentClosed) &&
			msg == models.Back {
			handleSent(c)
		} else if cm == models.MenuSentNew {
			handleSentTasks(c, cm, models.TaskStatusNew)
		} else if cm == models.MenuSentStarted {
			handleSentTasks(c, cm, models.TaskStatusStarted)
		} else if cm == models.MenuSentRejected {
			handleSentTasks(c, cm, models.TaskStatusRejected)
		} else if cm == models.MenuSentCompleted {
			handleSentTasks(c, cm, models.TaskStatusCompleted)
		} else if cm == models.MenuSentClosed {
			handleSentTasks(c, cm, models.TaskStatusClosed)
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

func handleCommand(c *models.UserCache) {

	switch c.Command {
	case "start":
		handleCommandStart(c)
	case "task":
		handleCommandTask(c)
		return
	case "history":
		handleTaskHistory(c)
		return
	case "comments":
		handleTaskComment(c)
		return
	}
}

func handleCommandStart(c *models.UserCache) {

	//команда старт в нас обробляється тільки для адміністратора
	envID := os.Getenv("TELEGRAM_TASKERAM_ADMIN")
	if envID == "" {
		log.Fatal("Env variable TELEGRAM_TASKERAM_ADMIN does not exist")
	}

	tgID, err := strconv.Atoi(envID)
	if err != nil {
		return
	}

	if c.User.TelegramID != tgID {
		return
	}

	rows, err := db.Query(`
		SELECT
		u.id
		FROM 
			users u
		WHERE
			u.tgid=?`, tgID)
	if err != nil {
		log.Fatal(err)
	}

	if rows.Next() {
		rows.Close()

		stmt, err := db.Prepare(`
			UPDATE 
				users
			SET
				first_name=?,
				last_name=?,
				changed_at=?,
				changed_by=?
			WHERE
				tgid=?
				AND changed_by=0`)
		if err != nil {
			log.Fatal(err)
		}

		_, err = stmt.Exec(c.Message.From.FirstName, c.Message.From.LastName, time.Now().UTC(), tgID, tgID)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func handleCommandTask(c *models.UserCache) {

	var err error
	if c.Arguments == "" {
		msg := tgbotapi.NewMessage(c.ChatID, fmt.Sprintln(c.Arguments, "you should input Task number after /Task command "))
		_, err := bot.Send(msg)
		if err != nil {
			log.Println(err)
		}
		return
	}

	c.TaskID, err = strconv.Atoi(c.Arguments)
	if err != nil {
		msg := tgbotapi.NewMessage(c.ChatID, fmt.Sprintln(c.Arguments, " - wrong argument type"))
		_, err := bot.Send(msg)
		if err != nil {
			log.Println(err)
		}
		return
	}

	c.CurrentMessage = 0

	showTask(c)
}

func handleTaskHistory(c *models.UserCache) {

	var err error

	if c.Arguments == "" {
		msg := tgbotapi.NewMessage(c.ChatID, fmt.Sprintln(c.Arguments, "you should input Task number after /Task command "))
		_, err := bot.Send(msg)
		if err != nil {
			log.Println(err)
		}
		return
	}

	c.TaskID, err = strconv.Atoi(c.Arguments)
	if err != nil {
		msg := tgbotapi.NewMessage(c.ChatID, fmt.Sprintln(c.Arguments, " - wrong argument type"))
		_, err := bot.Send(msg)
		if err != nil {
			log.Println(err)
		}
		return
	}

	c.CallbackID = ""

	showHistory(c)
	handleMain(c)
}

func handleTaskComment(c *models.UserCache) {

	var err error

	if c.Arguments == "" {
		msg := tgbotapi.NewMessage(c.ChatID, fmt.Sprintln(c.Arguments, "you should input Task number after /Task command "))
		_, err := bot.Send(msg)
		if err != nil {
			log.Println(err)
		}
		return
	}

	c.TaskID, err = strconv.Atoi(c.Arguments)
	if err != nil {
		msg := tgbotapi.NewMessage(c.ChatID, fmt.Sprintln(c.Arguments, " - wrong argument type"))
		_, err := bot.Send(msg)
		if err != nil {
			log.Println(err)
		}
		return
	}

	c.CallbackID = ""

	showComments(c)
	handleMain(c)
}

func handleMain(c *models.UserCache) {

	c.CurrentMenu = models.MenuMain
	chatID := c.Message.Chat.ID

	var kbrd [][]tgbotapi.KeyboardButton

	kbrd = append(kbrd, tgbotapi.NewKeyboardButtonRow(buttons.Inbox, buttons.Sent, buttons.New))

	if c.User.Admin == 1 {
		kbrd = append(kbrd, tgbotapi.NewKeyboardButtonRow(buttons.Users))
	}

	markup := tgbotapi.NewReplyKeyboard(kbrd[:]...)
	markup.Selective = true

	msg := tgbotapi.NewMessage(chatID, "Menu: <b>Main</b>")
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = markup
	_, err := bot.Send(msg)
	if err != nil {
		log.Println(err)
	}
}

func handleUsers(c *models.UserCache) {

	if c.User.Admin == 0 {
		c.CurrentMenu = models.MenuMain
		handleMain(c)
		return
	}

	c.CurrentMenu = models.MenuUsers

	btnRow1 := tgbotapi.NewKeyboardButtonRow(buttons.Back)
	btnRow2 := tgbotapi.NewKeyboardButtonRow(buttons.View, buttons.Edit)

	markup := tgbotapi.NewReplyKeyboard(btnRow1, btnRow2)
	markup.Selective = true

	msg := tgbotapi.NewMessage(c.ChatID, "Menu: <b>Users</b>")
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = markup

	_, err := bot.Send(msg)
	if err != nil {
		log.Println(err)
	}
}

func handleUsersView(c *models.UserCache) {

	if c.User.Admin == 0 {
		c.CurrentMenu = models.MenuMain
		handleMain(c)
		return
	}

	c.CurrentMenu = models.MenuUsersView

	btnRow1 := tgbotapi.NewKeyboardButtonRow(buttons.Back)
	btnRow2 := tgbotapi.NewKeyboardButtonRow(buttons.All, buttons.Requests, buttons.Banned)

	markup := tgbotapi.NewReplyKeyboard(btnRow1, btnRow2)
	markup.Selective = true

	msg := tgbotapi.NewMessage(c.ChatID, "Menu: <b>Users->View</b>")
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = markup

	_, err := bot.Send(msg)
	if err != nil {
		log.Println(err)
	}
}

func handleUsersViewALl(c *models.UserCache) {

	if c.User.Admin == 0 {
		c.CurrentMenu = models.MenuMain
		handleMain(c)
		return
	}

	var reply string

	rows, err := db.Query(`
		SELECT 
			u.tgid,			
			u.first_name,
			u.last_name			
		FROM 
			users u
		WHERE
			u.status=?
		ORDER BY
			u.id;`, models.UserApprowed)
	if err != nil {
		msg := tgbotapi.NewMessage(c.ChatID, "Internal error. Can't select users from db")
		msg.ReplyToMessageID = c.MessageID
		_, err := bot.Send(msg)
		if err != nil {
			log.Println(err)
		}

		handleUsersView(c)
	}
	defer rows.Close()

	var xs []models.DbUsers
	var u models.DbUsers

	for rows.Next() {
		err := rows.Scan(&u.TelegramID, &u.FirstName, &u.LastName)
		if err != nil {
			log.Println(err)
		} else {
			xs = append(xs, u)
		}
	}

	reply = fmt.Sprintf("We have <b>%v</b> approved user", len(xs))
	msg := tgbotapi.NewMessage(c.ChatID, reply)
	msg.ParseMode = "HTML"
	_, err = bot.Send(msg)
	if err != nil {
		log.Println(err)
	}

	for key, val := range xs {
		reply = fmt.Sprintf(`#%v <a href="tg://user?id=%v">%v %v</a>`, key+1, val.TelegramID, val.FirstName, val.LastName)
		msg := tgbotapi.NewMessage(c.ChatID, reply)
		msg.ParseMode = "HTML"
		_, err := bot.Send(msg)
		if err != nil {
			log.Println(err)
		}
	}
}

func handleUsersViewRequests(c *models.UserCache) {

	if c.User.Admin == 0 {
		c.CurrentMenu = models.MenuMain
		handleMain(c)
		return
	}

	var reply string

	rows, err := db.Query(`
		SELECT 
			u.tgid,
			u.first_name,
			u.last_name			
		FROM 
			users u
		WHERE
			u.status=?;`, models.UserRequested)
	if err != nil {
		msg := tgbotapi.NewMessage(c.ChatID, "Internal error. Can't select users for approving from db")
		msg.ReplyToMessageID = c.MessageID
		_, err := bot.Send(msg)
		if err != nil {
			log.Println(err)
		}

		handleUsersView(c)
	}
	defer rows.Close()

	var xs []models.DbUsers
	var u models.DbUsers

	for rows.Next() {
		err := rows.Scan(&u.TelegramID, &u.FirstName, &u.LastName)
		if err != nil {
			log.Println(err)
		} else {
			xs = append(xs, u)
		}
	}

	reply = fmt.Sprintf("We have <b>%v</b> users requests:", len(xs))
	msg := tgbotapi.NewMessage(c.ChatID, reply)
	msg.ParseMode = "HTML"
	_, err = bot.Send(msg)
	if err != nil {
		log.Println(err)
	}

	for key, val := range xs {
		reply = fmt.Sprintf(`#%v <a href="tg://user?id=%v">%v %v</a>`, key+1, val.TelegramID, val.FirstName, val.LastName)
		msg := tgbotapi.NewMessage(c.ChatID, reply)
		msg.ParseMode = "HTML"
		_, err := bot.Send(msg)
		if err != nil {
			log.Println(err)
		}
	}
}

func handleUsersViewBanned(c *models.UserCache) {

	if c.User.Admin == 0 {
		c.CurrentMenu = models.MenuMain
		handleMain(c)
		return
	}

	var reply string

	rows, err := db.Query(`
		SELECT 
			u.tgid,			
			u.first_name,
			u.last_name			
		FROM 
			users u
		WHERE
			u.status=?;`, models.UserBanned)
	if err != nil {
		msg := tgbotapi.NewMessage(c.ChatID, "Internal error. Can't select banned users from db")
		msg.ReplyToMessageID = c.MessageID
		_, err := bot.Send(msg)
		if err != nil {
			log.Println(err)
		}

		handleUsersView(c)
	}
	defer rows.Close()

	var xs []models.DbUsers
	var u models.DbUsers

	for rows.Next() {
		err := rows.Scan(&u.TelegramID, &u.FirstName, &u.LastName)
		if err != nil {
			log.Println(err)
		} else {
			xs = append(xs, u)
		}
	}

	reply = fmt.Sprintf("We have <b>%v</b> banned users", len(xs))
	msg := tgbotapi.NewMessage(c.ChatID, reply)
	msg.ParseMode = "HTML"
	_, err = bot.Send(msg)
	if err != nil {
		log.Println(err)
	}

	for key, val := range xs {
		reply = fmt.Sprintf(`#%v <a href="tg://user?id=%v">%v %v</a>`, key+1, val.TelegramID, val.FirstName, val.LastName)
		msg := tgbotapi.NewMessage(c.ChatID, reply)
		msg.ParseMode = "HTML"
		_, err := bot.Send(msg)
		if err != nil {
			log.Println(err)
		}
	}
}

func handleUsersEdit(c *models.UserCache) {

	if c.User.Admin == 0 {
		c.CurrentMenu = models.MenuMain
		handleMain(c)
		return
	}

	c.CurrentMenu = models.MenuUsersEdit
	c.UserSlider.EditingUserIndx = 0

	btnRow1 := tgbotapi.NewKeyboardButtonRow(buttons.Back)
	btnRow2 := tgbotapi.NewKeyboardButtonRow(buttons.Approve, buttons.Ban, buttons.Unban)

	markup := tgbotapi.NewReplyKeyboard(btnRow1, btnRow2)
	markup.Selective = true

	msg := tgbotapi.NewMessage(c.ChatID, "Menu: <b>Users->Edit</b>")
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = markup

	_, err := bot.Send(msg)
	if err != nil {
		log.Println(err)
	}
}

func handleUsersEditApprove(c *models.UserCache) {

	if c.User.Admin == 0 {
		c.CurrentMenu = models.MenuMain
		handleMain(c)
		return
	}

	c.CurrentMenu = models.MenuUsersEditApprove

	editingUser := c.UserSlider.EditingUserIndx
	users := c.UserSlider.Users

	var u models.DbUsers
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
			c.UserSlider.EditingUserIndx = 0
			c.CurrentMenu = models.MenuUsersEdit

			msg := tgbotapi.NewMessage(c.ChatID, "Something went wrong while taking approving list :(")
			msg.ReplyToMessageID = c.MessageID
			_, err := bot.Send(msg)
			if err != nil {
				log.Println(err)
			}
			return
		}
		defer rows.Close()

		users = make(map[int]models.DbUsers)

		i := 1
		for rows.Next() {
			err := rows.Scan(&u.ID, &u.TelegramID, &u.FirstName, &u.LastName)
			if err != nil {
				log.Println(err)
			} else {
				users[i] = u
				i++
			}
		}

		if len(users) == 0 {
			c.UserSlider.EditingUserIndx = 0
			c.CurrentMenu = models.MenuUsersEdit

			msg := tgbotapi.NewMessage(c.ChatID, "You have no users to approve for now")
			_, err := bot.Send(msg)
			if err != nil {
				log.Println(err)
			}

			handleUsersEdit(c)
			return
		}

		editingUser = 1
		c.UserSlider.Users = users
		c.UserSlider.EditingUserIndx = editingUser
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
			c.UserSlider.EditingUserIndx = 0
			c.CurrentMenu = models.MenuUsersEdit

			msg := tgbotapi.NewMessage(c.ChatID, "Something went wrong while approving user :(")
			msg.ReplyToMessageID = c.MessageID
			_, err := bot.Send(msg)
			if err != nil {
				log.Println(err)
			}

			handleUsersEdit(c)
			return
		}

		timeNow := time.Now().UTC()
		_, err = stmt.Exec(models.UserApprowed, timeNow, c.User.TelegramID, u.TelegramID)
		if err != nil {
			c.CurrentMenu = models.MenuUsersEdit

			msg := tgbotapi.NewMessage(c.ChatID, "Something went wrong while approving user :(")
			msg.ReplyToMessageID = c.MessageID
			_, err := bot.Send(msg)
			if err != nil {
				log.Println(err)
			}

			handleUsersEdit(c)
			return
		}

		reply := fmt.Sprintf(`Your account has been <b>approved</b> by <a href="tg://user?id=%v">%v %v</a> at %v`, c.User.TelegramID, c.User.FirstName, c.User.LastName, timeNow)
		msg := tgbotapi.NewMessage(int64(u.TelegramID), reply)
		msg.ParseMode = "HTML"
		_, err = bot.Send(msg)
		if err != nil {
			log.Println(err)
		}

		reply = fmt.Sprintf("Account %v %v has been <b>approved</b>", u.FirstName, u.LastName)
		msg = tgbotapi.NewMessage(c.ChatID, reply)
		msg.ParseMode = "HTML"
		_, err = bot.Send(msg)
		if err != nil {
			log.Println(err)
		}

		editingUser++

		if editingUser > len(users) {
			c.UserSlider.EditingUserIndx = 0
			c.CurrentMenu = models.MenuUsersEdit

			msg := tgbotapi.NewMessage(c.ChatID, "No more users to approve. It was last one")
			_, err := bot.Send(msg)
			if err != nil {
				log.Println(err)
			}

			handleUsersEdit(c)
			return
		}

		c.UserSlider.EditingUserIndx = editingUser

	case models.Next:
		editingUser++

		if editingUser > len(users) {
			c.UserSlider.EditingUserIndx = 0
			c.CurrentMenu = models.MenuUsersEdit

			msg := tgbotapi.NewMessage(c.ChatID, "No more users to approve. It was last one")
			_, err := bot.Send(msg)
			if err != nil {
				log.Println(err)
			}

			handleUsersEdit(c)
			return
		}
		c.UserSlider.EditingUserIndx = editingUser
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

	_, err := bot.Send(msg)
	if err != nil {
		log.Println(err)
	}
}

func handleUsersEditBan(c *models.UserCache) {

	if c.User.Admin == 0 {
		c.CurrentMenu = models.MenuMain
		handleMain(c)
		return
	}

	c.CurrentMenu = models.MenuUsersEditBan

	currentUserIndx := c.UserSlider.EditingUserIndx
	users := c.UserSlider.Users

	var u models.DbUsers
	var msgText string
	var reply string

	doAction := true

	if currentUserIndx == 0 {
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
			c.UserSlider.EditingUserIndx = 0
			c.CurrentMenu = models.MenuUsersEdit

			msg := tgbotapi.NewMessage(c.ChatID, "Something went wrong while taking users list :(")
			msg.ReplyToMessageID = c.MessageID
			_, err := bot.Send(msg)
			if err != nil {
				log.Println(err)
			}

			return
		}
		defer rows.Close()

		users = make(map[int]models.DbUsers)

		i := 1
		for rows.Next() {
			err := rows.Scan(&u.ID, &u.TelegramID, &u.FirstName, &u.LastName, &u.Status)
			if err != nil {
				log.Println(err)
			} else {
				users[i] = u
				i++
			}
		}

		if len(users) == 0 {
			c.UserSlider.EditingUserIndx = 0
			c.CurrentMenu = models.MenuUsersEdit

			msg := tgbotapi.NewMessage(c.ChatID, "You have no users to ban for now")
			_, err := bot.Send(msg)
			if err != nil {
				log.Println(err)
			}

			handleUsersEdit(c)
			return
		}

		currentUserIndx = 1
		c.UserSlider.Users = users
		c.UserSlider.EditingUserIndx = currentUserIndx
	}

	if doAction {
		msgText = c.Text
	} else {
		msgText = ""
	}

	switch msgText {
	case models.Ban:

		u := users[currentUserIndx]

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
			c.UserSlider.EditingUserIndx = 0
			c.CurrentMenu = models.MenuUsersEdit

			msg := tgbotapi.NewMessage(c.ChatID, "Something went wrong while banning user :(")
			msg.ReplyToMessageID = c.MessageID
			_, err := bot.Send(msg)
			if err != nil {
				log.Println(err)
			}

			handleUsersEdit(c)
			return
		}

		timeNow := time.Now().UTC()
		_, err = stmt.Exec(models.UserBanned, timeNow, c.User.TelegramID, u.TelegramID)
		if err != nil {
			c.UserSlider.EditingUserIndx = 0
			c.CurrentMenu = models.MenuUsersEdit

			msg := tgbotapi.NewMessage(c.ChatID, "Something went wrong while banning user :(")
			msg.ReplyToMessageID = c.MessageID
			_, err := bot.Send(msg)
			if err != nil {
				log.Println(err)
			}

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
		_, err = bot.Send(msg)
		if err != nil {
			log.Println(err)
		}

		reply = fmt.Sprintf("Account %v %v has been <b>banned</b>", u.FirstName, u.LastName)
		msg = tgbotapi.NewMessage(c.ChatID, reply)
		msg.ParseMode = "HTML"
		_, err = bot.Send(msg)
		if err != nil {
			log.Println(err)
		}

		currentUserIndx++

		if currentUserIndx > len(users) {
			c.UserSlider.EditingUserIndx = 0
			c.CurrentMenu = models.MenuUsersEdit

			msg := tgbotapi.NewMessage(c.ChatID, "No more users to approve. It was last one")
			_, err := bot.Send(msg)
			if err != nil {
				log.Println(err)
			}

			handleUsersEdit(c)
			return
		}

		c.UserSlider.EditingUserIndx = currentUserIndx

	case models.Next:
		currentUserIndx++

		if currentUserIndx > len(users) {
			c.UserSlider.EditingUserIndx = 0
			c.CurrentMenu = models.MenuUsersEdit

			msg := tgbotapi.NewMessage(c.ChatID, "No more users to ban. It was last one")
			_, err := bot.Send(msg)
			if err != nil {
				log.Println(err)
			}

			handleUsersEdit(c)
			return
		}
		c.UserSlider.EditingUserIndx = currentUserIndx
	}

	u = users[currentUserIndx]

	reply = fmt.Sprintf(`You moderating: <a href="tg://user?id=%v">%v %v</a>`, u.TelegramID, u.FirstName, u.LastName)
	msg := tgbotapi.NewMessage(c.ChatID, reply)
	msg.ParseMode = "HTML"

	btnRow1 := tgbotapi.NewKeyboardButtonRow(buttons.Back)
	btnRow2 := tgbotapi.NewKeyboardButtonRow(buttons.Ban, buttons.Next)

	markup := tgbotapi.NewReplyKeyboard(btnRow1, btnRow2)
	markup.Selective = true
	msg.ReplyMarkup = markup

	_, err := bot.Send(msg)
	if err != nil {
		log.Println(err)
	}
}

func handleUsersEditUnban(c *models.UserCache) {

	if c.User.Admin == 0 {
		c.CurrentMenu = models.MenuMain
		handleMain(c)
		return
	}

	c.CurrentMenu = models.MenuUsersEditUnban

	currentUserIndx := c.UserSlider.EditingUserIndx
	users := c.UserSlider.Users

	var u models.DbUsers
	var msgText string

	doAction := true

	if currentUserIndx == 0 {
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
				u.id`, models.UserBanned, c.User.TelegramID)
		if err != nil {
			c.UserSlider.EditingUserIndx = 0
			c.CurrentMenu = models.MenuUsersEdit

			msg := tgbotapi.NewMessage(c.ChatID, "Something went wrong while taking ban list :(")
			msg.ReplyToMessageID = c.MessageID
			_, err := bot.Send(msg)
			if err != nil {
				log.Println(err)
			}

			return
		}
		defer rows.Close()

		users = make(map[int]models.DbUsers)

		i := 1
		for rows.Next() {
			err := rows.Scan(&u.ID, &u.TelegramID, &u.FirstName, &u.LastName)
			if err != nil {
				log.Println(err)
			} else {
				users[i] = u
				i++
			}
		}

		if len(users) == 0 {
			c.UserSlider.EditingUserIndx = 0
			c.CurrentMenu = models.MenuUsersEdit

			msg := tgbotapi.NewMessage(c.ChatID, "You have no users to unban for now")
			_, err := bot.Send(msg)
			if err != nil {
				log.Println(err)
			}

			handleUsersEdit(c)
			return
		}

		currentUserIndx = 1
		c.UserSlider.Users = users
		c.UserSlider.EditingUserIndx = currentUserIndx
	}

	if doAction {
		msgText = c.Text
	} else {
		msgText = ""
	}

	switch msgText {
	case models.Unban:

		u := users[currentUserIndx]

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
			c.UserSlider.EditingUserIndx = 0
			c.CurrentMenu = models.MenuUsersEdit

			msg := tgbotapi.NewMessage(c.ChatID, "Something went wrong while unbanning user :(")
			msg.ReplyToMessageID = c.MessageID
			_, err := bot.Send(msg)
			if err != nil {
				log.Println(err)
			}

			handleUsersEdit(c)
			return
		}

		timeNow := time.Now().UTC()
		_, err = stmt.Exec(models.UserApprowed, timeNow, c.User.TelegramID, u.TelegramID)
		if err != nil {
			c.UserSlider.EditingUserIndx = 0
			c.CurrentMenu = models.MenuUsersEdit

			msg := tgbotapi.NewMessage(c.ChatID, "Something went wrong while unbanning user :(")
			msg.ReplyToMessageID = c.MessageID
			_, err := bot.Send(msg)
			if err != nil {
				log.Println(err)
			}

			handleUsersEdit(c)
			return
		}

		reply := fmt.Sprintf(`Your account has been <b>unbanned</b> by <a href="tg://user?id=%v">%v %v</a> at %v. Try to text to admin`, c.User.TelegramID, c.User.FirstName, c.User.LastName, timeNow)
		msg := tgbotapi.NewMessage(int64(u.TelegramID), reply)
		msg.ParseMode = "HTML"
		_, err = bot.Send(msg)
		if err != nil {
			log.Println(err)
		}

		reply = fmt.Sprintf("Account %v %v has been <b>unbanned</b>", u.FirstName, u.LastName)
		msg = tgbotapi.NewMessage(c.ChatID, reply)
		msg.ParseMode = "HTML"
		_, err = bot.Send(msg)
		if err != nil {
			log.Println(err)
		}

		currentUserIndx++

		if currentUserIndx > len(users) {
			c.UserSlider.EditingUserIndx = 0
			c.CurrentMenu = models.MenuUsersEdit

			msg := tgbotapi.NewMessage(c.ChatID, "No more users to unban. It was last one")
			_, err := bot.Send(msg)
			if err != nil {
				log.Println(err)
			}

			handleUsersEdit(c)
			return
		}

		c.UserSlider.EditingUserIndx = currentUserIndx

	case models.Next:
		currentUserIndx++

		if currentUserIndx > len(users) {
			c.UserSlider.EditingUserIndx = 0
			c.CurrentMenu = models.MenuUsersEdit

			msg := tgbotapi.NewMessage(c.ChatID, "No more users to unban. It was last one")
			_, err := bot.Send(msg)
			if err != nil {
				log.Println(err)
			}

			handleUsersEdit(c)
			return
		}
		c.UserSlider.EditingUserIndx = currentUserIndx
	}

	u = users[currentUserIndx]

	reply := fmt.Sprintf(`You moderating: <a href="tg://user?id=%v">%v %v</a>`, u.TelegramID, u.FirstName, u.LastName)
	msg := tgbotapi.NewMessage(c.ChatID, reply)
	msg.ParseMode = "HTML"

	btnRow1 := tgbotapi.NewKeyboardButtonRow(buttons.Back)
	btnRow2 := tgbotapi.NewKeyboardButtonRow(buttons.Unban, buttons.Next)

	markup := tgbotapi.NewReplyKeyboard(btnRow1, btnRow2)
	markup.Selective = true
	msg.ReplyMarkup = markup

	_, err := bot.Send(msg)
	if err != nil {
		log.Println(err)
	}
}

func handleInbox(c *models.UserCache) {

	c.CurrentMenu = models.MenuInbox
	c.UserSlider.EditingUserIndx = 0
	c.NewTask = nil

	row1 := tgbotapi.NewKeyboardButtonRow(buttons.Back)
	row2 := tgbotapi.NewKeyboardButtonRow(buttons.New, buttons.Started, buttons.Rejected)
	row3 := tgbotapi.NewKeyboardButtonRow(buttons.Completed, buttons.Closed)
	markup := tgbotapi.NewReplyKeyboard(row1, row2, row3)

	msg := tgbotapi.NewMessage(c.ChatID, "Menu: <b>Inbox</b>")
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = markup
	_, err := bot.Send(msg)
	if err != nil {
		log.Println(err)
	}
}

func backToInbox(c *models.UserCache) {
	c.TaskSlider.EditingTaskIndx = 0
	c.CurrentMenu = models.MenuInbox
	c.TaskSlider.Tasks = nil

	//видалимо повідомлення із слайдера
	if c.CurrentMessage != 0 {
		_, err := bot.DeleteMessage(tgbotapi.DeleteMessageConfig{
			ChatID:    c.ChatID,
			MessageID: c.CurrentMessage,
		})
		if err != nil {
			log.Println(err)
		}
		c.CurrentMessage = 0
	}

	handleInbox(c)
}

func handleSent(c *models.UserCache) {

	c.CurrentMenu = models.MenuSent
	c.UserSlider.EditingUserIndx = 0
	c.NewTask = nil

	row1 := tgbotapi.NewKeyboardButtonRow(buttons.Back)
	row2 := tgbotapi.NewKeyboardButtonRow(buttons.New, buttons.Started, buttons.Rejected)
	row3 := tgbotapi.NewKeyboardButtonRow(buttons.Completed, buttons.Closed)
	markup := tgbotapi.NewReplyKeyboard(row1, row2, row3)

	msg := tgbotapi.NewMessage(c.ChatID, "Menu: <b>Sent<b>")
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = markup
	_, err := bot.Send(msg)
	if err != nil {
		log.Println(err)
	}

}

func backToSent(c *models.UserCache) {
	c.TaskSlider.EditingTaskIndx = 0
	c.CurrentMenu = models.MenuSent
	c.TaskSlider.Tasks = nil

	//видалимо повідомлення із слайдера
	if c.CurrentMessage != 0 {
		_, err := bot.DeleteMessage(tgbotapi.DeleteMessageConfig{
			ChatID:    c.ChatID,
			MessageID: c.CurrentMessage,
		})
		if err != nil {
			log.Println(err)
		}
		c.CurrentMessage = 0
	}

	handleSent(c)
}

func handleInboxTasks(c *models.UserCache, menu string, status string) {

	c.CurrentMenu = menu

	editingTask := c.TaskSlider.EditingTaskIndx
	tasks := c.TaskSlider.Tasks

	var t models.DbTasks
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
			t.id`, c.User.TelegramID, status)
		if err != nil {
			reply := fmt.Sprintf("Something went wrong while selecting %v tasks", status)
			msg := tgbotapi.NewMessage(c.ChatID, reply)
			msg.ParseMode = "HTML"
			_, err := bot.Send(msg)
			if err != nil {
				log.Println(err)
			}

			c.Text = ""
			handleInbox(c)
			return
		}
		defer rows.Close()

		tasks = make(map[int]models.DbTasks)

		i := 1
		for rows.Next() {
			err := rows.Scan(&t.ID, &t.Title, &t.Description, &t.ChangedAt)
			if err != nil {
				log.Println(err)
			} else {
				tasks[i] = t
				i++
			}
		}

		if len(tasks) == 0 {
			reply := fmt.Sprintf("You have no %v tasks", status)
			msg := tgbotapi.NewMessage(c.ChatID, reply)
			_, err := bot.Send(msg)
			if err != nil {
				log.Println(err)
			}

			handleInbox(c)
			return
		}

		c.TaskSlider.Tasks = tasks
		c.TaskSlider.EditingTaskIndx = 1
	}

	if doAction {
		msgText = c.Text
	} else {
		msgText = ""
		c.TaskID = tasks[c.TaskSlider.EditingTaskIndx].ID

		reply := fmt.Sprintf("Menu Inbox->%v->", status)
		msg := tgbotapi.NewMessage(c.ChatID, reply)
		msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(tgbotapi.NewKeyboardButtonRow(buttons.Inbox))
		_, err := bot.Send(msg)
		if err != nil {
			log.Println(err)
		}

		showTask(c)
		return
	}

	switch msgText {
	case models.Next:

		editingTask++

		if editingTask > len(tasks) {
			reply := fmt.Sprintf("No more %v tasks. It was last one", status)
			msg := tgbotapi.NewMessage(c.ChatID, reply)
			_, err := bot.Send(msg)
			if err != nil {
				log.Println(err)
			}

			backToInbox(c)
			return
		}

		c.TaskSlider.EditingTaskIndx = editingTask
		c.TaskID = tasks[editingTask].ID

		showTask(c)
	case models.Previous:
		editingTask--

		if editingTask == 0 {
			reply := fmt.Sprintf("No more %v tasks. It was first one", status)
			msg := tgbotapi.NewMessage(c.ChatID, reply)
			_, err := bot.Send(msg)
			if err != nil {
				log.Println(err)
			}

			backToInbox(c)
			return
		}

		c.TaskSlider.EditingTaskIndx = editingTask
		c.TaskID = tasks[editingTask].ID

		showTask(c)
	case models.Back:
		backToInbox(c)
		return
	case models.Inbox:
		backToInbox(c)
		return
	default:
		backToInbox(c)
		return
	}
}

func handleSentTasks(c *models.UserCache, menu string, status string) {

	c.CurrentMenu = menu

	editingTask := c.TaskSlider.EditingTaskIndx
	tasks := c.TaskSlider.Tasks

	var t models.DbTasks
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
			t.from_user=?
			AND t.status=?
		ORDER BY
			t.id`, c.User.TelegramID, status)
		if err != nil {
			reply := fmt.Sprintf("Something went wrong while selecting %v tasks", status)
			msg := tgbotapi.NewMessage(c.ChatID, reply)
			msg.ParseMode = "HTML"
			_, err := bot.Send(msg)
			if err != nil {
				log.Println(err)
			}

			c.Text = ""
			handleSent(c)
			return
		}
		defer rows.Close()

		tasks = make(map[int]models.DbTasks)

		i := 1
		for rows.Next() {
			err := rows.Scan(&t.ID, &t.Title, &t.Description, &t.ChangedAt)
			if err != nil {
				log.Println(err)
			} else {
				tasks[i] = t
				i++
			}
		}

		if len(tasks) == 0 {
			reply := fmt.Sprintf("You have no %v tasks", status)
			msg := tgbotapi.NewMessage(c.ChatID, reply)
			_, err := bot.Send(msg)
			if err != nil {
				log.Println(err)
			}

			handleSent(c)
			return
		}

		c.TaskSlider.Tasks = tasks
		c.TaskSlider.EditingTaskIndx = 1
	}

	if doAction {
		msgText = c.Text
	} else {
		msgText = ""
		c.TaskID = tasks[c.TaskSlider.EditingTaskIndx].ID

		reply := fmt.Sprintf("Menu: <b>Sent->%v</b>", status)
		msg := tgbotapi.NewMessage(c.ChatID, reply)
		msg.ParseMode = "HTML"
		msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(tgbotapi.NewKeyboardButtonRow(buttons.Sent))
		_, err := bot.Send(msg)
		if err != nil {
			log.Println(err)
		}

		showTask(c)
		return
	}

	switch msgText {
	case models.Next:

		editingTask++

		if editingTask > len(tasks) {
			reply := fmt.Sprintf("No more %v tasks. It was last one", status)
			msg := tgbotapi.NewMessage(c.ChatID, reply)
			_, err := bot.Send(msg)
			if err != nil {
				log.Println(err)
			}

			backToSent(c)
			return
		}

		c.TaskSlider.EditingTaskIndx = editingTask
		c.TaskID = tasks[editingTask].ID

		showTask(c)
	case models.Previous:
		editingTask--

		if editingTask == 0 {
			reply := fmt.Sprintf("No more %v tasks. It was first one", status)
			msg := tgbotapi.NewMessage(c.ChatID, reply)
			_, err := bot.Send(msg)
			if err != nil {
				log.Println(err)
			}

			backToSent(c)
			return
		}

		c.TaskSlider.EditingTaskIndx = editingTask
		c.TaskID = tasks[editingTask].ID

		showTask(c)
	case models.Back:
		backToSent(c)
		return
	case models.Sent:
		backToSent(c)
		return
	}
}

func handleNew(c *models.UserCache) {

	c.CurrentMenu = models.MenuNew
	c.UserSlider.EditingUserIndx = 0

	if c.NewTask == nil {
		c.NewTask = &models.Task{}
	}

	switch c.NewTask.Step {
	case models.NewTaskStepUser:
		switch c.Text {
		case models.Cancel:
			c.NewTask = &models.Task{}
			c.CurrentMenu = models.MenuMain
			handleMain(c)
			return
		case models.New, "":
			rows, err := db.Query(`
			SELECT 
				u.tgid,
				u.first_name,
				u.last_name
			FROM 
				users u
			WHERE
				u.status=?`, models.UserApprowed)
			if err != nil {
				c.CurrentMenu = models.MenuMain
				c.UserSlider.EditingUserIndx = 0
				c.NewTask = nil

				reply := "Something went wrong while selecting user can't start new Task"
				msg := tgbotapi.NewMessage(c.ChatID, reply)
				msg.ReplyToMessageID = c.MessageID
				_, err := bot.Send(msg)
				if err != nil {
					log.Println(err)
				}

				handleMain(c)
				return
			}
			defer rows.Close()

			var u models.DbUsers
			var users = make(map[int]models.DbUsers)

			i := 1
			for rows.Next() {
				err := rows.Scan(&u.TelegramID, &u.FirstName, &u.LastName)
				if err != nil {
					log.Println(err)
				} else {
					users[i] = u
					i++
				}
			}

			c.UserSlider.Users = users

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
				//btnRow = nil
			}

			markup := tgbotapi.NewReplyKeyboard(keyboard[:]...)
			markup.Selective = true

			reply := "Choose user:"
			msg := tgbotapi.NewMessage(c.ChatID, reply)
			msg.ReplyMarkup = markup
			_, err = bot.Send(msg)
			if err != nil {
				log.Println(err)
			}
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

			users := c.UserSlider.Users

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

			reply := fmt.Sprintf(`<b>New Task</b>
			To user: <a href="tg://user?id=%v">%v %v</a>
			
			Enter Task title <i>(then press Enter)</i>:`, toUser.TelegramID, toUser.FirstName, toUser.LastName)
			msg := tgbotapi.NewMessage(c.ChatID, reply)
			msg.ParseMode = "HTML"
			msg.ReplyMarkup = markup
			_, err = bot.Send(msg)
			if err != nil {
				log.Println(err)
			}
			return
		}
	case models.NewTaskStepTitle:
		switch c.Text {
		case models.Cancel:
			c.NewTask = &models.Task{}
			c.CurrentMenu = models.MenuMain
			handleMain(c)
			return
		case models.Back:
			c.NewTask.ToUser = &models.DbUsers{}
			c.NewTask.Step = models.NewTaskStepUser

			c.Text = ""
			handleNew(c)
			return
		case "":
			toUser := c.NewTask.ToUser

			row1 := tgbotapi.NewKeyboardButtonRow(buttons.Back, buttons.Cancel)
			markup := tgbotapi.NewReplyKeyboard(row1)
			markup.Selective = true

			reply := fmt.Sprintf(`<b>New Task</b>
			To user: <a href="tg://user?id=%v">%v %v</a>
			
			Enter Task title <i>(then press Enter)</i>:`, toUser.TelegramID, toUser.FirstName, toUser.LastName)
			msg := tgbotapi.NewMessage(c.ChatID, reply)
			msg.ParseMode = "HTML"
			msg.ReplyMarkup = markup
			_, err := bot.Send(msg)
			if err != nil {
				log.Println(err)
			}

			return
		default:
			c.NewTask.Title = c.Text
			c.NewTask.Step = models.NewTaskStepDescription

			toUser := c.NewTask.ToUser

			row1 := tgbotapi.NewKeyboardButtonRow(buttons.Back, buttons.Cancel)
			markup := tgbotapi.NewReplyKeyboard(row1)
			markup.Selective = true

			reply := fmt.Sprintf(`<b>New Task</b>
			To user: <a href="tg://user?id=%v">%v %v</a>
			Title: %v
			
			Enter Task description:`, toUser.TelegramID, toUser.FirstName, toUser.LastName, c.Text)

			msg := tgbotapi.NewMessage(c.ChatID, reply)
			msg.ParseMode = "HTML"
			msg.ReplyMarkup = markup
			_, err := bot.Send(msg)
			if err != nil {
				log.Println(err)
			}
			return
		}
	case models.NewTaskStepDescription:
		switch c.Text {
		case models.Cancel:
			c.NewTask = &models.Task{}
			c.CurrentMenu = models.MenuMain
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

			reply := fmt.Sprintf(`<b>New Task</b>
			To user: <a href="tg://user?id=%v">%v %v</a>
			Title: %v
			
			Enter Task description:`, toUser.TelegramID, toUser.FirstName, toUser.LastName, c.NewTask.Title)

			msg := tgbotapi.NewMessage(c.ChatID, reply)
			msg.ParseMode = "HTML"
			msg.ReplyMarkup = markup
			_, err := bot.Send(msg)
			if err != nil {
				log.Println(err)
			}

			return
		default:
			c.NewTask.Description = c.Text
			c.NewTask.Step = models.NewTaskStepSaveToDB
			toUser := c.NewTask.ToUser

			row1 := tgbotapi.NewKeyboardButtonRow(buttons.Back, buttons.Cancel, buttons.Save)
			markup := tgbotapi.NewReplyKeyboard(row1)
			markup.Selective = true

			reply := fmt.Sprintf(`<b>New Task</b>
			To user: <a href="tg://user?id=%v">%v %v</a>
			Title: %v
			Description: %v`, toUser.TelegramID, toUser.FirstName, toUser.LastName, c.NewTask.Title, c.Text)
			msg := tgbotapi.NewMessage(c.ChatID, reply)
			msg.ParseMode = "HTML"
			msg.ReplyMarkup = markup
			_, err := bot.Send(msg)
			if err != nil {
				log.Println(err)
			}
			return
		}
	case models.NewTaskStepSaveToDB:
		switch c.Text {
		case models.Cancel:
			c.NewTask = &models.Task{}
			c.CurrentMenu = models.MenuMain
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
				c.NewTask.Step = models.NewTaskStepUser

				msg := tgbotapi.NewMessage(c.ChatID, "Something went wrong while saving Task")
				_, err := bot.Send(msg)
				if err != nil {
					log.Println(err)
				}

				handleMain(c)
				return
			}

			res, err := stmt.Exec(c.User.TelegramID, toUser.TelegramID, models.TaskStatusNew, createdAt, c.User.TelegramID, c.NewTask.Title, c.NewTask.Description)
			if err != nil {
				c.NewTask.Step = models.NewTaskStepUser

				msg := tgbotapi.NewMessage(c.ChatID, "Something went wrong while saving Task")
				_, err := bot.Send(msg)
				if err != nil {
					log.Println(err)
				}

				handleMain(c)
				return
			}

			taskID, err := res.LastInsertId()
			if err != nil {
				c.NewTask.Step = models.NewTaskStepUser

				msg := tgbotapi.NewMessage(c.ChatID, "Something went wrong while saving Task")
				_, err := bot.Send(msg)
				if err != nil {
					log.Println(err)
				}

				handleMain(c)
				return
			}

			reply := fmt.Sprintf(`<b>Task #%v</b>
				To user: <a href="tg://user?id=%v">%v %v</a>
				Title: %v
				Description: %v`, taskID, toUser.TelegramID, toUser.FirstName, toUser.LastName, c.NewTask.Title, c.NewTask.Description)

			msg := tgbotapi.NewMessage(c.ChatID, reply)
			msg.ParseMode = "HTML"
			_, err = bot.Send(msg)
			if err != nil {
				log.Println(err)
			}

			if c.User.TelegramID != toUser.TelegramID {
				reply = fmt.Sprintf(`<b>You have new Task #%v</b>				
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

			c.NewTask = &models.Task{}
			c.CurrentMenu = models.MenuMain

			handleMain(c)
			return

		default:
			return
		}
	default:
		return
	}
}

func handleComment(c *models.UserCache) {

	if c.Text == "" {
		c.CurrentMenu = ""

		msg := tgbotapi.NewMessage(c.ChatID, "I don't see any sense to add an empty comment, sorry.:(")
		msg.ChatID = c.ChatID
		_, err := bot.Send(msg)
		if err != nil {
			log.Println(err)
		}

		handleMain(c)
		return
	}

	if c.TaskID == 0 {
		c.CurrentMenu = ""
		handleMain(c)
		return
	}

	stmt, err := db.Prepare(`
		UPDATE 
			tasks
		SET
			comment=?,
			commented_at=?,
			commented_by=?
		WHERE
			id=?;`)
	if err != nil {
		msg := tgbotapi.NewMessage(c.ChatID, "Something went wrong while updating Task comment:(")
		msg.ReplyToMessageID = c.MessageID
		_, err := bot.Send(msg)
		if err != nil {
			log.Println(err)
		}

		handleUsersEdit(c)
		return
	}

	_, err = stmt.Exec(c.Text, time.Now(), c.User.TelegramID, c.TaskID)
	if err != nil {
		msg := tgbotapi.NewMessage(c.ChatID, "Something went wrong while updating Task comment :(")
		msg.ReplyToMessageID = c.MessageID
		_, err := bot.Send(msg)
		if err != nil {
			log.Println(err)
		}

		handleUsersEdit(c)
		return
	}

	handleMain(c)
}

func getUserByTelegramID(userid int) models.DbUsers {

	var u models.DbUsers

	rows, err := db.Query(`
		SELECT
			u.id,
			u.tgid,			
			u.first_name,
			u.last_name,
			u.admin,
			u.status			
		FROM 
			users u
		WHERE
			tgid=?`, userid)
	if err != nil {
		log.Fatal(err.Error())
	}
	defer rows.Close()

	if rows.Next() {
		err := rows.Scan(&u.ID, &u.TelegramID, &u.FirstName, &u.LastName, &u.Admin, &u.Status)
		if err != nil {
			log.Println(err)
		}
	}

	return u
}

func handleCallbackQuery(c *models.UserCache) {

	var err error

	command := c.CallbackData

	if command == "" {
		return
	}

	xs := strings.Split(command, "|")
	do := xs[0]

	switch do {
	case models.NewUserCancel:
		newUserCancel(c)
	case models.NewUserRequest:
		newUserAdd(c)
	case models.NewUserDecline:
		if len(xs) == 2 {
			newUserDecline(c, xs[1])
		}
	case models.NewUserAccept:
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
	case models.Previous:
		c.Text = models.Previous
		switch c.CurrentMenu {
		case models.MenuInboxNew:
			handleInboxTasks(c, models.MenuInboxNew, models.TaskStatusNew)
		case models.MenuInboxStarted:
			handleInboxTasks(c, models.MenuInboxStarted, models.TaskStatusStarted)
		case models.MenuInboxRejected:
			handleInboxTasks(c, models.MenuInboxRejected, models.TaskStatusRejected)
		case models.MenuInboxCompleted:
			handleInboxTasks(c, models.MenuInboxCompleted, models.TaskStatusCompleted)
		case models.MenuInboxClosed:
			handleInboxTasks(c, models.MenuInboxClosed, models.TaskStatusClosed)
		case models.MenuSentNew:
			handleSentTasks(c, models.MenuSentNew, models.TaskStatusNew)
		case models.MenuSentStarted:
			handleSentTasks(c, models.MenuSentStarted, models.TaskStatusStarted)
		case models.MenuSentRejected:
			handleSentTasks(c, models.MenuSentRejected, models.TaskStatusRejected)
		case models.MenuSentCompleted:
			handleSentTasks(c, models.MenuSentCompleted, models.TaskStatusCompleted)
		case models.MenuSentClosed:
			handleSentTasks(c, models.MenuSentClosed, models.TaskStatusClosed)
		default:
			c.TaskSlider.EditingTaskIndx = 0
			c.CurrentMenu = models.MenuMain
			c.TaskSlider.Tasks = nil

			//видалимо повідомлення із слайдера
			if c.CurrentMessage != 0 {
				_, err := bot.DeleteMessage(tgbotapi.DeleteMessageConfig{
					ChatID:    c.ChatID,
					MessageID: c.CurrentMessage,
				})
				if err != nil {
					log.Println(err)
				}
				c.CurrentMessage = 0
			}

			handleMain(c)
		}
	case models.Next:
		c.Text = models.Next
		switch c.CurrentMenu {
		case models.MenuInboxNew:
			handleInboxTasks(c, models.MenuInboxNew, models.TaskStatusNew)
		case models.MenuInboxStarted:
			handleInboxTasks(c, models.MenuInboxStarted, models.TaskStatusStarted)
		case models.MenuInboxRejected:
			handleInboxTasks(c, models.MenuInboxRejected, models.TaskStatusRejected)
		case models.MenuInboxCompleted:
			handleInboxTasks(c, models.MenuInboxCompleted, models.TaskStatusCompleted)
		case models.MenuInboxClosed:
			handleInboxTasks(c, models.MenuInboxClosed, models.TaskStatusClosed)
		case models.MenuSentNew:
			handleSentTasks(c, models.MenuSentNew, models.TaskStatusNew)
		case models.MenuSentStarted:
			handleSentTasks(c, models.MenuSentStarted, models.TaskStatusStarted)
		case models.MenuSentRejected:
			handleSentTasks(c, models.MenuSentRejected, models.TaskStatusRejected)
		case models.MenuSentCompleted:
			handleSentTasks(c, models.MenuSentCompleted, models.TaskStatusCompleted)
		case models.MenuSentClosed:
			handleSentTasks(c, models.MenuSentClosed, models.TaskStatusClosed)
		default:
			c.TaskSlider.EditingTaskIndx = 0
			c.CurrentMenu = models.MenuMain
			c.TaskSlider.Tasks = nil

			//видалимо повідомлення із слайдера
			if c.CurrentMessage != 0 {
				_, err := bot.DeleteMessage(tgbotapi.DeleteMessageConfig{
					ChatID:    c.ChatID,
					MessageID: c.CurrentMessage,
				})
				if err != nil {
					log.Println(err)
				}
				c.CurrentMessage = 0
			}

			handleMain(c)
		}
	}
}

func newUserCancel(c *models.UserCache) {

	var cbConfig tgbotapi.CallbackConfig

	cbConfig.CallbackQueryID = c.CallbackID
	cbConfig.ShowAlert = true
	cbConfig.Text = "You canceled activation"

	_, err := bot.AnswerCallbackQuery(cbConfig)
	if err != nil {
		log.Println(err)
	}

	reply := fmt.Sprintf("Dear, %s %s. See you next time\nBye!", c.User.FirstName, c.User.LastName)
	updMsg := tgbotapi.NewEditMessageText(c.ChatID, c.MessageID, reply)
	_, err = bot.Send(updMsg)
	if err != nil {
		log.Println(err)
	}
}

func newUserAdd(c *models.UserCache) {

	var cbConfig tgbotapi.CallbackConfig

	cbConfig.CallbackQueryID = c.CallbackID

	rows, err := db.Query(`
		SELECT 
			u.id
		FROM 
			users u
		WHERE
			u.tgid=?`, c.User.TelegramID)
	if err != nil {
		cbConfig.Text = fmt.Sprintf("Dear, %s %s. sorry, somethings went wrong. Try make request later", c.User.FirstName, c.User.LastName)
		_, err := bot.AnswerCallbackQuery(cbConfig)
		if err != nil {
			log.Println(err)
		}
		return
	}

	if rows.Next() {
		cbConfig.Text = "You have already made request"
		cbConfig.ShowAlert = true
		_, err := bot.AnswerCallbackQuery(cbConfig)
		if err != nil {
			log.Println(err)
		}

		updMsg := tgbotapi.NewEditMessageText(c.ChatID, c.MessageID, "Keep calm and wait for approval message")
		_, err = bot.Send(updMsg)
		if err != nil {
			log.Println(err)
		}
		return
	}
	rows.Close()

	stmt, err := db.Prepare(`INSERT INTO users (tgid, first_name, last_name, status, changed_by, changed_at) values(?, ?, ?, ?, ?, ?)`)
	if err != nil {
		cbConfig.Text = fmt.Sprintf("Dear, %s %s. sorry, somethings went wrong. Try make request later", c.User.FirstName, c.User.LastName)
		_, err := bot.AnswerCallbackQuery(cbConfig)
		if err != nil {
			log.Println(err)
		}
		return
	}

	res, err := stmt.Exec(c.User.TelegramID, c.User.FirstName, c.User.LastName, models.UserRequested, c.User.TelegramID, time.Now().UTC())
	if err != nil {
		cbConfig.Text = fmt.Sprintf("Dear, %s %s. sorry, somethings went wrong. Try make request later", c.User.FirstName, c.User.LastName)
		_, err := bot.AnswerCallbackQuery(cbConfig)
		if err != nil {
			log.Println(err)
		}
		return
	}

	_, err = res.LastInsertId()
	if err != nil {
		cbConfig.Text = fmt.Sprintf("Dear, %s %s. sorry, somethings went wrong. Try make request later", c.User.FirstName, c.User.LastName)
		_, err := bot.AnswerCallbackQuery(cbConfig)
		if err != nil {
			log.Println(err)
		}
		return
	}

	cbConfig.Text = ""
	_, err = bot.AnswerCallbackQuery(cbConfig)
	if err != nil {
		log.Println(err)
	}

	var reply string

	rows, err = db.Query(`
		SELECT
			u.tgid
		FROM 
			users u
		WHERE
			u.admin=1`)
	if err != nil {
		reply = fmt.Sprintf("Dear, %s %s. Ok, wait for approval message!", c.User.FirstName, c.User.LastName)
	} else {
		reply = fmt.Sprintf("Dear, %s %s. I made request to Taskeram admins to approve your account\nWait for approval message!", c.User.FirstName, c.User.LastName)
	}
	defer rows.Close()

	var u models.DbUsers
	for rows.Next() {
		err := rows.Scan(&u.TelegramID)
		if err != nil {
			log.Println(err)
		}

		btnAccept := tgbotapi.NewInlineKeyboardButtonData("Accept", fmt.Sprintf("%v|%v", models.NewUserAccept, c.User.TelegramID))
		btnDecline := tgbotapi.NewInlineKeyboardButtonData("Decline", fmt.Sprintf("%v|%v", models.NewUserDecline, c.User.TelegramID))
		btnRow1 := tgbotapi.NewInlineKeyboardRow(btnAccept, btnDecline)
		markup := tgbotapi.NewInlineKeyboardMarkup(btnRow1)

		text := fmt.Sprintf(`User: <a href="tg://user?id=%v">%v %v</a> made approve request`, c.User.TelegramID, c.User.FirstName, c.User.LastName)
		msg := tgbotapi.NewMessage(int64(u.TelegramID), text)
		msg.ParseMode = "HTML"
		msg.ReplyMarkup = markup
		_, err = bot.Send(msg)
		if err != nil {
			log.Println(err)
		}
	}

	updMsg := tgbotapi.NewEditMessageText(c.ChatID, c.MessageID, reply)
	_, err = bot.Send(updMsg)
	if err != nil {
		log.Println(err)
	}
}

func newUserDecline(c *models.UserCache, ID string) {

	var cbConfig tgbotapi.CallbackConfig

	userID, err := strconv.Atoi(ID)
	if err != nil {
		msg := tgbotapi.NewMessage(c.ChatID, "Sorry, something went wrong")
		_, err := bot.Send(msg)
		if err != nil {
			log.Println(err)
		}

		return
	}

	rows, err := db.Query(`
		SELECT 
			u.tgid,
			u.first_name,
			u.last_name,
			u.status,
			u.changed_at
		FROM 
			users u
		WHERE
			u.tgid=?`, userID)
	if err != nil {
		msg := tgbotapi.NewMessage(c.ChatID, "Sorry, something went wrong")
		_, err := bot.Send(msg)
		if err != nil {
			log.Println(err)
		}

		return
	}
	defer rows.Close()

	var u models.DbUsers
	if rows.Next() {
		err := rows.Scan(&u.TelegramID, &u.FirstName, &u.LastName, &u.Status, &u.ChangedAt)
		if err != nil {
			log.Println(err)
		}
	}

	if u.Status != models.UserRequested {
		reply := fmt.Sprintf(`User: <a href="tg://user?id=%v">%v %v</a>
		<i>current status:</i> %v
		<i>changed at:</i> %v`, u.TelegramID, u.FirstName, u.LastName, u.Status, u.ChangedAt)

		msg := tgbotapi.NewEditMessageText(c.ChatID, c.MessageID, reply)
		msg.ParseMode = "HTML"
		_, err := bot.Send(msg)
		if err != nil {
			log.Println(err)
		}

		cbConfig.CallbackQueryID = c.CallbackID
		cbConfig.Text = ""
		_, err = bot.AnswerCallbackQuery(cbConfig)
		if err != nil {
			log.Println(err)
		}
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
	if err != nil {
		log.Println(err)
	}

	reply := fmt.Sprintf(`Unfortunately your request has been <b>declined</b> by <a href="tg://user?id=%v">%v %v</a> at %v. Try to text to admin`, c.User.TelegramID, c.User.FirstName, c.User.LastName, timeNow)
	msg := tgbotapi.NewMessage(int64(userID), reply)
	msg.ParseMode = "HTML"
	_, err = bot.Send(msg)
	if err != nil {
		log.Println(err)
	}

	reply = fmt.Sprintf(`<a href="tg://user?id=%v">%v %v</a> has been <b>banned</b>`, u.TelegramID, u.FirstName, u.LastName)
	msgEdited := tgbotapi.NewEditMessageText(c.ChatID, c.MessageID, reply)
	msgEdited.ParseMode = "HTML"
	_, err = bot.Send(msgEdited)
	if err != nil {
		log.Println(err)
	}

	cbConfig.CallbackQueryID = c.CallbackID
	cbConfig.Text = ""
	_, err = bot.AnswerCallbackQuery(cbConfig)
	if err != nil {
		log.Println(err)
	}
}

func newUserAccpet(c *models.UserCache, ID string) {

	var cbConfig tgbotapi.CallbackConfig

	userID, err := strconv.Atoi(ID)
	if err != nil {
		msg := tgbotapi.NewMessage(c.ChatID, "Sorry, something went wrong")
		_, err := bot.Send(msg)
		if err != nil {
			log.Println(err)
		}

		return
	}

	rows, err := db.Query(`
		SELECT 
			u.tgid,
			u.first_name,
			u.last_name,
			u.status,
			u.changed_at
		FROM 
			users u 
		WHERE
			u.tgid=?`, userID)
	if err != nil {
		msg := tgbotapi.NewMessage(c.ChatID, "Sorry, something went wrong")
		_, err := bot.Send(msg)
		if err != nil {
			log.Println(err)
		}

		return
	}

	var u models.DbUsers
	if rows.Next() {
		err := rows.Scan(&u.TelegramID, &u.FirstName, &u.LastName, &u.Status, &u.ChangedAt)
		if err != nil {
			log.Println(err)
		}
	}
	rows.Close()

	if u.Status != models.UserRequested {
		reply := fmt.Sprintf(`User: <a href="tg://user?id=%v">%v %v</a>
		<i>current status:</i> %v
		<i>changed at:</i> %v`, u.TelegramID, u.FirstName, u.LastName, u.Status, u.ChangedAt)

		msg := tgbotapi.NewEditMessageText(c.ChatID, c.MessageID, reply)
		msg.ParseMode = "HTML"
		_, err := bot.Send(msg)
		if err != nil {
			log.Println(err)
		}

		cbConfig.CallbackQueryID = c.CallbackID
		cbConfig.Text = ""
		_, err = bot.AnswerCallbackQuery(cbConfig)
		if err != nil {
			log.Println(err)
		}
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
	_, err = stmt.Exec(models.UserApprowed, c.User.TelegramID, timeNow, u.TelegramID)
	if err != nil {
		reply := fmt.Sprintf(`Can't approve '<a href="tg://user?id=%v">%v %v</a>. Err:%v`, u.TelegramID, u.FirstName, u.LastName, err.Error())
		msg := tgbotapi.NewMessage(c.ChatID, reply)
		msg.ParseMode = "HTML"
		_, err := bot.Send(msg)
		if err != nil {
			log.Println(err)
		}
		return
	}

	reply := fmt.Sprintf(`Your account has been <b>approved</b> by <a href="tg://user?id=%v">%v %v</a> at %v`, c.User.TelegramID, c.User.FirstName, c.User.LastName, timeNow)
	msg := tgbotapi.NewMessage(int64(userID), reply)
	msg.ParseMode = "HTML"
	_, err = bot.Send(msg)
	if err != nil {
		log.Println(err)
	}

	reply = fmt.Sprintf(`<a href="tg://user?id=%v">%v %v</a> has been <b>approved</b>`, u.TelegramID, u.FirstName, u.LastName)
	msgEdited := tgbotapi.NewEditMessageText(c.ChatID, c.MessageID, reply)
	msgEdited.ParseMode = "HTML"
	_, err = bot.Send(msgEdited)
	if err != nil {
		log.Println(err)
	}

	cbConfig.CallbackQueryID = c.CallbackID
	cbConfig.Text = ""
	_, err = bot.AnswerCallbackQuery(cbConfig)
	if err != nil {
		log.Println(err)
	}
}

func changeStatus(c *models.UserCache, action string) {

	var t models.DbTasks
	var cbConfig tgbotapi.CallbackConfig

	newStatus := actionStatus[action]

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
		cbConfig.Text = "Something went wrong while checking current Task status"
		cbConfig.ShowAlert = true
		cbConfig.CallbackQueryID = c.CallbackID
		_, err := bot.AnswerCallbackQuery(cbConfig)
		if err != nil {
			log.Println(err)
		}
		return
	}

	if rows.Next() {
		err := rows.Scan(&t.ID, &t.Status, &t.ToUser, &t.FromUser)
		if err != nil {
			log.Println(err)
		}
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

	if !rule.Contains(action) {
		cbConfig.Text = fmt.Sprintf("It isn't allowed to change the status to %v for Task #%v", newStatus, t.ID)
		cbConfig.ShowAlert = true
		cbConfig.CallbackQueryID = c.CallbackID
		_, err := bot.AnswerCallbackQuery(cbConfig)
		if err != nil {
			log.Println(err)
		}

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
		cbConfig.Text = "Something went wrong while updating Task status"
		cbConfig.ShowAlert = true
		cbConfig.CallbackQueryID = c.CallbackID
		_, err := bot.AnswerCallbackQuery(cbConfig)
		if err != nil {
			log.Println(err)
		}
		return
	}

	changedAt := time.Now().UTC()

	_, err = stmt.Exec(newStatus, changedAt, c.User.TelegramID, t.ID)
	if err != nil {
		cbConfig.Text = "Something went wrong while updating Task status"
		cbConfig.ShowAlert = true
		cbConfig.CallbackQueryID = c.CallbackID
		_, err := bot.AnswerCallbackQuery(cbConfig)
		if err != nil {
			log.Println(err)
		}
		return
	}

	cbConfig.Text = fmt.Sprintf("Status has been changed to %v for Task %v", newStatus, t.ID)
	cbConfig.CallbackQueryID = c.CallbackID
	_, err = bot.AnswerCallbackQuery(cbConfig)
	if err != nil {
		log.Println(err)
	}

	updateTaskInlineKeyboard(c.ChatID, c.MessageID, t.ID, taskType, newStatus)

	chatID := t.FromUser
	if taskType == "Sent" {
		chatID = t.ToUser
	}

	reply := fmt.Sprintf(`Task <b>#%v</b>
		status was changed to %v
		by <a href="tg://user?id=%v">%v %v</a> at %v`, t.ID, newStatus, c.User.TelegramID, c.User.FirstName, c.User.LastName, changedAt)
	msg := tgbotapi.NewMessage(int64(chatID), reply)
	msg.ParseMode = "HTML"
	_, err = bot.Send(msg)
	if err != nil {
		log.Println(err)
	}
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

func showTask(c *models.UserCache) {

	rows, err := db.Query(`
		SELECT
			t.id,
			t.from_user,
			t.to_user,
			t.status,
			t.changed_at,
			t.changed_by,
			t.comment,			
			t.title,
			t.description,			
			t.images,
			t.documents	
		FROM tasks t
		WHERE
			t.id=?
			AND (t.to_user=? OR t.from_user=?)`, c.TaskID, c.User.TelegramID, c.User.TelegramID)
	if err != nil {
		msg := tgbotapi.NewMessage(c.ChatID, "Something went wrong while selecting Task info")
		_, err := bot.Send(msg)
		if err != nil {
			log.Println(err)
		}
		return
	}
	defer rows.Close()

	var t models.DbTasks

	if rows.Next() {
		err = rows.Scan(&t.ID, &t.FromUser, &t.ToUser, &t.Status, &t.ChangedAt, &t.ChangedBy, &t.Comments, &t.Title, &t.Description, &t.Images, &t.Documents)
		if err != nil {
			log.Println(err)
		}
	} else {
		msg := tgbotapi.NewMessage(c.ChatID, fmt.Sprintln("Can't find any Task with ID: ", c.TaskID))
		_, err := bot.Send(msg)
		if err != nil {
			log.Println(err)
		}
		return
	}

	if t.ToUser != c.User.TelegramID && t.FromUser != c.User.TelegramID {
		msg := tgbotapi.NewMessage(c.ChatID, "Access denided")
		_, err := bot.Send(msg)
		if err != nil {
			log.Println(err)
		}
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
	var kbdReply [][]tgbotapi.InlineKeyboardButton
	for _, val := range rule {
		btnRow = append(btnRow, tgbotapi.NewInlineKeyboardButtonData(val, fmt.Sprintf("%v|%v", val, t.ID)))
	}

	kbdReply = append(kbdReply, btnRow)

	//ми прийшли сюди з меню задач. Запсукаємо слайдер
	if c.TaskSlider.EditingTaskIndx != 0 {
		var btnRowNavigation []tgbotapi.InlineKeyboardButton

		if c.TaskSlider.EditingTaskIndx > 1 {
			btnRowNavigation = append(btnRowNavigation, tgbotapi.NewInlineKeyboardButtonData("← Previous", models.Previous))
		}

		if c.TaskSlider.EditingTaskIndx <= len(c.TaskSlider.Tasks) {
			btnRowNavigation = append(btnRowNavigation, tgbotapi.NewInlineKeyboardButtonData("Next →", models.Next))
		}

		kbdReply = append(kbdReply, btnRowNavigation)
	}

	replyMarkup := tgbotapi.NewInlineKeyboardMarkup(kbdReply...)

	var msgSent tgbotapi.Message
	if c.CurrentMessage == 0 {
		msg := tgbotapi.NewMessage(c.ChatID, reply)
		msg.ReplyMarkup = replyMarkup
		msg.ParseMode = "HTML"
		msgSent, err = bot.Send(msg)
	} else {
		msg := tgbotapi.NewEditMessageText(c.ChatID, c.CurrentMessage, reply)
		msg.BaseEdit.ReplyMarkup = &replyMarkup
		msg.ParseMode = "HTML"
		msgSent, err = bot.Send(msg)
	}

	if err != nil {
		log.Println(err)
		c.CurrentMessage = 0
		return
	}

	c.CurrentMessage = msgSent.MessageID
}

func showHistory(c *models.UserCache) {

	var cbConfig tgbotapi.CallbackConfig
	var row models.DbHistory
	var xs []models.DbHistory

	rows, err := db.Query(`
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
			h.taskid=?	
			AND (t.from_user=?
				OR t.to_user=?)		
		ORDER BY 
			h.date`, c.TaskID, c.User.TelegramID, c.User.TelegramID)
	if err != nil {
		cbConfig.Text = "Something went wrong while selecting Task history"
		cbConfig.ShowAlert = true
		cbConfig.CallbackQueryID = c.CallbackID
		_, err := bot.AnswerCallbackQuery(cbConfig)
		if err != nil {
			log.Println(err)
		}
		return
	}

	for rows.Next() {
		err := rows.Scan(&row.HDb.Status, &row.HDb.Date, &row.HDb.TaskID, &row.UDb.TelegramID, &row.UDb.FirstName, &row.UDb.LastName, &row.TDb.Title)
		if err != nil {
			log.Println(err)
		} else {
			xs = append(xs, row)
		}
	}
	rows.Close()

	if len(xs) == 0 {
		cbConfig.Text = "There is no history for this Task"
		cbConfig.ShowAlert = true
		cbConfig.CallbackQueryID = c.CallbackID
		_, err := bot.AnswerCallbackQuery(cbConfig)
		if err != nil {
			log.Println(err)
		}
		return
	}

	reply := fmt.Sprintf(`<strong>Task #%v</strong>
	<i>Title</i> %v

	`, xs[0].HDb.TaskID, xs[0].TDb.Title)

	for key, val := range xs {
		reply += fmt.Sprintf(`%v. <b>%v</b> by <a href="tg://user?id=%v">%v %v</a> at %v\n`, key+1, val.HDb.Status, val.UDb.TelegramID, val.UDb.FirstName, val.UDb.LastName, val.HDb.Date.Time)
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

func showComments(c *models.UserCache) {

	var cbConfig tgbotapi.CallbackConfig
	var row models.DbComment
	var xs []models.DbComment

	rows, err := db.Query(`
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
			c.taskid=?
			AND (t.from_user=?
				OR t.to_user=?)		
		ORDER BY 
			c.date`, c.TaskID, c.User.TelegramID, c.User.TelegramID)
	if err != nil {
		cbConfig.Text = "Something went wrong while selecting Task history"
		cbConfig.ShowAlert = true
		cbConfig.CallbackQueryID = c.CallbackID
		_, err := bot.AnswerCallbackQuery(cbConfig)
		if err != nil {
			log.Println(err)
		}
		return
	}

	for rows.Next() {
		err := rows.Scan(&row.CDb.Comment, &row.CDb.Date, &row.CDb.TaskID, &row.UDb.TelegramID, &row.UDb.FirstName, &row.UDb.LastName, &row.TDb.Title)
		if err != nil {
			log.Println(err)
		} else {
			xs = append(xs, row)
		}
	}
	rows.Close()

	if len(xs) == 0 {
		cbConfig.Text = "There is no comments for this Task"
		cbConfig.ShowAlert = true
		cbConfig.CallbackQueryID = c.CallbackID
		_, err := bot.AnswerCallbackQuery(cbConfig)
		if err != nil {
			log.Println(err)
		}
		return
	}

	reply := fmt.Sprintf(`<strong>Task #%v</strong>
	<i>Title</i> %v

	`, xs[0].CDb.TaskID, xs[0].TDb.Title)

	for key, val := range xs {
		reply += fmt.Sprintf(`<b>%v. Comment:</b> %v
		by <a href="tg://user?id=%v">%v %v</a> at %v`, key+1, val.CDb.Comment, val.UDb.TelegramID, val.UDb.FirstName, val.UDb.LastName, val.CDb.Date.Time)
		reply += fmt.Sprintln()
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

func addComment(c *models.UserCache) {

	var t models.DbTasks
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
		cbConfig.Text = "Something went wrong while checking current Task status"
		cbConfig.ShowAlert = true
		cbConfig.CallbackQueryID = c.CallbackID
		_, err := bot.AnswerCallbackQuery(cbConfig)
		if err != nil {
			log.Println(err)
		}
		return
	}

	if rows.Next() {
		err := rows.Scan(&t.ID, &t.Status, &t.ToUser, &t.FromUser)
		if err != nil {
			log.Println(err)
		}
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
		_, err := bot.AnswerCallbackQuery(cbConfig)
		if err != nil {
			log.Println(err)
		}

		updateTaskInlineKeyboard(c.ChatID, c.MessageID, c.TaskID, taskType, t.Status)
		return
	}

	//видалимо повідомлення в якому натиснули кнопку коментувати
	//немає сенсу тримати його, далі починається інша логіка
	if c.CurrentMessage != 0 {
		_, err := bot.DeleteMessage(tgbotapi.DeleteMessageConfig{
			ChatID:    c.ChatID,
			MessageID: c.CurrentMessage,
		})
		if err != nil {
			log.Println(err)
		}
		c.CurrentMessage = 0
	}

	c.CurrentMenu = models.MenuComment
	reply := fmt.Sprintf(`<strong>Task #%v</strong>
	Enter comment: <i>(then pres enter)</i>`, c.TaskID)

	msg := tgbotapi.NewMessage(c.ChatID, reply)
	msg.ParseMode = "HTML"
	_, err = bot.Send(msg)
	if err != nil {
		log.Println(err)
	}
}
