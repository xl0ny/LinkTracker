package application

type ChatRepository interface {
	AddChat(id int) error
	DeleteChat(id int) error
}

type chatUC struct {
	repo ChatRepository
}

func (uc *chatUC) chatRegistration(id int64) error {
	return uc.repo.AddChat(id)
}

func (uc *chatUC) chatDelition(id int) error {
	return uc.repo.DeleteChat(id)
}
