package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/satori/go.uuid"
	"github.com/slevchyk/taskeram/dbase"
	"github.com/slevchyk/taskeram/models"
	"github.com/slevchyk/taskeram/utils"
	"gopkg.in/telegram-bot-api.v4"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func startWebApp() {

	http.Handle("/public/", http.StripPrefix("/public", http.FileServer(http.Dir("./public"))))
	http.Handle("/assets/", http.StripPrefix("/assets", http.FileServer(http.Dir("./assets"))))
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/login", loginHandler)
	http.HandleFunc("/auth", authHandler)
	http.HandleFunc("/logout", logoutHandler)
	http.HandleFunc("/tasks", tasksHandler)
	http.HandleFunc("/task", taskHanlder)
	http.HandleFunc("/user", userHanlder)
	http.HandleFunc("/api/history", apiHistoryHandler)
	http.HandleFunc("/api/updatetaskstatus", apiUpdateTaskStatusHandler)
	http.HandleFunc("/api/commenttask", apiCommentTaskHandler)
	err := http.ListenAndServe(":80", nil)
	if err != nil {
		panic(err)
	}
}

func apiCommentTaskHandler(w http.ResponseWriter, r *http.Request) {

	sessionUUID := r.FormValue("session")
	taskIDValue := r.FormValue("id")

	commentByte, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return
	}

	comment := string(commentByte)

	if sessionUUID == "" || taskIDValue == "" || comment == "" {
		return
	}

	loggedIn, user := alreadyLoggedIn(w, r, sessionUUID)
	if !loggedIn {
		return
	}

	taskID, err := strconv.Atoi(taskIDValue)
	if err != nil {
		return
	}

	stmt, err := dbase.UpdateTaskComment(cfg)
	if err != nil {
		return
	}

	_, err = stmt.Exec(comment, time.Now().UTC(), user.TelegramID, taskID)
	if err != nil {
		return
	}
}

func apiUpdateTaskStatusHandler(w http.ResponseWriter, r *http.Request) {

	sessionUUID := r.FormValue("session")
	taskIDValue := r.FormValue("id")
	status := r.FormValue("status")

	if sessionUUID == "" || taskIDValue == "" || status == "" {
		return
	}

	loggedIn, user := alreadyLoggedIn(w, r, sessionUUID)
	if !loggedIn {
		return
	}

	taskID, err := strconv.Atoi(taskIDValue)
	if err != nil {
		return
	}

	stmt, err := dbase.UpdateTaskStatus(cfg)
	if err != nil {
		return
	}

	_, err = stmt.Exec(status, time.Now().UTC(), user.TelegramID, taskID)
	if err != nil {
		return
	}
}

func apiHistoryHandler(w http.ResponseWriter, r *http.Request) {

	sessionUUID := r.FormValue("session")
	taskIDValue := r.FormValue("id")

	if sessionUUID == "" || taskIDValue == "" {
		return
	}

	loggedIn, user := alreadyLoggedIn(w, r, sessionUUID)
	if !loggedIn {
		return
	}

	taskID, err := strconv.Atoi(taskIDValue)
	if err != nil {
		return
	}

	rows, err := dbase.SelectHistory(cfg, taskID, user.TelegramID)
	if err != nil {
		return
	}

	var record models.DbHistory
	var xs []models.DbHistory

	for rows.Next() {
		err := dbase.ScanHistory(rows, &record)
		if err != nil {
			rows.Close()
			return
		}

		xs = append(xs, record)
	}
	rows.Close()

	json.NewEncoder(w).Encode(xs)
}

func indexHandler(w http.ResponseWriter, r *http.Request) {

	var td models.TplIndex

	loggedIn, user := alreadyLoggedIn(w, r, "")
	if !loggedIn {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	}

	td.NavBar.MainMenu = getMainMenu("")
	td.NavBar.LoggedIn = loggedIn
	td.NavBar.User = user

	err := tpl.ExecuteTemplate(w, "index.gohtml", td)
	if err != nil {
		log.Println(err)
	}
}

