package buttons

import (
	"github.com/slevchyk/taskeram/models"
	"gopkg.in/telegram-bot-api.v4"
)

var Next = tgbotapi.NewKeyboardButton(models.Next)
var Users = tgbotapi.NewKeyboardButton(models.Users)
var Back = tgbotapi.NewKeyboardButton(models.Back)
var View = tgbotapi.NewKeyboardButton(models.View)
var All = tgbotapi.NewKeyboardButton(models.All)
var Requests = tgbotapi.NewKeyboardButton(models.Requests)
var Banned = tgbotapi.NewKeyboardButton(models.Banned)
var Edit = tgbotapi.NewKeyboardButton(models.Edit)
var Approve = tgbotapi.NewKeyboardButton(models.Approve)
var Ban = tgbotapi.NewKeyboardButton(models.Ban)
var Unban = tgbotapi.NewKeyboardButton(models.Unban)
var Inbox = tgbotapi.NewKeyboardButton(models.Inbox)
var Sent = tgbotapi.NewKeyboardButton(models.Sent)
var New = tgbotapi.NewKeyboardButton(models.New)
var Started = tgbotapi.NewKeyboardButton(models.TaskStatusStarted)
var Rejected = tgbotapi.NewKeyboardButton(models.TaskStatusRejected)
var Completed = tgbotapi.NewKeyboardButton(models.TaskStatusCompleted)
var Closed = tgbotapi.NewKeyboardButton(models.TaskStatusClosed)
var Save = tgbotapi.NewKeyboardButton(models.Save)
var Cancel = tgbotapi.NewKeyboardButton(models.Cancel)
var Start = tgbotapi.NewKeyboardButton(models.Start)
var Complete = tgbotapi.NewKeyboardButton(models.Complete)
var History = tgbotapi.NewKeyboardButton(models.History)
var Close = tgbotapi.NewKeyboardButton(models.Close)
var Reject = tgbotapi.NewKeyboardButton(models.Reject)
