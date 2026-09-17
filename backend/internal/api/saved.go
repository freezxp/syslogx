package api

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/freezxp/syslogx/backend/internal/controlstore"
)

type savedSearch = controlstore.SavedSearch

func (s *Server) listSaved(w http.ResponseWriter, r *http.Request) {
	owner, _ := s.auth.get(r)
	if s.savedStore != nil {
		out, err := s.savedStore.ListSavedSearches(r.Context(), owner.User)
		if err != nil {
			s.logger.Error("list saved searches", "error", err)
			writeJSON(w, 503, map[string]any{"code": "control_store_unavailable"})
			return
		}
		writeJSON(w, 200, map[string]any{"data": out})
		return
	}
	s.savedMu.RLock()
	out := make([]savedSearch, 0, len(s.saved))
	for _, v := range s.saved {
		if v.CreatedBy == owner.User {
			out = append(out, v)
		}
	}
	s.savedMu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	writeJSON(w, 200, map[string]any{"data": out})
}

func (s *Server) createSaved(w http.ResponseWriter, r *http.Request) {
	var v savedSearch
	if jsonDecode(r, &v) != nil || strings.TrimSpace(v.Name) == "" || len(v.Name) > 120 || len(v.Description) > 1024 || len(v.Query) > 4096 || len(v.DefaultRange) > 32 {
		writeJSON(w, 400, map[string]any{"code": "invalid_saved_search"})
		return
	}
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		s.logger.Error("saved search id generation", "error", err)
		writeJSON(w, 500, map[string]any{"code": "internal_error"})
		return
	}
	owner, _ := s.auth.get(r)
	now := time.Now().UTC()
	v.ID = hex.EncodeToString(id[:])
	v.CreatedBy = owner.User
	v.CreatedAt = now
	v.UpdatedAt = now
	if v.DefaultRange == "" {
		v.DefaultRange = "1h"
	}
	if s.savedStore != nil {
		if err := s.savedStore.CreateSavedSearch(r.Context(), v); err != nil {
			s.logger.Error("create saved search", "error", err)
			writeJSON(w, 503, map[string]any{"code": "control_store_unavailable"})
			return
		}
	} else {
		s.savedMu.Lock()
		s.saved[v.ID] = v
		s.savedMu.Unlock()
	}
	writeJSON(w, 201, v)
}

func (s *Server) deleteSaved(w http.ResponseWriter, r *http.Request) {
	owner, _ := s.auth.get(r)
	id := r.PathValue("id")
	if s.savedStore != nil {
		found, err := s.savedStore.DeleteSavedSearch(r.Context(), owner.User, id)
		if err != nil {
			s.logger.Error("delete saved search", "error", err)
			writeJSON(w, 503, map[string]any{"code": "control_store_unavailable"})
			return
		}
		if !found {
			writeJSON(w, 404, map[string]any{"code": "not_found"})
			return
		}
	} else {
		s.savedMu.Lock()
		v, ok := s.saved[id]
		if ok && v.CreatedBy == owner.User {
			delete(s.saved, id)
		}
		s.savedMu.Unlock()
		if !ok || v.CreatedBy != owner.User {
			writeJSON(w, 404, map[string]any{"code": "not_found"})
			return
		}
	}
	w.WriteHeader(204)
}
