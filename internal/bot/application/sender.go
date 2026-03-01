package application

type sender interface {
	SendMessage(chatid int64, message string) error
}
