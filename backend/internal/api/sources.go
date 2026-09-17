package api

import "net/http"

type sourceView struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Protocol string `json:"protocol"`
	Address  string `json:"address"`
	Parser   string `json:"parser"`
	Enabled  bool   `json:"enabled"`
	Status   string `json:"status"`
}

func (s *Server) listSources(w http.ResponseWriter, _ *http.Request) {
	out := make([]sourceView, 0, len(s.sources)+1)
	for _, src := range s.sources {
		status := "disabled"
		if src.Enabled {
			status = "running"
			if !s.accepting.Load() {
				status = "draining"
			}
		}
		out = append(out, sourceView{ID: src.ID, Name: src.Name, Protocol: src.Protocol, Address: src.Address, Parser: src.Parser, Enabled: src.Enabled, Status: status})
	}
	out = append(out, sourceView{ID: "http-json", Name: "HTTP JSON", Protocol: "http", Address: s.http.Addr, Parser: "json", Enabled: true, Status: "running"})
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
}
