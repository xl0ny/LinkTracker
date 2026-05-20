package model

import "time"

type Link struct {
	URL         string
	Tags        []string
	Filters     []string
	LastUpdated time.Time
}
