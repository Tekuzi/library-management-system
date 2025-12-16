package handlers

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"library-management-system/internal/firebase"
	"library-management-system/internal/middleware"
	"library-management-system/internal/models"
)

// BooksHandler obsługuje operacje na książkach
type BooksHandler struct {
	catalogTemplate *template.Template
	detailTemplate  *template.Template
	fbClient        *firebase.Client
}

// NewBooksHandler tworzy nowy handler dla książek
func NewBooksHandler(fbClient *firebase.Client) *BooksHandler {
	funcMap := template.FuncMap{
		"sub": func(a, b int) int {
			return a - b
		},
	}

	catalogTmpl, err := template.ParseFiles("internal/templates/catalog.html")
	if err != nil {
		log.Printf("Błąd ładowania szablonu catalog.html: %v", err)
	}

	detailTmpl, err := template.New("detail.html").Funcs(funcMap).ParseFiles("internal/templates/books/detail.html")
	if err != nil {
		log.Printf("Błąd ładowania szablonu detail.html: %v", err)
	}

	return &BooksHandler{
		catalogTemplate: catalogTmpl,
		detailTemplate:  detailTmpl,
		fbClient:        fbClient,
	}
}

// ListBooksHandler zwraca listę książek (GET /books)
func (h *BooksHandler) ListBooksHandler(w http.ResponseWriter, r *http.Request) {
	// Sprawdź czy Firebase jest zainicjalizowany
	if firebase.GlobalClient == nil {
		session := middleware.GetSessionFromContext(r.Context())
		data := NewTemplateData(session)
		data["Error"] = "Firebase nie został zainicjalizowany. Sprawdź konfigurację."
		data["Books"] = nil
		if h.catalogTemplate != nil {
			h.catalogTemplate.Execute(w, data)
		} else {
			http.Error(w, "Firebase nie został zainicjalizowany", http.StatusInternalServerError)
		}
		return
	}

	// Pobierz parametry wyszukiwania
	search := r.URL.Query().Get("search")
	title := r.URL.Query().Get("title")
	author := r.URL.Query().Get("author")
	isbn := r.URL.Query().Get("isbn")
	category := r.URL.Query().Get("category")
	availableOnly := r.URL.Query().Get("available") == "true"

	var books []*models.Book
	var err error

	// Wykonaj odpowiednie zapytanie
	// Proste wyszukiwanie po wszystkim
	if search != "" {
		books, err = firebase.GlobalClient.SearchBooks(search)
	} else if title != "" || author != "" || isbn != "" {
		// Zaawansowane wyszukiwanie
		books, err = firebase.GlobalClient.SearchBooksAdvanced(title, author, isbn)
	} else if category != "" {
		books, err = firebase.GlobalClient.GetBooksByCategory(category)
	} else if availableOnly {
		books, err = firebase.GlobalClient.GetAvailableBooks()
	} else {
		books, err = firebase.GlobalClient.ListBooks()
	}

	if err != nil {
		log.Printf("Błąd pobierania książek: %v", err)
		session := middleware.GetSessionFromContext(r.Context())
		data := NewTemplateData(session)
		data["Error"] = "Błąd pobierania książek z bazy danych: " + err.Error()
		data["Books"] = nil
		if h.catalogTemplate != nil {
			h.catalogTemplate.Execute(w, data)
		} else {
			http.Error(w, "Błąd pobierania książek", http.StatusInternalServerError)
		}
		return
	}

	// Renderuj stronę z katalogiem
	h.renderCatalogPage(w, r, books)
}

// ShowBookHandler wyświetla szczegóły książki (GET /books/{id})
func (h *BooksHandler) ShowBookHandler(w http.ResponseWriter, r *http.Request) {
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

	// TODO: Renderuj szablon szczegółów książki
	h.renderBookDetails(w, r, book)
}

