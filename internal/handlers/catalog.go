package handlers

import (
	"html/template"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"library-management-system/internal/firebase"
	"library-management-system/internal/middleware"
	"library-management-system/internal/models"
)

// CatalogHandler obsługuje zarządzanie katalogiem książek
type CatalogHandler struct {
	listTemplate *template.Template
	formTemplate *template.Template
}

// NewCatalogHandler tworzy nowy handler katalogu
func NewCatalogHandler() *CatalogHandler {
	funcMap := template.FuncMap{
		"sub": func(a, b int) int {
			return a - b
		},
		"add": func(a, b int) int {
			return a + b
		},
		"mkRange": func(start, end int) []int {
			result := make([]int, end-start+1)
			for i := range result {
				result[i] = start + i
			}
			return result
		},
	}

	listTmpl, err := template.New("catalog_list.html").Funcs(funcMap).ParseFiles("internal/templates/staff/catalog_list.html")
	if err != nil {
		log.Printf("Błąd ładowania szablonu catalog_list.html: %v", err)
	}

	formTmpl, err := template.ParseFiles("internal/templates/staff/catalog_form.html")
	if err != nil {
		log.Printf("Błąd ładowania szablonu catalog_form.html: %v", err)
	}

	return &CatalogHandler{
		listTemplate: listTmpl,
		formTemplate: formTmpl,
	}
}

// ListBooks wyświetla listę wszystkich książek (GET /staff/catalog)
func (h *CatalogHandler) ListBooks(w http.ResponseWriter, r *http.Request) {
	if h.listTemplate == nil {
		http.Error(w, "Szablon nie został załadowany", http.StatusInternalServerError)
		return
	}

	// Parametry paginacji i sortowania
	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}

	limit := 20
	offset := (page - 1) * limit

	sortBy := r.URL.Query().Get("sort")
	sortOrder := r.URL.Query().Get("order")
	if sortOrder == "" {
		sortOrder = "asc"
	}

	// Pobierz książki z paginacją
	books, totalCount, err := firebase.GlobalClient.ListBooksWithPagination(limit, offset, sortBy, sortOrder)
	if err != nil {
		log.Printf("Błąd pobierania książek: %v", err)
		http.Error(w, "Błąd pobierania książek", http.StatusInternalServerError)
		return
	}

	// Oblicz liczbę stron
	totalPages := (totalCount + limit - 1) / limit

	session := middleware.GetSessionFromContext(r.Context())
	data := NewTemplateData(session)
	data["Books"] = books
	data["CurrentPage"] = page
	data["TotalPages"] = totalPages
	data["TotalCount"] = totalCount
	data["SortBy"] = sortBy
	data["SortOrder"] = sortOrder

	if err := h.listTemplate.Execute(w, data); err != nil {
		log.Printf("Błąd renderowania szablonu: %v", err)
		http.Error(w, "Błąd renderowania strony", http.StatusInternalServerError)
	}
}

// SearchBooks wyszukuje książki (GET /staff/catalog/search)
func (h *CatalogHandler) SearchBooks(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")

	log.Printf("Wyszukiwanie: query='%s'", query)

	books, err := firebase.GlobalClient.SearchBooks(query)
	if err != nil {
		log.Printf("Błąd wyszukiwania książek: %v", err)
		http.Error(w, "Błąd wyszukiwania", http.StatusInternalServerError)
		return
	}

	log.Printf("Znaleziono %d książek dla zapytania '%s'", len(books), query)

	session := middleware.GetSessionFromContext(r.Context())
	data := NewTemplateData(session)
	data["Books"] = books
	data["SearchQuery"] = query

	// Renderuj tylko fragment tabeli dla htmx
	h.renderBooksTable(w, data)
}

// ShowNewBookForm wyświetla formularz dodawania książki (GET /staff/catalog/new)
func (h *CatalogHandler) ShowNewBookForm(w http.ResponseWriter, r *http.Request) {
	if h.formTemplate == nil {
		http.Error(w, "Szablon nie został załadowany", http.StatusInternalServerError)
		return
	}

	session := middleware.GetSessionFromContext(r.Context())
	data := NewTemplateData(session)
	data["Action"] = "create"
	data["Book"] = &models.Book{}
	data["Categories"] = getBookCategories()

	if err := h.formTemplate.Execute(w, data); err != nil {
		log.Printf("Błąd renderowania formularza: %v", err)
		http.Error(w, "Błąd renderowania strony", http.StatusInternalServerError)
	}
}

