package main

import (
	"database/sql"
	"fmt"
	"github.com/satori/go.uuid"
	"github.com/slevchyk/taskeram/dbase"
	"github.com/slevchyk/taskeram/models"
	"gopkg.in/telegram-bot-api.v4"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func startWebApp() {

	http.Handle("/assets/", http.StripPrefix("/assets", http.FileServer(http.Dir("./assets"))))
	http.HandleFunc("/", tasksHandler)
	http.HandleFunc("/login", loginHandler)
	http.HandleFunc("/auth", authHandler)
	//http.HandleFunc("/logout", logoutHandler)
	http.HandleFunc("/tasks", tasksHandler)
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		panic(err)
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

func loginHandler(w	 http.ResponseWriter, r *http.Request) {

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

	var (
		td   models.TplTasks
		sr   []models.TaskRow
		t    models.DbTasks
		i    int
		rows *sql.Rows
		err  error
	)

	loggedIn, user := alreadyLoggedIn(w, r)

	td.NavBar.LoggedIn = loggedIn
	td.NavBar.User = user

	if !loggedIn {
		http.Redirect(w, r, "/login", http.StatusBadRequest)
	}

	taskStatus := r.FormValue("status")
	taskStatus = strings.Title(taskStatus)
	if taskStatus == "" {
		taskStatus = models.TaskStatusNew
	}

	taskType := r.FormValue("type")
	switch taskType {
	case "inbox":
		rows, err = dbase.SelectInboxTasks(cfg, 601192901, taskStatus)
	case "sent":
		rows, err = dbase.SelectSentTasks(cfg, 601192901, taskStatus)
	default:
		rows, err = dbase.SelectInboxTasks(cfg, 601192901, taskStatus)
	}

	if err != nil {
		log.Println(err)
	}

	if rows.Next() {
		err := dbase.ScanTask(rows, &t)
		if err != nil {
			log.Println(fmt.Errorf("Scan inbox task webapp: %v", err))
		}

		i++
		sr = append(sr, models.TaskRow{Number: i, Tasks: t})
	}
	rows.Close()

	td.NavBar.MainMenu = getTasksMainMenu()
	td.Tabs = template.HTML(getTabs(taskType, taskStatus))
	td.Rows = sr

	err = tpl.ExecuteTemplate(w, "inbox.gohtml", td)
	if err != nil {
		log.Println(err)
	}
}

func getTabs(taskType string, status string) string {

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

func getTasksMainMenu() []models.TplMainMenu {

	var mm []models.TplMainMenu

	mm = append(mm, struct {
		Link  string
		Alias string
	}{
		Link:  "/tasks?type=inbox",
		Alias: "Inbox",
	})

	mm = append(mm, struct {
		Link  string
		Alias string
	}{
		Link:  "/tasks?type=sent",
		Alias: "Sent",
	})

	return mm
}


//func logoutHandler(w http.ResponseWriter, r *http.Request) {
//
//	if !alreadyLoggedIn(w, r) {
//		http.Redirect(w, r, "/", http.StatusSeeOther)
//		return
//	}
//
//	c, err := r.Cookie("session")
//	if err != nil {
//		http.Redirect(w, r, "/", http.StatusSeeOther)
//		return
//	}
//
//	sessionID := c.Value
//	_, err = db.Query(dbase.DeleteSessionByUUID(), sessionID)
//	if err != nil {
//		panic(err)
//	}
//
//	c.MaxAge = -1
//	http.SetCookie(w, c)
//
//	http.Redirect(w, r, "/", http.StatusSeeOther)
//}

func alreadyLoggedIn(w http.ResponseWriter, r *http.Request) (bool, models.DbUsers) {

	var u models.DbUsers

	if time.Now().Sub(lastSessionCleaned) > (time.Duration(sessionLenght) * time.Second) {
		go cleanSession()
	}

	c, err := r.Cookie("session")
	if err != nil {
		return false, u
	}

	sessionUUID := c.Value

	stmt, err := dbase.UpdateSessionLastActivityByUuid(cfg)
	if err != nil {
		return false, u
	}

	_, err = stmt.Exec(time.Now().UTC(), sessionUUID)
	if err != nil {
		return false, u
	}

	rows, err := dbase.SelectUsersBySessionUUID(cfg, sessionUUID)

	if err != nil {
		panic(err)
	}

	ok := false
	if rows.Next() {
		ok = true
		err := dbase.ScanUser(rows, &u)
		if err != nil {
			return false, u
		}
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
				stmt, err := dbase.DeleteSessionByID(cfg)
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