// CreateBookHandler tworzy nową książkę (POST /books)
func (h *BooksHandler) CreateBookHandler(w http.ResponseWriter, r *http.Request) {
	// Sprawdź czy użytkownik ma uprawnienia (middleware powinien to zapewnić)
	user, err := middleware.GetUserFromContext(r.Context())
	if err != nil {
		http.Error(w, "Brak autoryzacji", http.StatusUnauthorized)
		return
	}

	// Tylko admin może dodawać książki
	if !user.IsAdmin() {
		http.Error(w, "Brak uprawnień", http.StatusForbidden)
		return
	}

	// Parsuj dane z formularza lub JSON
	var book models.Book

	contentType := r.Header.Get("Content-Type")
	if contentType == "application/json" {
		if err := json.NewDecoder(r.Body).Decode(&book); err != nil {
			http.Error(w, "Nieprawidłowe dane JSON", http.StatusBadRequest)
			return
		}
	} else {
		// Parsuj dane z formularza
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Błąd parsowania formularza", http.StatusBadRequest)
			return
		}

		book = models.Book{
			ISBN:          r.FormValue("isbn"),
			Title:         r.FormValue("title"),
			Author:        r.FormValue("author"),
			Publisher:     r.FormValue("publisher"),
			Category:      r.FormValue("category"),
			Description:   r.FormValue("description"),
			ShelfLocation: r.FormValue("shelf_location"),
			CoverImageURL: r.FormValue("cover_image_url"),
		}

		// Konwertuj wartości numeryczne
		if pubYear := r.FormValue("publication_year"); pubYear != "" {
			if year, err := strconv.Atoi(pubYear); err == nil {
				book.PublicationYear = year
			}
		}

		if total := r.FormValue("total_copies"); total != "" {
			if copies, err := strconv.Atoi(total); err == nil {
				book.TotalCopies = copies
				book.AvailableCopies = copies // Początkowo wszystkie egzemplarze są dostępne
			}
		}
	}

	// Walidacja podstawowych danych
	if book.Title == "" || book.Author == "" {
		http.Error(w, "Tytuł i autor są wymagane", http.StatusBadRequest)
		return
	}

	// Zapisz książkę
	if err := firebase.GlobalClient.CreateBook(&book); err != nil {
		log.Printf("Błąd tworzenia książki: %v", err)
		http.Error(w, "Błąd tworzenia książki", http.StatusInternalServerError)
		return
	}

	// Zwróć odpowiedź
	if r.Header.Get("HX-Request") == "true" {
		// Dla htmx - zwróć fragment HTML z nową książką
		w.Header().Set("HX-Trigger", "bookCreated")
		h.renderBookCard(w, &book)
	} else if contentType == "application/json" {
		// Dla JSON API - zwróć dane książki
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(book)
	} else {
		// Przekieruj na stronę książki
		http.Redirect(w, r, "/books/"+book.ID, http.StatusSeeOther)
	}
}

// UpdateBookHandler aktualizuje książkę (PUT /books/{id})
func (h *BooksHandler) UpdateBookHandler(w http.ResponseWriter, r *http.Request) {
	bookID := chi.URLParam(r, "id")
	if bookID == "" {
		http.Error(w, "Brak ID książki", http.StatusBadRequest)
		return
	}

	// Sprawdź uprawnienia
	user, err := middleware.GetUserFromContext(r.Context())
	if err != nil {
		http.Error(w, "Brak autoryzacji", http.StatusUnauthorized)
		return
	}

	if !user.IsAdmin() {
		http.Error(w, "Brak uprawnień", http.StatusForbidden)
		return
	}

	// Pobierz istniejącą książkę
	existingBook, err := firebase.GlobalClient.GetBook(bookID)
	if err != nil {
		http.Error(w, "Książka nie została znaleziona", http.StatusNotFound)
		return
	}

	// Parsuj nowe dane
	var book models.Book

	contentType := r.Header.Get("Content-Type")
	if contentType == "application/json" {
		if err := json.NewDecoder(r.Body).Decode(&book); err != nil {
			http.Error(w, "Nieprawidłowe dane JSON", http.StatusBadRequest)
			return
		}
	} else {
		// Parsuj dane z formularza
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Błąd parsowania formularza", http.StatusBadRequest)
			return
		}

		book = *existingBook // Zachowaj istniejące dane

		// Aktualizuj tylko podane pola
		if isbn := r.FormValue("isbn"); isbn != "" {
			book.ISBN = isbn
		}
		if title := r.FormValue("title"); title != "" {
			book.Title = title
		}
		if author := r.FormValue("author"); author != "" {
			book.Author = author
		}
		if publisher := r.FormValue("publisher"); publisher != "" {
			book.Publisher = publisher
		}
		if category := r.FormValue("category"); category != "" {
			book.Category = category
		}
		if description := r.FormValue("description"); description != "" {
			book.Description = description
		}
		if shelfLocation := r.FormValue("shelf_location"); shelfLocation != "" {
			book.ShelfLocation = shelfLocation
		}
		if coverImage := r.FormValue("cover_image_url"); coverImage != "" {
			book.CoverImageURL = coverImage
		}

		if pubYear := r.FormValue("publication_year"); pubYear != "" {
			if year, err := strconv.Atoi(pubYear); err == nil {
				book.PublicationYear = year
			}
		}

		if total := r.FormValue("total_copies"); total != "" {
			if copies, err := strconv.Atoi(total); err == nil {
				book.TotalCopies = copies
			}
		}
	}

	// Zachowaj ważne pola
	book.CreatedAt = existingBook.CreatedAt

	// Aktualizuj książkę
	if err := firebase.GlobalClient.UpdateBook(bookID, &book); err != nil {
		log.Printf("Błąd aktualizacji książki: %v", err)
		http.Error(w, "Błąd aktualizacji książki", http.StatusInternalServerError)
		return
	}

	// Zwróć odpowiedź
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Trigger", "bookUpdated")
		h.renderBookCard(w, &book)
	} else if contentType == "application/json" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(book)
	} else {
		http.Redirect(w, r, "/books/"+book.ID, http.StatusSeeOther)
	}
}

