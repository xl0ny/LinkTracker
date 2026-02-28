package domain

type Action struct {
	ChatID  int64
	Command string
	Text    string
}
