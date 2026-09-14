package api

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type session struct {
	User, Role string
	Expires    time.Time
}
type authManager struct {
	enabled  bool
	hash     []byte
	mu       sync.RWMutex
	sessions map[string]session
}

func newAuthManager() *authManager {
	p := os.Getenv("SYSLOGX_ADMIN_PASSWORD")
	a := &authManager{enabled: p != "", sessions: map[string]session{}}
	if p != "" {
		a.hash, _ = bcrypt.GenerateFromPassword([]byte(p), bcrypt.DefaultCost)
	}
	return a
}
func (a *authManager) issue() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	t := base64.RawURLEncoding.EncodeToString(b)
	a.mu.Lock()
	a.sessions[t] = session{"admin", "admin", time.Now().Add(12 * time.Hour)}
	a.mu.Unlock()
	return t
}
func (a *authManager) get(r *http.Request) (session, bool) {
	if !a.enabled {
		return session{"development", "admin", time.Now().Add(time.Hour)}, true
	}
	c, e := r.Cookie("syslogx_session")
	if e != nil {
		return session{}, false
	}
	a.mu.RLock()
	s, ok := a.sessions[c.Value]
	a.mu.RUnlock()
	return s, ok && time.Now().Before(s.Expires)
}
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var in struct{ Username, Password string }
	if jsonDecode(r, &in) != nil || in.Username != "admin" || !s.auth.enabled || bcrypt.CompareHashAndPassword(s.auth.hash, []byte(in.Password)) != nil {
		writeJSON(w, 401, map[string]any{"code": "invalid_credentials"})
		return
	}
	token := s.auth.issue()
	http.SetCookie(w, &http.Cookie{Name: "syslogx_session", Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: r.TLS != nil, MaxAge: 43200})
	writeJSON(w, 200, map[string]any{"username": "admin", "role": "admin"})
}
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if c, e := r.Cookie("syslogx_session"); e == nil {
		s.auth.mu.Lock()
		delete(s.auth.sessions, c.Value)
		s.auth.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{Name: "syslogx_session", Path: "/", MaxAge: -1, HttpOnly: true})
	w.WriteHeader(204)
}
func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	u, ok := s.auth.get(r)
	if !ok {
		writeJSON(w, 401, map[string]any{"code": "unauthorized"})
		return
	}
	writeJSON(w, 200, map[string]any{"username": u.User, "role": u.Role, "auth_enabled": s.auth.enabled})
}
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.auth.enabled || r.URL.Path == "/api/v1/auth/login" || !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		if _, ok := s.auth.get(r); !ok {
			writeJSON(w, 401, map[string]any{"code": "unauthorized"})
			return
		}
		next.ServeHTTP(w, r)
	})
}
func jsonDecode(r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<20)
	return json.NewDecoder(r.Body).Decode(v)
}