// DeleteBookHandler usuwa książkę (DELETE /books/{id})
func (h *BooksHandler) DeleteBookHandler(w http.ResponseWriter, r *http.Request) {
	bookID := chi.URLParam(r, "id")
	if bookID == "" {
		http.Error(w, "Brak ID książki", http.StatusBadRequest)
		return
	}

	// Sprawdź uprawnienia - tylko admin może usuwać książki
	user, err := middleware.GetUserFromContext(r.Context())
	if err != nil {
		http.Error(w, "Brak autoryzacji", http.StatusUnauthorized)
		return
	}

	if !user.IsAdmin() {
		http.Error(w, "Brak uprawnień - tylko administrator może usuwać książki", http.StatusForbidden)
		return
	}

	// Usuń książkę
	if err := firebase.GlobalClient.DeleteBook(bookID); err != nil {
		log.Printf("Błąd usuwania książki: %v", err)
		http.Error(w, "Błąd usuwania książki", http.StatusInternalServerError)
		return
	}

	// Zwróć odpowiedź
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Trigger", "bookDeleted")
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusNoContent)
	}
}

// Funkcje pomocnicze do renderowania

func (h *BooksHandler) renderBooksFragment(w http.ResponseWriter, books []*models.Book) {
	// Renderuj tylko fragment HTML z listą książek dla htmx
	if h.catalogTemplate == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(books)
		return
	}

	data := map[string]interface{}{
		"Books": books,
	}

	if err := h.catalogTemplate.Execute(w, data); err != nil {
		log.Printf("Błąd renderowania fragmentu książek: %v", err)
		http.Error(w, "Błąd renderowania", http.StatusInternalServerError)
	}
}

func (h *BooksHandler) renderCatalogPage(w http.ResponseWriter, r *http.Request, books []*models.Book) {
	if h.catalogTemplate == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(books)
		return
	}

	session := middleware.GetSessionFromContext(r.Context())
	data := NewTemplateData(session)
	data["Books"] = books
	data["Error"] = nil
	data["SearchQuery"] = r.URL.Query().Get("search")

	// Parametry zaawansowanego wyszukiwania
	searchParams := map[string]string{
		"Title":    r.URL.Query().Get("title"),
		"Author":   r.URL.Query().Get("author"),
		"ISBN":     r.URL.Query().Get("isbn"),
		"Category": r.URL.Query().Get("category"),
	}
	data["Search"] = searchParams

	if err := h.catalogTemplate.Execute(w, data); err != nil {
		log.Printf("Błąd renderowania katalogu: %v", err)
		http.Error(w, "Błąd renderowania", http.StatusInternalServerError)
	}
}