func authHandler(w http.ResponseWriter, r *http.Request) {

	var a models.DbAuth

	token, err := r.Cookie("token")
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if token.Value == "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	}

	rows, err := dbase.SelectAuthByToken(cfg, token.Value)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if rows.Next() {
		err := dbase.ScanAuth(rows, &a)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	}
	rows.Close()

	if time.Now().UTC().Sub(a.ExpiryDate.Time) > 0 {
		stmt, err := dbase.DeleteAuthByToken(cfg)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		_, err = stmt.Exec(token.Value)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		token.MaxAge = -1
		http.SetCookie(w, token)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	}

	if a.Approved == 1 {
		stmt, err := dbase.DeleteAuthByToken(cfg)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		_, err = stmt.Exec(token.Value)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		token.MaxAge = -1
		http.SetCookie(w, token)

		var s models.DbSessions

		sessionUUID, _ := uuid.NewV4()

		s.UUID = sessionUUID.String()
		s.TelegramID = a.TelegramID
		s.LastActivity.Time = time.Now().UTC()
		s.IP = r.RemoteAddr
		s.UserAgent = r.Header.Get("User-Agent")
		s.StartedAt = s.LastActivity

		stmt, err = dbase.InsertSession(cfg)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		_, err = dbase.InsertSessionExec(stmt, s)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		c := &http.Cookie{
			Name:  "session",
			Value: s.UUID,
		}
		http.SetCookie(w, c)

		http.Redirect(w, r, "/", http.StatusSeeOther)
	}

	err = tpl.ExecuteTemplate(w, "auth.gohtml", nil)
	if err != nil {
		log.Println(err)
	}
}

func loginHandler(w http.ResponseWriter, r *http.Request) {

	var td models.TplLogin
	var u models.DbUsers

	if r.Method == http.MethodPost {
		tgidString := r.FormValue("tdid")

		tgid, err := strconv.Atoi(tgidString)
		if err != nil {
			http.Error(w, "Incorrect user id", http.StatusForbidden)
			return
		}

		rows, err := dbase.SelectUsersByTelegramID(cfg, tgid)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if rows.Next() {
			err := dbase.ScanUser(rows, &u)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		} else {
			http.Redirect(w, r, "/login", http.StatusBadRequest)
		}
		rows.Close()

		token, err := uuid.NewV4()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var a models.DbAuth

		a.Token = token.String()
		a.TelegramID = tgid
		a.ExpiryDate.Time = time.Now().UTC().Add(time.Second * time.Duration(authSessionLengt))
		a.Approved = 0

		stmt, err := dbase.InsertAuth(cfg)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		_, err = dbase.InsertAuthExec(stmt, a)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		c := &http.Cookie{
			Name:   "token",
			Value:  a.Token,
			MaxAge: authSessionLengt,
		}
		http.SetCookie(w, c)

		btnYes := tgbotapi.NewInlineKeyboardButtonData("✓ Yes", fmt.Sprintf("%v|%v", models.Confirm, a.Token))
		btnNo := tgbotapi.NewInlineKeyboardButtonData("🚫 No", fmt.Sprintf("%v|%v", models.Cancel, a.Token))
		keyboard := tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(btnYes, btnNo))

		reply := fmt.Sprintf(`Do you confirm the login on the web application?
		<b>IP:</b> %v
		<b>User-agent:</b> %v`, r.RemoteAddr, r.Header.Get("User-Agent"))
		msg := tgbotapi.NewMessage(int64(tgid), reply)
		msg.ParseMode = "HTML"
		msg.ReplyMarkup = &keyboard
		_, err = bot.Send(msg)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/auth", http.StatusSeeOther)
		return
	}

	rows, err := dbase.SelectUsersByStatus(cfg, models.UserApprowed)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for rows.Next() {
		err := dbase.ScanUser(rows, &u)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		td.Users = append(td.Users, u)
	}
	rows.Close()

	err = tpl.ExecuteTemplate(w, "login.gohtml", td)
	if err != nil {
		log.Println(err)
	}
}

func tasksHandler(w http.ResponseWriter, r *http.Request) {

	loggedIn, user := alreadyLoggedIn(w, r, "")
	if !loggedIn {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	}

	var (
		td   models.TplTasks
		sr   []models.TasksRow
		t    models.DbTasks
		i    int
		rows *sql.Rows
		err  error
	)

	td.NavBar.LoggedIn = loggedIn
	td.NavBar.User = user

	taskStatus := r.FormValue("status")
	taskStatus = strings.Title(taskStatus)
	if taskStatus == "" {
		taskStatus = models.TaskStatusNew
	}

	taskType := r.FormValue("type")
	switch taskType {
	case "inbox":
		rows, err = dbase.SelectInboxTasks(cfg, user.TelegramID, taskStatus)
	case "sent":
		rows, err = dbase.SelectSentTasks(cfg, user.TelegramID, taskStatus)
	default:
		rows, err = dbase.SelectInboxTasks(cfg, user.TelegramID, taskStatus)
	}

	if err != nil {
		log.Println(err)
	}

	for rows.Next() {
		err := dbase.ScanTask(rows, &t)
		if err != nil {
			log.Println(fmt.Errorf("Scan inbox task webapp: %v", err))
		}

		i++

		tu := dbase.GetUserByTelegramID(cfg, t.ToUser)
		fu := dbase.GetUserByTelegramID(cfg, t.FromUser)

		sr = append(sr, models.TasksRow{Number: i, Task: t, ToUser: tu, FromUser: fu})
	}
	rows.Close()

	td.NavBar.MainMenu = getMainMenu(taskType)
	td.Tabs = template.HTML(getTasksTabs(taskType, taskStatus))
	td.Rows = sr

	err = tpl.ExecuteTemplate(w, "tasks.gohtml", td)
	if err != nil {
		log.Println(err)
	}
}

