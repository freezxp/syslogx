package postgres

import (
	"context"
	"fmt"

	"github.com/freezxp/syslogx/backend/internal/controlstore"
)

func (s *Store) ListSavedSearches(ctx context.Context, owner string) ([]controlstore.SavedSearch, error) {
	rows, err := s.pool.Query(ctx, `SELECT id,name,description,query,default_time_range,created_by,created_at,updated_at FROM saved_searches WHERE created_by=$1 ORDER BY name,id`, owner)
	if err != nil {
		return nil, fmt.Errorf("list saved searches: %w", err)
	}
	defer rows.Close()
	out := make([]controlstore.SavedSearch, 0)
	for rows.Next() {
		var v controlstore.SavedSearch
		if err := rows.Scan(&v.ID, &v.Name, &v.Description, &v.Query, &v.DefaultRange, &v.CreatedBy, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan saved search: %w", err)
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate saved searches: %w", err)
	}
	return out, nil
}

func (s *Store) CreateSavedSearch(ctx context.Context, v controlstore.SavedSearch) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO saved_searches(id,name,description,query,default_time_range,created_by,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, v.ID, v.Name, v.Description, v.Query, v.DefaultRange, v.CreatedBy, v.CreatedAt, v.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create saved search: %w", err)
	}
	return nil
}

func (s *Store) DeleteSavedSearch(ctx context.Context, owner, id string) (bool, error) {
	tag, err := s.pool.Exec(ctx, `DELETE FROM saved_searches WHERE id=$1 AND created_by=$2`, id, owner)
	if err != nil {
		return false, fmt.Errorf("delete saved search: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}
