package domain

import "time"

// LinkCheckUpdate — одно обнаруженное изменение (например, новое issue в репозитории).
type LinkCheckUpdate struct {
	Description string
	Author      string
	At          time.Time
}

type LinkCheckOutcome struct {
	Changed     bool
	Latest      time.Time
	Description string
	Author      string
	Updates     []LinkCheckUpdate
}