func taskHanlder(w http.ResponseWriter, r *http.Request) {

	var td models.TplTask
	var t models.DbTasks
	var u models.DbUsers

	loggedIn, user := alreadyLoggedIn(w, r, "")
	if !loggedIn {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	}

	rows, err := dbase.SelectUsersByStatus(cfg, models.UserApprowed)
	if err != nil {
		http.Error(w, fmt.Sprintf("Selecting users for task. Err: %v", err), http.StatusInternalServerError)
		return
	}

	for rows.Next() {
		err := dbase.ScanUser(rows, &u)
		if err != nil {
			http.Error(w, fmt.Sprintf("Scaning users for task. Err: %v", err), http.StatusInternalServerError)
			return
		}
		td.Users = append(td.Users, u)
	}
	rows.Close()

	td.Edit = false

	do := r.FormValue("do")
	switch do {
	case "add":

		t.FromUser = user.TelegramID

		tgid, err := strconv.Atoi(r.FormValue("toUser"))
		if err != nil {
			http.Error(w, fmt.Sprintf("Adding new task. Converting user tg id to int. Err: %v", err), http.StatusInternalServerError)
			return
		}

		u = dbase.GetUserByTelegramID(cfg, tgid)
		if u.ID == 0 {
			http.Error(w, fmt.Sprintf("Adding new task. Can't find any user by tg id."), http.StatusInternalServerError)
			return
		}

		t.ToUser = tgid
		t.Status = models.TaskStatusNew
		t.ChangedAt.Time = time.Now().UTC()
		t.ChangedBy = user.TelegramID
		t.Title = r.FormValue("title")
		t.Description = r.FormValue("description")

		stmt, err := dbase.InsertTask(cfg)
		if err != nil {
			http.Error(w, fmt.Sprintf("Adding new task. Inserting new task. Err: %v", err), http.StatusInternalServerError)
			return
		}

		res, err := dbase.InsertTaskExec(stmt, t)
		if err != nil {
			http.Error(w, fmt.Sprintf("Adding new task. Inserting (commit) new task. Err: %v", err), http.StatusInternalServerError)
			return
		}

		newTaskID, err := res.LastInsertId()
		if err != nil {
			http.Error(w, fmt.Sprintf("Adding new task. Sql result (new task id). Err: %v", err), http.StatusInternalServerError)
			return
		}

		informNewTask(newTaskID, t, user, u)
		http.Redirect(w, r, fmt.Sprintf("/tasks?type=sent&status=%v", strings.ToLower(models.TaskStatusNew)), http.StatusSeeOther)
	case "update":

		var t models.DbTasks

		taskIDValue := r.FormValue("id")
		if taskIDValue == "" {
			return
		}

		taskID, err := strconv.Atoi(taskIDValue)
		if err != nil {
			http.Error(w, fmt.Sprintf("Updating task. Can't convert task id to int. Err: %v", err), http.StatusInternalServerError)
			return
		}

		rows, err := dbase.SelectTasksByIDUserTelegramID(cfg, taskID, user.TelegramID)
		if err != nil {
			http.Error(w, fmt.Sprintf("Updating new task. Selecting task by id. Err: %v", err), http.StatusInternalServerError)
			return
		}

		if rows.Next() {
			err := dbase.ScanTask(rows, &t)
			if err != nil {
				http.Error(w, fmt.Sprintf("Updating new task. Scaning task. Err: %v", err), http.StatusInternalServerError)
				return
			}
		} else {
			return
		}
		rows.Close()

		//встановимо тип задачі
		var taskType string
		if t.ToUser == user.TelegramID {
			taskType = "Inbox"
		} else {
			taskType = "Sent"
		}

		//для значення "status" перетворимо у верхній регістр першу літеру так як в нас заведено в константах
		newStatus := strings.Title(r.FormValue("status"))

		//якщо при поновленні змінюється статус
		if newStatus != "" {
			//перевіримо чи новий статус доступний для цієї задачі
			rule := taskRules[taskType][t.Status]
			if !rule.Contains(newStatus) {
				http.Redirect(w, r, "/", http.StatusNotFound)
			}

			t.Status = newStatus
			t.ChangedAt.Time = time.Now().UTC()
			t.ChangedBy = user.TelegramID

			stmt, err := dbase.UpdateTaskStatus(cfg)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			_, err = dbase.UpdateTaskStatusExec(stmt, t)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}

	default:

		var t models.DbTasks

		taskIDValue := r.FormValue("id")
		if taskIDValue == "" {
			break
		}

		td.Edit = true

		taskID, err := strconv.Atoi(taskIDValue)
		if err != nil {
			http.Error(w, fmt.Sprintf("Updating task. Can't convert task id to int. Err: %v", err), http.StatusInternalServerError)
			return
		}

		rows, err := dbase.SelectTasksByIDUserTelegramID(cfg, taskID, user.TelegramID)
		if err != nil {
			http.Error(w, fmt.Sprintf("Updating new task. Selecting task by id. Err: %v", err), http.StatusInternalServerError)
			return
		}

		if rows.Next() {
			err := dbase.ScanTask(rows, &t)
			if err != nil {
				http.Error(w, fmt.Sprintf("Updating new task. Scaning task. Err: %v", err), http.StatusInternalServerError)
				return
			}
		} else {
			return
		}
		rows.Close()

		var taskType string

		if t.ToUser == user.TelegramID {
			taskType = "Inbox"
		} else {
			taskType = "Sent"
		}

		rule := taskRules[taskType][t.Status]
		for _, val := range rule {
			td.Actions = append(td.Actions, models.TplActions{
				Action: strings.ToLower(val),
				Alias:  val,
			})
		}

		td.Task = t
		td.ToUser = dbase.GetUserByTelegramID(cfg, t.ToUser)
		td.FromUser = dbase.GetUserByTelegramID(cfg, t.FromUser)
		td.CommentedBy = dbase.GetUserByTelegramID(cfg, t.CommentedBy)
	}

	td.NavBar.LoggedIn = loggedIn
	td.NavBar.User = user
	td.NavBar.MainMenu = getMainMenu("new")

	err = tpl.ExecuteTemplate(w, "task.gohtml", td)
	if err != nil {
		log.Println(err)
	}
}

