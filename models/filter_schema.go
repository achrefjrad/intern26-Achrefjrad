package models

import "time"

type FilterSchema struct {
	Subcategory string              `json:"subcategory"`
	Filters     map[string][]string `json:"filters"`
	UpdatedAt   time.Time           `json:"updatedAt"`
}

type FilterValue struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

type FilterGroup struct {
	Name   string        `json:"name"`
	Key    string        `json:"key"`
	Values []FilterValue `json:"values"`
}

type EnrichedFilterSchema struct {
	Subcategory string        `json:"subcategory"`
	Category    string        `json:"category,omitempty"`
	Filters     []FilterGroup `json:"filters"`
	UpdatedAt   time.Time     `json:"updatedAt"`
}
