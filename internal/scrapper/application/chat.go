package application

import "context"

type ChatRepository interface {
	AddChat(ctx context.Context, id int64) error
	DeleteChat(ctx context.Context, id int64) error
	AddLink(ctx context.Context, id int64) error
}

type chatUC struct {
	repo ChatRepository
}

func (uc *chatUC) chatRegistration(ctx context.Context, id int64) error {
	return uc.repo.AddChat(ctx, id)
}

func (uc *chatUC) chatDelition(ctx context.Context, id int64) error {
	return uc.repo.DeleteChat(ctx, id)
}

func (uc *chatUC) linkAddment(ctx context.Context, chatId int64) error {

}

// func
