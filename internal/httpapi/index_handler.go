package httpapi

import (
	"html/template"
	"net/http"
	"path/filepath"
)

type indexData struct {
	Admin bool
}

func (s *Server) Index(w http.ResponseWriter, r *http.Request) {
	s.renderIndex(w, false)
}

func (s *Server) AdminIndex(w http.ResponseWriter, r *http.Request) {
	s.renderIndex(w, true)
}

func (s *Server) renderIndex(w http.ResponseWriter, admin bool) {
	tmplPath := filepath.Join(s.workspace, "templates", "index.html")
	tmpl, err := template.ParseFiles(tmplPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Success: false, Error: "template error"})
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, indexData{Admin: admin}); err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Success: false, Error: "template execute error"})
		return
	}
}
