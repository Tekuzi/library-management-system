package handlers

import (
	"html/template"
	"log"
	"net/http"

	"library-management-system/internal/middleware"
)

// IndexHandler obsługuje stronę główną
type IndexHandler struct {
	homeTemplate    *template.Template
	catalogTemplate *template.Template
}

// NewIndexHandler tworzy nowy handler strony głównej
func NewIndexHandler() *IndexHandler {
	homeTmpl, err := template.ParseFiles("internal/templates/home.html")
	if err != nil {
		log.Printf("Błąd ładowania szablonu home.html: %v", err)
	}

	catalogTmpl, err := template.ParseFiles("internal/templates/catalog.html")
	if err != nil {
		log.Printf("Błąd ładowania szablonu catalog.html: %v", err)
	}

	return &IndexHandler{
		homeTemplate:    homeTmpl,
		catalogTemplate: catalogTmpl,
	}
}

// ServeHTTP obsługuje żądanie GET /
func (h *IndexHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.homeTemplate == nil {
		http.Error(w, "Szablon strony głównej nie został załadowany", http.StatusInternalServerError)
		return
	}

	session := middleware.GetSessionFromContext(r.Context())
	data := NewTemplateData(session)

	if err := h.homeTemplate.Execute(w, data); err != nil {
		log.Printf("Błąd renderowania strony głównej: %v", err)
		http.Error(w, "Błąd renderowania strony", http.StatusInternalServerError)
		return
	}
}
