package domain

import "time"

type LinkCheckOutcome struct {
	Changed     bool
	Latest      time.Time
	Description string
}
