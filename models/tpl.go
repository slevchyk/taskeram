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

//TasksRow is a part of TplTasks struct for levels.gohtml
type TasksRow struct {
	Number int
	Tasks DbTasks
}

//TplTasks data type for tasks.gohtml
type TplTasks struct {
	NavBar TplNavBar
	Tabs template.HTML
	Rows []TasksRow
}

type TplLogin struct {
	Users []DbUsers
}

type TplActions struct {
	Action string
	Alias string
}

type TplTask struct {
	NavBar TplNavBar
	Edit bool
	Task DbTasks
	Actions []TplActions
	Users []DbUsers
}

type TplIndex struct {
	NavBar TplNavBar
}
