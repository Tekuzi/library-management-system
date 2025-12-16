package firebase

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"

	"library-management-system/internal/models"
)

const (
	// BooksCollection to nazwa kolekcji książek w Firestore
	BooksCollection = "books"
)

// GetBook pobiera książkę po ID
func (c *Client) GetBook(id string) (*models.Book, error) {
	if id == "" {
		return nil, fmt.Errorf("ID książki nie może być puste")
	}

	doc, err := c.Firestore.Collection(BooksCollection).Doc(id).Get(c.ctx)
	if err != nil {
		return nil, fmt.Errorf("błąd pobierania książki: %w", err)
	}

	var book models.Book
	if err := doc.DataTo(&book); err != nil {
		return nil, fmt.Errorf("błąd parsowania danych książki: %w", err)
	}

	// Ustaw ID z dokumentu Firestore
	book.ID = doc.Ref.ID

	return &book, nil
}

// CreateBook tworzy nową książkę w bazie
func (c *Client) CreateBook(book *models.Book) error {
	if book == nil {
		return fmt.Errorf("książka nie może być nil")
	}

	// Walidacja podstawowych pól
	if book.Title == "" {
		return fmt.Errorf("tytuł książki jest wymagany")
	}
	if book.Author == "" {
		return fmt.Errorf("autor książki jest wymagany")
	}

	// Ustawienie timestamps
	now := time.Now()
	book.CreatedAt = now
	book.UpdatedAt = now

	// Jeśli nie ma ID, Firestore wygeneruje automatycznie
	var docRef *firestore.DocumentRef
	if book.ID == "" {
		docRef = c.Firestore.Collection(BooksCollection).NewDoc()
		book.ID = docRef.ID
	} else {
		docRef = c.Firestore.Collection(BooksCollection).Doc(book.ID)
	}

	// Zapisz książkę
	_, err := docRef.Set(c.ctx, book)
	if err != nil {
		return fmt.Errorf("błąd zapisywania książki: %w", err)
	}

	return nil
}

// UpdateBook aktualizuje istniejącą książkę
func (c *Client) UpdateBook(id string, book *models.Book) error {
	if id == "" {
		return fmt.Errorf("ID książki nie może być puste")
	}
	if book == nil {
		return fmt.Errorf("książka nie może być nil")
	}

	// Sprawdź czy książka istnieje
	_, err := c.GetBook(id)
	if err != nil {
		return fmt.Errorf("książka nie istnieje: %w", err)
	}

	// Aktualizuj timestamp
	book.UpdatedAt = time.Now()
	book.ID = id

	// Zapisz zmiany
	_, err = c.Firestore.Collection(BooksCollection).Doc(id).Set(c.ctx, book)
	if err != nil {
		return fmt.Errorf("błąd aktualizacji książki: %w", err)
	}

	return nil
}

// DeleteBook usuwa książkę z bazy
func (c *Client) DeleteBook(id string) error {
	if id == "" {
		return fmt.Errorf("ID książki nie może być puste")
	}

	// Sprawdź czy książka istnieje
	_, err := c.GetBook(id)
	if err != nil {
		return fmt.Errorf("książka nie istnieje: %w", err)
	}

	// Usuń książkę
	_, err = c.Firestore.Collection(BooksCollection).Doc(id).Delete(c.ctx)
	if err != nil {
		return fmt.Errorf("błąd usuwania książki: %w", err)
	}

	return nil
}

// ListBooks pobiera listę wszystkich książek
func (c *Client) ListBooks() ([]*models.Book, error) {
	return c.ListBooksWithFilter(nil)
}

// ListBooksWithFilter pobiera listę książek z opcjonalnym filtrowaniem
func (c *Client) ListBooksWithFilter(queryFn func(firestore.Query) firestore.Query) ([]*models.Book, error) {
	var books []*models.Book

	query := c.Firestore.Collection(BooksCollection).Query

	// Zastosuj filtr jeśli podano
	if queryFn != nil {
		query = queryFn(query)
	}

	// Sortuj po tytule
	query = query.OrderBy("title", firestore.Asc)

	iter := query.Documents(c.ctx)
	defer iter.Stop()

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("błąd iteracji po książkach: %w", err)
		}

		var book models.Book
		if err := doc.DataTo(&book); err != nil {
			return nil, fmt.Errorf("błąd parsowania książki: %w", err)
		}

		// Ustaw ID z dokumentu Firestore
		book.ID = doc.Ref.ID

		books = append(books, &book)
	}

	return books, nil
}

// SearchBooks wyszukuje książki po tytule, autorze lub ISBN
func (c *Client) SearchBooks(searchTerm string) ([]*models.Book, error) {
	if searchTerm == "" {
		return c.ListBooks()
	}

	// Pobierz wszystkie książki i filtruj po stronie aplikacji (Firestore ma ograniczone możliwości wyszukiwania)
	allBooks, err := c.ListBooks()
	if err != nil {
		return nil, err
	}

	var results []*models.Book
	searchLower := strings.ToLower(searchTerm)

	for _, book := range allBooks {
		titleLower := strings.ToLower(book.Title)
		authorLower := strings.ToLower(book.Author)
		isbnLower := strings.ToLower(book.ISBN)

		if strings.Contains(titleLower, searchLower) ||
			strings.Contains(authorLower, searchLower) ||
			strings.Contains(isbnLower, searchLower) {
			results = append(results, book)
		}
	}

	return results, nil
}

