package domain

import "time"

type Link struct {
	URL         string    `json:"url"`
	Tags        []string  `json:"tags,omitempty"`
	Filters     []string  `json:"filters,omitempty"`
	LastUpdated time.Time `json:"last_updated,omitempty"`
}
