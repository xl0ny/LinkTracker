package db

import (
	"context"
	"fmt"
)

type reposiotory struct {
	Chats map[int64]struct{}
}

func (r *reposiotory) NewRepository() *reposiotory {
	return &reposiotory{}
}

func (r *reposiotory) AddChat(ctx context.Context, id int64) error {
	if !r.exists(id) {
		r.Chats[id] = struct{}{}
		return nil
	}
	return fmt.Errorf("chat already exists")

}

func (r *reposiotory) DeleteChat(ctx context.Context, id int64) error {
	if r.exists(id) {
		delete(r.Chats, id)
		return nil
	}
	return fmt.Errorf("chat not found")
}

func (r *reposiotory) exists(id int64) bool {
	_, ok := r.Chats[id]
	return ok
}
