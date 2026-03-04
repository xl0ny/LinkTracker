package application

type ChatRepository interface {
	AddChat(id int) error
}