// CreateBook tworzy nową książkę (POST /staff/catalog)
func (h *CatalogHandler) CreateBook(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Błąd parsowania formularza", http.StatusBadRequest)
		return
	}

	// Walidacja
	isbn := r.FormValue("isbn")
	if isbn == "" {
		h.renderFormError(w, r, "ISBN jest wymagany", nil)
		return
	}

	// Sprawdź czy ISBN już istnieje
	existingBook, err := firebase.GlobalClient.GetBookByISBN(isbn)
	if err != nil {
		log.Printf("Błąd sprawdzania ISBN: %v", err)
		h.renderFormError(w, r, "Błąd sprawdzania ISBN", nil)
		return
	}
	if existingBook != nil {
		h.renderFormError(w, r, "Książka z tym ISBN już istnieje", nil)
		return
	}

	// Parsuj pozostałe dane
	totalCopies, _ := strconv.Atoi(r.FormValue("total_copies"))
	publicationYear, _ := strconv.Atoi(r.FormValue("publication_year"))

	book := &models.Book{
		ISBN:            isbn,
		Title:           r.FormValue("title"),
		Author:          r.FormValue("author"),
		Publisher:       r.FormValue("publisher"),
		PublicationYear: publicationYear,
		Category:        r.FormValue("category"),
		Description:     r.FormValue("description"),
		TotalCopies:     totalCopies,
		AvailableCopies: totalCopies, // Na początku wszystkie dostępne
	}

	// Walidacja podstawowa
	if book.Title == "" {
		h.renderFormError(w, r, "Tytuł jest wymagany", book)
		return
	}
	if book.Author == "" {
		h.renderFormError(w, r, "Autor jest wymagany", book)
		return
	}
	if book.TotalCopies < 1 {
		h.renderFormError(w, r, "Liczba egzemplarzy musi być większa od 0", book)
		return
	}

	// Zapisz książkę
	if err := firebase.GlobalClient.CreateBook(book); err != nil {
		log.Printf("Błąd tworzenia książki: %v", err)
		h.renderFormError(w, r, "Błąd zapisywania książki: "+err.Error(), book)
		return
	}

	// Przekieruj do listy książek (htmx)
	w.Header().Set("HX-Redirect", "/staff/catalog")
	w.WriteHeader(http.StatusOK)
}

// ShowEditBookForm wyświetla formularz edycji książki (GET /staff/catalog/{id}/edit)
func (h *CatalogHandler) ShowEditBookForm(w http.ResponseWriter, r *http.Request) {
	if h.formTemplate == nil {
		http.Error(w, "Szablon nie został załadowany", http.StatusInternalServerError)
		return
	}

	bookID := chi.URLParam(r, "id")
	if bookID == "" {
		http.Error(w, "Brak ID książki", http.StatusBadRequest)
		return
	}

	book, err := firebase.GlobalClient.GetBook(bookID)
	if err != nil {
		log.Printf("Błąd pobierania książki: %v", err)
		http.Error(w, "Książka nie została znaleziona", http.StatusNotFound)
		return
	}

	session := middleware.GetSessionFromContext(r.Context())
	data := NewTemplateData(session)
	data["Action"] = "edit"
	data["Book"] = book
	data["Categories"] = getBookCategories()

	if err := h.formTemplate.Execute(w, data); err != nil {
		log.Printf("Błąd renderowania formularza: %v", err)
		http.Error(w, "Błąd renderowania strony", http.StatusInternalServerError)
	}
}

