package application

type chatUC struct {
	repo ChatRepository
}

func (uc *chatUC) chatRegistration(id int) error {
	return uc.repo.AddChat(id)
}
