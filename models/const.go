package models

const (
	Main     = "Main"
	Users    = "Users"
	Back     = "Back"
	View     = "View"
	All      = "All"
	Requests = "Requests"
	Banned   = "Banned"
	Edit     = "Edit"
	Approve  = "Approve"
	Ban      = "Ban"
	Unban    = "Unban"
	Previous = "Previous"
	Next     = "Next"
	Inbox    = "Inbox"
	Sent     = "Sent"
	New      = "New"
	Save     = "Save"
	Cancel   = "Cancel"
	Start    = "Start"
	Complete = "Complete"
	History  = "History"
	Close    = "Close"
	Reject   = "Reject"
	Comment  = "Comment"
)

const (
	MenuMain             = Main
	MenuUsers            = Users
	MenuUsersView        = Users + View
	MenuUsersEdit        = Users + Edit
	MenuUsersEditApprove = Users + Edit + Approve
	MenuUsersEditBan     = Users + Edit + Ban
	MenuUsersEditUnban   = Users + Edit + Unban
	MenuInbox            = Inbox
	MenuInboxNew         = Inbox + New
	MenuInboxStarted     = Inbox + TaskStatusStarted
	MenuInboxCompleted   = Inbox + TaskStatusCompleted
	MenuInboxRejected    = Inbox + TaskStatusRejected
	MenuInboxClosed      = Inbox + TaskStatusClosed
	MenuSent             = Sent
	MenuSentNew          = Sent + New
	MenuSentStarted      = Sent + TaskStatusStarted
	MenuSentCompleted    = Sent + TaskStatusCompleted
	MenuSentRejected     = Sent + TaskStatusRejected
	MenuSentClosed       = Sent + TaskStatusClosed
	MenuNew              = New
	MenuComment          = Comment
)

const (
	NewTaskStepUser = iota
	NewTaskStepTitle
	NewTaskStepDescription
	NewTaskStepSaveToDB
)

const (
	TaskStatusNew       = "New"
	TaskStatusStarted   = "Started"
	TaskStatusRejected  = "Rejected"
	TaskStatusCompleted = "Completed"
	TaskStatusClosed    = "Closed"
)

const (
	UserRequested = "Requested"
	UserApprowed  = "Approwed"
	UserBanned    = "Banned"
)

const (
	NewUserRequest = "NewUserRequest"
	NewUserCancel  = "NewUserCancel"
	NewUserAccept  = "NewUserAccept"
	NewUserDecline = "NewUserDecline"
)