func userHanlder(w http.ResponseWriter, r *http.Request) {

	var td models.TplUser
	var u models.DbUsers
	var err error

	loggedIn, user := alreadyLoggedIn(w, r, "")
	if !loggedIn {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	}

	tgidString := r.FormValue("id")
	tgid, err := strconv.Atoi(tgidString)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	if user.TelegramID != tgid && user.Admin != 1 {
		http.Error(w, "Access denied", http.StatusNotFound)
		return
	}

	//якщо для поновлення ми отримали того ж користувача що є залогіненим не будемо зе раз шукати його в базі
	//використаємо того що вже є
	if user.TelegramID != tgid {
		u = dbase.GetUserByTelegramID(cfg, tgid)
	}

	//якщо для поновлення ми отримали того ж користувача що є залогіненим
	//чи ми не знайшли по ID такого, то використаємо того що вже є
	if user.TelegramID == tgid || u.ID == 0 {
		u = user
	}

	//на поновлення ми отримала користувача до якого є доcтуп і такий є а базі, то можемо його поновити
	if u.ID != 0 {
		do := r.FormValue("do")

		switch do {
		case "update":

			needUpadte := false

			firstName := r.FormValue("firstName")
			lastName := r.FormValue("lastName")

			if firstName != "" && u.FirstName != firstName {
				needUpadte = true
				u.FirstName = firstName
			}

			if lastName != "" && u.LastName != lastName {
				needUpadte = true
				u.LastName = lastName
			}

			var userpic string

			mf, fh, err := r.FormFile("userpic")
			if err == nil {
				userpic, err = utils.UpdateUserpic(mf, fh, u)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			}

			if userpic != "" && u.Userpic != userpic {
				needUpadte = true
				u.Userpic = userpic
			}

			if needUpadte {
				stmt, err := dbase.UpdateUserData(cfg)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}

				_, err = stmt.Exec(u.FirstName, u.LastName, u.Userpic, u.TelegramID)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			}

			if u.ID == user.ID {
				user = u
			}

			http.Redirect(w, r, fmt.Sprintf("/user?id=%v", u.TelegramID), http.StatusSeeOther)
		}
	}

	td.NavBar.LoggedIn = loggedIn
	td.NavBar.MainMenu = getMainMenu("")
	td.NavBar.User = user

	td.User = u

	err = tpl.ExecuteTemplate(w, "user.gohtml", td)
	if err != nil {
		log.Println(err)
	}
}

