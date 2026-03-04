package db

type reposiotory struct {
	Chats map[int]struct{}
}

func (r *reposiotory) NewRepository() *reposiotory {
	return &reposiotory{}
}

func (r *reposiotory) AddChat(id int) error {
	if !r.exists(id) {
		r.Chats[id] = struct{}{}
		return nil
	}

}

func (r *reposiotory) exists(id int) bool {
	_, ok := r.Chats[id]
	return ok
}
