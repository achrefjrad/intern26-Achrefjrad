package models

import "time"

type FilterSchema struct {
	Subcategory string              `json:"subcategory"`
	Filters     map[string][]string `json:"filters"`
	UpdatedAt   time.Time           `json:"updatedAt"`
}