func getTasksTabs(taskType string, status string) string {

	if status == "" {
		status = models.TaskStatusNew
	}

	var xs []string

	xs = append(xs, models.TaskStatusNew)
	xs = append(xs, models.TaskStatusStarted)
	xs = append(xs, models.TaskStatusRejected)
	xs = append(xs, models.TaskStatusCompleted)
	xs = append(xs, models.TaskStatusClosed)

	nav := `<div class="row">
	<ul class="nav nav-tabs">`

	for _, val := range xs {
		nav += fmt.Sprintf(`<li class="nav-item">
	<a class="nav-link%v" href="/tasks?type=%v&status=%v">%v</a>
	</li>`, isStatusActive(status, val), taskType, strings.ToLower(val), val)
	}

	nav += fmt.Sprintln(`	
		</ul>
	</div>`)

	return nav
}

func isStatusActive(currentStatus string, status string) string {
	if currentStatus == status {
		return " active"
	}

	return ""
}

func getMainMenu(curremtMenu string) []models.TplMainMenu {

	var mm []models.TplMainMenu
	var m models.TplMainMenu

	m.Link = "/task"
	m.Alias = `<i class="far fa-file"></i> New`
	if curremtMenu == "new" {
		m.Alias += `<span class="sr-only">(current)</span>`
	}
	mm = append(mm, m)

	m.Link = "/tasks?type=inbox"
	m.Alias = "Inbox"
	if curremtMenu == "inbox" {
		m.Alias += `<span class="sr-only">(current)</span>`
	}
	mm = append(mm, m)

	m.Link = "/tasks?type=sent"
	m.Alias = "Sent"
	if curremtMenu == "sent" {
		m.Alias += `<span class="sr-only">(current)</span>`
	}
	mm = append(mm, m)

	return mm
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {

	loggedIn, _ := alreadyLoggedIn(w, r, "")
	if !loggedIn {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	c, err := r.Cookie("session")
	if err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	sessionUUID := c.Value
	stmt, err := dbase.DeleteSessionByUUID(cfg)
	if err != nil {
		log.Println(fmt.Errorf("Can't get Delete session stmt. %v", err))
	}

	_, err = stmt.Exec(sessionUUID)
	if err != nil {
		log.Println(fmt.Errorf("Can't Delete session UUID: %v. %v", sessionUUID, err))
	}

	c.MaxAge = -1
	http.SetCookie(w, c)

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func alreadyLoggedIn(w http.ResponseWriter, r *http.Request, sessionUUID string) (bool, models.DbUsers) {

	var u models.DbUsers
	ok := false

	if time.Now().Sub(lastSessionCleaned) > (time.Duration(sessionLenght) * time.Second) {
		//go cleanSession()
	}

	c, err := r.Cookie("session")
	if err != nil {
		return ok, u
	}

	if sessionUUID == "" {
		sessionUUID = c.Value
	}

	stmt, err := dbase.UpdateSessionLastActivityByUuid(cfg)
	if err != nil {
		return ok, u
	}

	_, err = stmt.Exec(time.Now().UTC(), sessionUUID)
	if err != nil {
		return ok, u
	}

	rows, err := dbase.SelectUsersBySessionUUID(cfg, sessionUUID)

	if err != nil {
		panic(err)
	}

	if rows.Next() {
		err := dbase.ScanUser(rows, &u)
		if err != nil {
			return ok, u
		}
		ok = true
	}
	rows.Close()

	c.MaxAge = sessionLenght
	http.SetCookie(w, c)

	return ok, u
}

func cleanSession() {

	rows, err := dbase.SelectSessions(cfg)
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	var s models.DbSessions

	for rows.Next() {
		err = dbase.ScanSession(rows, &s)
		if err == nil {
			if time.Now().Sub(s.LastActivity.Time) > (time.Duration(sessionLenght) * time.Second) {
				stmt, err := dbase.DeleteSessionByUUID(cfg)
				if err != nil {
					log.Println()
					continue
				}
				_, err = stmt.Exec(s.ID)
				if err != nil {
					log.Println()
					continue
				}
			}
		}
	}

	lastSessionCleaned = time.Now()
}
