package httpapi

import (
	"html/template"
	"net/http"
	"path/filepath"
)

type indexData struct {
	Admin bool
	About bool
}

func (s *Server) Index(w http.ResponseWriter, r *http.Request) {
	s.renderIndex(w, indexData{})
}

func (s *Server) AdminIndex(w http.ResponseWriter, r *http.Request) {
	s.renderIndex(w, indexData{Admin: true})
}

func (s *Server) AboutIndex(w http.ResponseWriter, r *http.Request) {
	s.renderIndex(w, indexData{About: true})
}

func (s *Server) renderIndex(w http.ResponseWriter, data indexData) {
	tmplPath := filepath.Join(s.workspace, "templates", "index.html")
	tmpl, err := template.ParseFiles(tmplPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Success: false, Error: "template error"})
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, data); err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Success: false, Error: "template execute error"})
		return
	}
}