// UpdateBook aktualizuje książkę (PUT /staff/catalog/{id})
func (h *CatalogHandler) UpdateBook(w http.ResponseWriter, r *http.Request) {
	bookID := chi.URLParam(r, "id")
	if bookID == "" {
		http.Error(w, "Brak ID książki", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Błąd parsowania formularza", http.StatusBadRequest)
		return
	}

	// Pobierz istniejącą książkę
	existingBook, err := firebase.GlobalClient.GetBook(bookID)
	if err != nil {
		log.Printf("Błąd pobierania książki: %v", err)
		http.Error(w, "Książka nie została znaleziona", http.StatusNotFound)
		return
	}

	// Parsuj dane
	totalCopies, _ := strconv.Atoi(r.FormValue("total_copies"))
	publicationYear, _ := strconv.Atoi(r.FormValue("publication_year"))

	// Oblicz różnicę w dostępnych egzemplarzach
	copiesDiff := totalCopies - existingBook.TotalCopies
	newAvailableCopies := existingBook.AvailableCopies + copiesDiff
	if newAvailableCopies < 0 {
		newAvailableCopies = 0
	}
	if newAvailableCopies > totalCopies {
		newAvailableCopies = totalCopies
	}

	book := &models.Book{
		ID:              bookID,
		ISBN:            r.FormValue("isbn"),
		Title:           r.FormValue("title"),
		Author:          r.FormValue("author"),
		Publisher:       r.FormValue("publisher"),
		PublicationYear: publicationYear,
		Category:        r.FormValue("category"),
		Description:     r.FormValue("description"),
		TotalCopies:     totalCopies,
		AvailableCopies: newAvailableCopies,
		CreatedAt:       existingBook.CreatedAt,
	}

	// Walidacja
	if book.Title == "" {
		h.renderFormError(w, r, "Tytuł jest wymagany", book)
		return
	}
	if book.Author == "" {
		h.renderFormError(w, r, "Autor jest wymagany", book)
		return
	}
	if book.TotalCopies < 1 {
		h.renderFormError(w, r, "Liczba egzemplarzy musi być większa od 0", book)
		return
	}

	// Aktualizuj książkę
	if err := firebase.GlobalClient.UpdateBook(bookID, book); err != nil {
		log.Printf("Błąd aktualizacji książki: %v", err)
		h.renderFormError(w, r, "Błąd zapisywania książki: "+err.Error(), book)
		return
	}

	// Przekieruj do listy książek
	w.Header().Set("HX-Redirect", "/staff/catalog")
	w.WriteHeader(http.StatusOK)
}

// DeleteBook usuwa książkę (DELETE /staff/catalog/{id})
func (h *CatalogHandler) DeleteBook(w http.ResponseWriter, r *http.Request) {
	bookID := chi.URLParam(r, "id")
	if bookID == "" {
		http.Error(w, "Brak ID książki", http.StatusBadRequest)
		return
	}

	// Sprawdź czy są aktywne wypożyczenia
	hasLoans, err := firebase.GlobalClient.HasActiveLoans(bookID)
	if err != nil {
		log.Printf("Błąd sprawdzania wypożyczeń: %v", err)
		http.Error(w, "Błąd sprawdzania wypożyczeń", http.StatusInternalServerError)
		return
	}

	if hasLoans {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(`{"error": "Nie można usunąć książki z aktywnymi wypożyczeniami"}`))
		return
	}

	// Usuń książkę
	if err := firebase.GlobalClient.DeleteBook(bookID); err != nil {
		log.Printf("Błąd usuwania książki: %v", err)
		http.Error(w, "Błąd usuwania książki", http.StatusInternalServerError)
		return
	}

	// Zwróć sukces dla htmx
	w.WriteHeader(http.StatusOK)
}

// Funkcje pomocnicze

func (h *CatalogHandler) renderBooksTable(w http.ResponseWriter, data map[string]interface{}) {
	// Prosty szablon tabeli dla htmx
	tmpl := `
	{{range .Books}}
	<tr>
		<td class="px-6 py-4 whitespace-nowrap">{{.Title}}</td>
		<td class="px-6 py-4 whitespace-nowrap">{{.Author}}</td>
		<td class="px-6 py-4 whitespace-nowrap">{{.ISBN}}</td>
		<td class="px-6 py-4 whitespace-nowrap">{{.Category}}</td>
		<td class="px-6 py-4 whitespace-nowrap">{{.AvailableCopies}}/{{.TotalCopies}}</td>
		<td class="px-6 py-4 whitespace-nowrap text-sm">
			<a href="/staff/catalog/{{.ID}}/edit" class="text-blue-600 hover:text-blue-900 mr-3">Edytuj</a>
			<button hx-delete="/staff/catalog/{{.ID}}" 
					hx-confirm="Czy na pewno chcesz usunąć tę książkę?"
					hx-target="closest tr"
					hx-swap="outerHTML swap:1s"
					class="text-red-600 hover:text-red-900">Usuń</button>
		</td>
	</tr>
	{{else}}
	<tr>
		<td colspan="6" class="px-6 py-4 text-center text-gray-500">Brak wyników</td>
	</tr>
	{{end}}
	`

	t, err := template.New("table").Parse(tmpl)
	if err != nil {
		log.Printf("Błąd parsowania szablonu: %v", err)
		http.Error(w, "Błąd renderowania", http.StatusInternalServerError)
		return
	}

	if err := t.Execute(w, data); err != nil {
		log.Printf("Błąd renderowania tabeli: %v", err)
		http.Error(w, "Błąd renderowania", http.StatusInternalServerError)
	}
}

func (h *CatalogHandler) renderFormError(w http.ResponseWriter, r *http.Request, errorMsg string, book *models.Book) {
	if h.formTemplate == nil {
		http.Error(w, errorMsg, http.StatusBadRequest)
		return
	}

	session := middleware.GetSessionFromContext(r.Context())
	data := NewTemplateData(session)
	data["Error"] = errorMsg
	data["Book"] = book
	data["Categories"] = getBookCategories()

	w.WriteHeader(http.StatusBadRequest)
	if err := h.formTemplate.Execute(w, data); err != nil {
		log.Printf("Błąd renderowania formularza z błędem: %v", err)
	}
}

func getBookCategories() []string {
	return []string{
		"Beletrystyka",
		"Fantastyka",
		"Kryminał",
		"Romans",
		"Popularnonaukowa",
		"Naukowa",
		"Informatyka",
		"Historia",
		"Biografia",
		"Poradniki",
		"Literatura piękna",
		"Dla dzieci",
		"Komiks",
		"Inne",
	}
}