// SearchBooksAdvanced wyszukuje książki po wielu kryteriach
func (c *Client) SearchBooksAdvanced(title, author, isbn string) ([]*models.Book, error) {
	// Pobierz wszystkie książki i filtruj po stronie aplikacji
	allBooks, err := c.ListBooks()
	if err != nil {
		return nil, err
	}

	var results []*models.Book
	titleLower := strings.ToLower(title)
	authorLower := strings.ToLower(author)
	isbnLower := strings.ToLower(isbn)

	for _, book := range allBooks {
		match := true

		// Sprawdź każde kryterium (AND logic)
		if title != "" && !strings.Contains(strings.ToLower(book.Title), titleLower) {
			match = false
		}
		if author != "" && !strings.Contains(strings.ToLower(book.Author), authorLower) {
			match = false
		}
		if isbn != "" && !strings.Contains(strings.ToLower(book.ISBN), isbnLower) {
			match = false
		}

		if match {
			results = append(results, book)
		}
	}

	return results, nil
}

// Funkcje pomocnicze
func toLower(s string) string {
	return strings.ToLower(s)
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// GetBookByISBN pobiera książkę po ISBN
func (c *Client) GetBookByISBN(isbn string) (*models.Book, error) {
	if isbn == "" {
		return nil, fmt.Errorf("ISBN nie może być pusty")
	}

	iter := c.Firestore.Collection(BooksCollection).Where("isbn", "==", isbn).Limit(1).Documents(c.ctx)
	defer iter.Stop()

	doc, err := iter.Next()
	if err == iterator.Done {
		return nil, nil // Nie znaleziono książki
	}
	if err != nil {
		return nil, fmt.Errorf("błąd wyszukiwania książki: %w", err)
	}

	var book models.Book
	if err := doc.DataTo(&book); err != nil {
		return nil, fmt.Errorf("błąd parsowania książki: %w", err)
	}

	book.ID = doc.Ref.ID
	return &book, nil
}

// HasActiveLoans sprawdza czy książka ma aktywne wypożyczenia
func (c *Client) HasActiveLoans(bookID string) (bool, error) {
	if bookID == "" {
		return false, fmt.Errorf("ID książki nie może być puste")
	}

	// Sprawdź czy są aktywne wypożyczenia
	iter := c.Firestore.Collection(LoansCollection).
		Where("book_id", "==", bookID).
		Where("status", "==", "active").
		Limit(1).
		Documents(c.ctx)
	defer iter.Stop()

	_, err := iter.Next()
	if err == iterator.Done {
		return false, nil // Brak aktywnych wypożyczeń
	}
	if err != nil {
		return false, fmt.Errorf("błąd sprawdzania wypożyczeń: %w", err)
	}

	return true, nil
}

// ListBooksWithPagination pobiera książki z paginacją i sortowaniem
func (c *Client) ListBooksWithPagination(limit int, offset int, sortBy string, sortOrder string) ([]*models.Book, int, error) {
	var books []*models.Book

	query := c.Firestore.Collection(BooksCollection).Query

	// Sortowanie
	direction := firestore.Asc
	if sortOrder == "desc" {
		direction = firestore.Desc
	}

	// Domyślnie sortuj po tytule
	if sortBy == "" {
		sortBy = "title"
	}

	query = query.OrderBy(sortBy, direction)

	// Pobierz całkowitą liczbę dokumentów dla paginacji
	allDocs, err := c.Firestore.Collection(BooksCollection).Documents(c.ctx).GetAll()
	if err != nil {
		return nil, 0, fmt.Errorf("błąd pobierania liczby książek: %w", err)
	}
	totalCount := len(allDocs)

	// Zastosuj limit i offset
	query = query.Limit(limit).Offset(offset)

	iter := query.Documents(c.ctx)
	defer iter.Stop()

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, 0, fmt.Errorf("błąd iteracji po książkach: %w", err)
		}

		var book models.Book
		if err := doc.DataTo(&book); err != nil {
			return nil, 0, fmt.Errorf("błąd parsowania książki: %w", err)
		}

		book.ID = doc.Ref.ID
		books = append(books, &book)
	}

	return books, totalCount, nil
}

// GetAvailableBooks pobiera tylko dostępne książki
func (c *Client) GetAvailableBooks() ([]*models.Book, error) {
	return c.ListBooksWithFilter(func(q firestore.Query) firestore.Query {
		return q.Where("available_copies", ">", 0)
	})
}

// GetBooksByCategory pobiera książki z danej kategorii
func (c *Client) GetBooksByCategory(category string) ([]*models.Book, error) {
	if category == "" {
		return c.ListBooks()
	}

	return c.ListBooksWithFilter(func(q firestore.Query) firestore.Query {
		return q.Where("category", "==", category)
	})
}

// UpdateBookAvailability aktualizuje dostępność książki
func (c *Client) UpdateBookAvailability(bookID string, increment bool) error {
	docRef := c.Firestore.Collection(BooksCollection).Doc(bookID)

	return c.Firestore.RunTransaction(c.ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		doc, err := tx.Get(docRef)
		if err != nil {
			return err
		}

		var book models.Book
		if err := doc.DataTo(&book); err != nil {
			return err
		}

		if increment {
			book.IncrementAvailableCopies()
		} else {
			if !book.IsAvailable() {
				return fmt.Errorf("książka nie jest dostępna")
			}
			book.DecrementAvailableCopies()
		}

		book.UpdatedAt = time.Now()

		return tx.Set(docRef, &book)
	})
}

// CountTotalBooks zwraca całkowitą liczbę książek w systemie
func (c *Client) CountTotalBooks() (int, error) {
	docs, err := c.Firestore.Collection(BooksCollection).Documents(c.ctx).GetAll()
	if err != nil {
		return 0, fmt.Errorf("błąd liczenia książek: %w", err)
	}
	return len(docs), nil
}
