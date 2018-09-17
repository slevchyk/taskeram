package models

import "html/template"

type TplMainMenu struct {
	Link  string
	Alias string
}

type TplNavBar struct {
	LoggedIn bool
	User     DbUsers
	MainMenu []TplMainMenu
}

//TaskRow is a part of TplTasks struct for levels.gohtml
type TaskRow struct {
	Number int
	Tasks DbTasks
}

//TplTasks data type for tasks.gohtml
type TplTasks struct {
	NavBar TplNavBar
	Tabs template.HTML
	Rows []TaskRow
}

type TplLogin struct {
	Users []DbUsers
}