func (h *BooksHandler) renderBookDetails(w http.ResponseWriter, r *http.Request, book *models.Book) {
	if h.detailTemplate == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(book)
		return
	}

	session := middleware.GetSessionFromContext(r.Context())
	data := NewTemplateData(session)
	data["Book"] = book

	// Sprawdź czy użytkownik może wypożyczyć
	if session != nil && h.fbClient != nil {
		user, err := h.fbClient.GetUser(session.UserID)
		if err == nil {
			data["CanBorrow"] = user.CanBorrow()
			if !user.CanBorrow() {
				if user.CurrentLoans >= user.MaxLoans {
					data["BorrowError"] = "Osiągnięto maksymalny limit wypożyczeń"
				} else if !user.IsActive {
					data["BorrowError"] = "Konto nieaktywne - skontaktuj się z biblioteką"
				}
			}
		}
	}

	if err := h.detailTemplate.Execute(w, data); err != nil {
		log.Printf("Błąd renderowania szczegółów książki: %v", err)
		http.Error(w, "Błąd renderowania strony", http.StatusInternalServerError)
	}
}

func (h *BooksHandler) renderBookCard(w http.ResponseWriter, book *models.Book) {
	// TODO: Renderuj kartę książki dla htmx
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(book)
}

// SearchBooksHandler obsługuje wyszukiwanie książek (GET /books/search)
func (h *BooksHandler) SearchBooksHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")

	if firebase.GlobalClient == nil {
		http.Error(w, "Firebase nie został zainicjalizowany", http.StatusInternalServerError)
		return
	}

	var books []*models.Book
	var err error

	if query != "" {
		books, err = firebase.GlobalClient.SearchBooks(query)
	} else {
		books, err = firebase.GlobalClient.ListBooks()
	}

	if err != nil {
		log.Printf("Błąd wyszukiwania książek: %v", err)
		http.Error(w, "Błąd wyszukiwania", http.StatusInternalServerError)
		return
	}

	h.renderBooksFragment(w, books)
}

// BorrowBook obsługuje wypożyczenie książki (POST /books/{id}/borrow)
func (h *BooksHandler) BorrowBook(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSessionFromContext(r.Context())
	if session == nil {
		http.Error(w, "Musisz być zalogowany", http.StatusUnauthorized)
		return
	}

	bookID := chi.URLParam(r, "id")
	if bookID == "" {
		http.Error(w, "Brak ID książki", http.StatusBadRequest)
		return
	}

	if h.fbClient == nil {
		http.Error(w, "Baza danych niedostępna", http.StatusInternalServerError)
		return
	}

	// Pobierz użytkownika
	user, err := h.fbClient.GetUser(session.UserID)
	if err != nil {
		log.Printf("Błąd pobierania użytkownika: %v", err)
		http.Error(w, "Błąd pobierania danych użytkownika", http.StatusInternalServerError)
		return
	}

	// Sprawdź czy użytkownik może wypożyczyć
	if !user.CanBorrow() {
		errMsg := "Nie możesz wypożyczyć książki"
		if user.CurrentLoans >= user.MaxLoans {
			errMsg = "Osiągnięto maksymalny limit wypożyczeń"
		} else if !user.IsActive {
			errMsg = "Konto nieaktywne - skontaktuj się z biblioteką"
		}
		w.Write([]byte(`<div class="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded text-sm">` + errMsg + `</div>`))
		return
	}

	// Pobierz książkę
	book, err := h.fbClient.GetBook(bookID)
	if err != nil {
		log.Printf("Błąd pobierania książki: %v", err)
		http.Error(w, "Książka nie została znaleziona", http.StatusNotFound)
		return
	}

	// Sprawdź dostępność
	if !book.IsAvailable() {
		w.Write([]byte(`<div class="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded text-sm">Książka jest obecnie niedostępna</div>`))
		return
	}

	// Utwórz wypożyczenie (CreateLoan automatycznie wygeneruje kod odbioru i ustawi status pending_pickup)
	loan := &models.Loan{
		BookID:    bookID,
		UserID:    session.UserID,
		BookTitle: book.Title,                           // Denormalizacja
		UserName:  user.FirstName + " " + user.LastName, // Denormalizacja
	}

	if err := h.fbClient.CreateLoan(loan); err != nil {
		log.Printf("Błąd tworzenia wypożyczenia: %v", err)
		http.Error(w, "Błąd wypożyczania książki", http.StatusInternalServerError)
		return
	}

	// Zmniejsz dostępne egzemplarze
	if err := h.fbClient.UpdateBookAvailability(bookID, false); err != nil {
		log.Printf("Błąd aktualizacji dostępności: %v", err)
		// Wypożyczenie zostało utworzone, ale nie udało się zaktualizować dostępności
	}

	// Zwiększ licznik wypożyczeń użytkownika
	if err := h.fbClient.UpdateUserLoansCount(session.UserID, true); err != nil {
		log.Printf("Błąd aktualizacji licznika wypożyczeń: %v", err)
	}

	// Zwróć komunikat sukcesu z kodem odbioru
	w.Write([]byte(`
		<div class="bg-green-100 border border-green-400 text-green-700 px-4 py-3 rounded text-sm">
			<p class="font-bold">Zamówienie utworzone!</p>
			<p class="text-2xl font-mono font-bold my-2">Kod odbioru: ` + loan.PickupCode + `</p>
			<p>Podaj ten kod w bibliotece, aby odebrać książkę.</p>
			<a href="/user" class="text-green-800 underline mt-2 inline-block">Zobacz moje wypożyczenia</a>
		</div>
	`))
}

