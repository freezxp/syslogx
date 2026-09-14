package api

import (
	"fmt"
	"net/http"
	"sort"
	"time"
)

type savedSearch struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	Query        string    `json:"query"`
	DefaultRange string    `json:"default_time_range"`
	CreatedBy    string    `json:"created_by"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (s *Server) listSaved(w http.ResponseWriter, r *http.Request) {
	s.savedMu.RLock()
	out := make([]savedSearch, 0, len(s.saved))
	for _, v := range s.saved {
		out = append(out, v)
	}
	s.savedMu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	writeJSON(w, 200, map[string]any{"data": out})
}
func (s *Server) createSaved(w http.ResponseWriter, r *http.Request) {
	var v savedSearch
	if jsonDecode(r, &v) != nil || v.Name == "" || len(v.Name) > 120 || len(v.Query) > 4096 {
		writeJSON(w, 400, map[string]any{"code": "invalid_saved_search"})
		return
	}
	now := time.Now().UTC()
	v.ID = fmt.Sprintf("search-%d", now.UnixNano())
	v.CreatedBy = "admin"
	v.CreatedAt = now
	v.UpdatedAt = now
	if v.DefaultRange == "" {
		v.DefaultRange = "1h"
	}
	s.savedMu.Lock()
	s.saved[v.ID] = v
	s.savedMu.Unlock()
	writeJSON(w, 201, v)
}
func (s *Server) deleteSaved(w http.ResponseWriter, r *http.Request) {
	s.savedMu.Lock()
	delete(s.saved, r.PathValue("id"))
	s.savedMu.Unlock()
	w.WriteHeader(204)
}
