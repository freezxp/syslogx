package controlstore

import (
	"context"
	"time"
)

type SavedSearch struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	Query        string    `json:"query"`
	DefaultRange string    `json:"default_time_range"`
	CreatedBy    string    `json:"created_by"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type SavedSearchStore interface {
	ListSavedSearches(context.Context, string) ([]SavedSearch, error)
	CreateSavedSearch(context.Context, SavedSearch) error
	DeleteSavedSearch(context.Context, string, string) (bool, error)
}