// ReserveBook obsługuje rezerwację książki (POST /books/{id}/reserve)
func (h *BooksHandler) ReserveBook(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSessionFromContext(r.Context())
	if session == nil {
		http.Error(w, "Musisz być zalogowany", http.StatusUnauthorized)
		return
	}

	bookID := chi.URLParam(r, "id")
	if bookID == "" {
		http.Error(w, "Brak ID książki", http.StatusBadRequest)
		return
	}

	if h.fbClient == nil {
		http.Error(w, "Baza danych niedostępna", http.StatusInternalServerError)
		return
	}

	// Pobierz użytkownika
	user, err := h.fbClient.GetUser(session.UserID)
	if err != nil {
		log.Printf("Błąd pobierania użytkownika: %v", err)
		http.Error(w, "Błąd pobierania danych użytkownika", http.StatusInternalServerError)
		return
	}

	// Sprawdź czy użytkownik jest aktywny
	if !user.IsActive {
		w.Write([]byte(`<div class="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded text-sm">Konto nieaktywne - skontaktuj się z biblioteką</div>`))
		return
	}

	// Sprawdź czy użytkownik nie ma już rezerwacji tej książki
	existingReservations, err := h.fbClient.GetUserReservations(session.UserID)
	if err == nil {
		for _, res := range existingReservations {
			if res.BookID == bookID && (res.Status == models.ReservationStatusPending || res.Status == models.ReservationStatusReady) {
				w.Write([]byte(`<div class="bg-yellow-100 border border-yellow-400 text-yellow-700 px-4 py-3 rounded text-sm">Masz już aktywną rezerwację tej książki</div>`))
				return
			}
		}
	}

	// Utwórz rezerwację
	reservation := &models.Reservation{
		BookID:     bookID,
		UserID:     session.UserID,
		Status:     models.ReservationStatusPending,
		ExpiryDate: time.Now().AddDate(0, 0, 7), // 7 dni na odbiór gdy będzie dostępna
	}

	if err := h.fbClient.CreateReservation(reservation); err != nil {
		log.Printf("Błąd tworzenia rezerwacji: %v", err)
		http.Error(w, "Błąd rezerwacji książki", http.StatusInternalServerError)
		return
	}

	// Zwróć komunikat sukcesu
	w.Write([]byte(`
		<div class="bg-green-100 border border-green-400 text-green-700 px-4 py-3 rounded text-sm">
			<p class="font-bold">Książka zarezerwowana!</p>
			<p>Powiadomimy Cię, gdy będzie dostępna</p>
			<a href="/user/reservations" class="text-green-800 underline mt-2 inline-block">Zobacz moje rezerwacje</a>
		</div>
	`))
}
