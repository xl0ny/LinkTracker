package domain

type Chat struct {
	ID    *int64 `json:"id,omitempty"`
	Links []Link
}
