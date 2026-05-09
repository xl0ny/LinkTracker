package domain

import "errors"

var (
	ErrChatAlreadyExists = errors.New("chat already exists")
	ErrChatNotFound      = errors.New("chat not found")
	ErrLinkAlreadyExists = errors.New("link already exists")
	ErrLinkNotFound      = errors.New("link not found")
	ErrTagAlreadyExists  = errors.New("tag already exists")
	ErrTagNotFound       = errors.New("tag not found")
)
