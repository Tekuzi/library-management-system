package firebase

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"

	"library-management-system/internal/models"
)

const (
	// LoansCollection to nazwa kolekcji wypożyczeń w Firestore
	LoansCollection = "loans"
)

// GeneratePickupCode generuje losowy 6-znakowy kod alfanumeryczny
func GeneratePickupCode() string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	const codeLength = 6

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	code := make([]byte, codeLength)
	for i := range code {
		code[i] = charset[r.Intn(len(charset))]
	}
	return string(code)
}

// GetLoan pobiera wypożyczenie po ID
func (c *Client) GetLoan(id string) (*models.Loan, error) {
	if id == "" {
		return nil, fmt.Errorf("ID wypożyczenia nie może być puste")
	}

	doc, err := c.Firestore.Collection(LoansCollection).Doc(id).Get(c.ctx)
	if err != nil {
		return nil, fmt.Errorf("błąd pobierania wypożyczenia: %w", err)
	}

	var loan models.Loan
	if err := doc.DataTo(&loan); err != nil {
		return nil, fmt.Errorf("błąd parsowania danych wypożyczenia: %w", err)
	}

	return &loan, nil
}

// CreateLoan tworzy nowe wypożyczenie
func (c *Client) CreateLoan(loan *models.Loan) error {
	if loan == nil {
		return fmt.Errorf("wypożyczenie nie może być nil")
	}

	// Walidacja
	if loan.BookID == "" || loan.UserID == "" {
		return fmt.Errorf("ID książki i użytkownika są wymagane")
	}

	// Domyślne wartości
	now := time.Now()
	loan.CreatedAt = now
	loan.UpdatedAt = now
	loan.LoanDate = now
	loan.Status = models.LoanStatusPendingPickup
	loan.PickupCode = GeneratePickupCode()

	// DueDate zostanie ustawiony gdy admin potwierdzi odbiór
	loan.DueDate = time.Time{}

	// Wygeneruj ID
	var docRef *firestore.DocumentRef
	if loan.ID == "" {
		docRef = c.Firestore.Collection(LoansCollection).NewDoc()
		loan.ID = docRef.ID
	} else {
		docRef = c.Firestore.Collection(LoansCollection).Doc(loan.ID)
	}

	_, err := docRef.Set(c.ctx, loan)
	if err != nil {
		return fmt.Errorf("błąd zapisywania wypożyczenia: %w", err)
	}

	return nil
}

// UpdateLoan aktualizuje wypożyczenie
func (c *Client) UpdateLoan(id string, loan *models.Loan) error {
	if id == "" {
		return fmt.Errorf("ID wypożyczenia nie może być puste")
	}
	if loan == nil {
		return fmt.Errorf("wypożyczenie nie może być nil")
	}

	_, err := c.GetLoan(id)
	if err != nil {
		return fmt.Errorf("wypożyczenie nie istnieje: %w", err)
	}

	loan.UpdatedAt = time.Now()
	loan.ID = id

	_, err = c.Firestore.Collection(LoansCollection).Doc(id).Set(c.ctx, loan)
	if err != nil {
		return fmt.Errorf("błąd aktualizacji wypożyczenia: %w", err)
	}

	return nil
}

// ConfirmPickup potwierdza odbiór książki przez użytkownika
func (c *Client) ConfirmPickup(pickupCode string) error {
	if pickupCode == "" {
		return fmt.Errorf("kod odbioru nie może być pusty")
	}

	// Znajdź wypożyczenie po kodzie odbioru
	iter := c.Firestore.Collection(LoansCollection).
		Where("pickup_code", "==", pickupCode).
		Where("status", "==", string(models.LoanStatusPendingPickup)).
		Limit(1).
		Documents(c.ctx)

	doc, err := iter.Next()
	if err == iterator.Done {
		return fmt.Errorf("nie znaleziono wypożyczenia z kodem %s", pickupCode)
	}
	if err != nil {
		return fmt.Errorf("błąd wyszukiwania wypożyczenia: %w", err)
	}

	var loan models.Loan
	if err := doc.DataTo(&loan); err != nil {
		return fmt.Errorf("błąd parsowania danych wypożyczenia: %w", err)
	}

	// Ustaw status na active i ustaw termin zwrotu (14 dni od teraz)
	now := time.Now()
	loan.Status = models.LoanStatusActive
	loan.DueDate = now.AddDate(0, 0, 14)
	loan.UpdatedAt = now

	// Zapisz zmiany
	_, err = c.Firestore.Collection(LoansCollection).Doc(loan.ID).Set(c.ctx, &loan)
	if err != nil {
		return fmt.Errorf("błąd aktualizacji wypożyczenia: %w", err)
	}

	log.Printf("Potwierdzono odbiór dla wypożyczenia %s (kod: %s)", loan.ID, pickupCode)
	return nil
}

// ReturnLoan obsługuje zwrot książki
func (c *Client) ReturnLoan(loanID string) error {
	loan, err := c.GetLoan(loanID)
	if err != nil {
		return err
	}

	if loan.Status != models.LoanStatusActive {
		return fmt.Errorf("wypożyczenie nie jest aktywne")
	}

	now := time.Now()
	loan.ReturnDate = &now
	loan.Status = models.LoanStatusReturned
	loan.UpdatedAt = now

	// Oblicz karę jeśli jest opóźnienie
	if loan.IsOverdue() {
		loan.FineAmount = loan.CalculateFine()
	}

	// Zaktualizuj status wypożyczenia
	if err := c.UpdateLoan(loanID, loan); err != nil {
		return fmt.Errorf("błąd aktualizacji wypożyczenia: %w", err)
	}

	// Zmniejsz licznik wypożyczeń użytkownika
	user, err := c.GetUser(loan.UserID)
	if err != nil {
		return fmt.Errorf("błąd pobierania użytkownika: %w", err)
	}

	if user.CurrentLoans > 0 {
		user.CurrentLoans--
		user.UpdatedAt = now

		if err := c.UpdateUser(loan.UserID, user); err != nil {
			return fmt.Errorf("błąd aktualizacji licznika wypożyczeń użytkownika: %w", err)
		}
	}

	// Sprawdź czy są rezerwacje na tę książkę
	nextReservation, err := c.GetNextReservation(loan.BookID)
	if err != nil {
		return fmt.Errorf("błąd sprawdzania rezerwacji: %w", err)
	}

	if nextReservation != nil {
		// Jest rezerwacja - oznacz jako gotową do odbioru (książka czeka na użytkownika)
		log.Printf("Znaleziono rezerwację %s dla książki %s, zmieniam status na 'ready'", nextReservation.ID, loan.BookID)
		if err := c.MarkReservationReady(nextReservation.ID); err != nil {
			return fmt.Errorf("błąd aktywacji rezerwacji: %w", err)
		}
		log.Printf("Rezerwacja %s aktywowana pomyślnie", nextReservation.ID)
		// NIE zwiększaj AvailableCopies - książka jest zarezerwowana
	} else {
		// Brak rezerwacji - zwróć książkę do katalogu
		log.Printf("Brak rezerwacji dla książki %s, zwracam do katalogu", loan.BookID)
		book, err := c.GetBook(loan.BookID)
		if err != nil {
			return fmt.Errorf("błąd pobierania książki: %w", err)
		}

		book.AvailableCopies++
		book.UpdatedAt = now

		if err := c.UpdateBook(loan.BookID, book); err != nil {
			return fmt.Errorf("błąd aktualizacji dostępności książki: %w", err)
		}
	}

	return nil
}

// ListLoans pobiera wszystkie wypożyczenia
func (c *Client) ListLoans() ([]*models.Loan, error) {
	var loans []*models.Loan

	iter := c.Firestore.Collection(LoansCollection).
		OrderBy("loan_date", firestore.Desc).
		Documents(c.ctx)
	defer iter.Stop()

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("błąd iteracji po wypożyczeniach: %w", err)
		}

		var loan models.Loan
		if err := doc.DataTo(&loan); err != nil {
			return nil, fmt.Errorf("błąd parsowania wypożyczenia: %w", err)
		}

		loans = append(loans, &loan)
	}

	return loans, nil
}

// GetActiveLoans pobiera aktywne wypożyczenia
func (c *Client) GetActiveLoans() ([]*models.Loan, error) {
	var loans []*models.Loan

	iter := c.Firestore.Collection(LoansCollection).
		Where("status", "==", string(models.LoanStatusActive)).
		Documents(c.ctx)
	defer iter.Stop()

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("błąd iteracji po wypożyczeniach: %w", err)
		}

		var loan models.Loan
		if err := doc.DataTo(&loan); err != nil {
			return nil, fmt.Errorf("błąd parsowania wypożyczenia: %w", err)
		}

		loans = append(loans, &loan)
	}

	return loans, nil
}

// GetUserLoans pobiera wypożyczenia użytkownika
func (c *Client) GetUserLoans(userID string) ([]*models.Loan, error) {
	if userID == "" {
		return nil, fmt.Errorf("ID użytkownika nie może być puste")
	}

	var loans []*models.Loan

	iter := c.Firestore.Collection(LoansCollection).
		Where("user_id", "==", userID).
		OrderBy("loan_date", firestore.Desc).
		Documents(c.ctx)
	defer iter.Stop()

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("błąd iteracji po wypożyczeniach: %w", err)
		}

		var loan models.Loan
		if err := doc.DataTo(&loan); err != nil {
			return nil, fmt.Errorf("błąd parsowania wypożyczenia: %w", err)
		}

		loans = append(loans, &loan)
	}

	return loans, nil
}

// GetBookLoans pobiera wypożyczenia książki
func (c *Client) GetBookLoans(bookID string) ([]*models.Loan, error) {
	if bookID == "" {
		return nil, fmt.Errorf("ID książki nie może być puste")
	}

	var loans []*models.Loan

	iter := c.Firestore.Collection(LoansCollection).
		Where("book_id", "==", bookID).
		OrderBy("loan_date", firestore.Desc).
		Documents(c.ctx)
	defer iter.Stop()

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("błąd iteracji po wypożyczeniach: %w", err)
		}

		var loan models.Loan
		if err := doc.DataTo(&loan); err != nil {
			return nil, fmt.Errorf("błąd parsowania wypożyczenia: %w", err)
		}

		loans = append(loans, &loan)
	}

	return loans, nil
}

// GetOverdueLoans pobiera przeterminowane wypożyczenia
func (c *Client) GetOverdueLoans() ([]*models.Loan, error) {
	// Pobierz wszystkie aktywne wypożyczenia i filtruj po stronie aplikacji
	activeLoans, err := c.GetActiveLoans()
	if err != nil {
		return nil, err
	}

	var overdueLoans []*models.Loan
	now := time.Now()
	for _, loan := range activeLoans {
		if loan.DueDate.Before(now) {
			overdueLoans = append(overdueLoans, loan)
		}
	}

	return overdueLoans, nil
}

// CountActiveLoans zwraca liczbę aktywnych wypożyczeń
func (c *Client) CountActiveLoans() (int, error) {
	docs, err := c.Firestore.Collection(LoansCollection).
		Where("status", "==", string(models.LoanStatusActive)).
		Documents(c.ctx).GetAll()
	if err != nil {
		return 0, fmt.Errorf("błąd liczenia aktywnych wypożyczeń: %w", err)
	}
	return len(docs), nil
}

// CountOverdueLoans zwraca liczbę przeterminowanych wypożyczeń
func (c *Client) CountOverdueLoans() (int, error) {
	// Pobierz wszystkie aktywne wypożyczenia i filtruj po stronie aplikacji
	activeLoans, err := c.GetActiveLoans()
	if err != nil {
		return 0, fmt.Errorf("błąd pobierania aktywnych wypożyczeń: %w", err)
	}

	count := 0
	now := time.Now()
	for _, loan := range activeLoans {
		if loan.DueDate.Before(now) {
			count++
		}
	}

	return count, nil
}

// GetUserActiveLoans pobiera aktywne wypożyczenia konkretnego użytkownika (active i pending_pickup)
func (c *Client) GetUserActiveLoans(userID string) ([]*models.Loan, error) {
	if userID == "" {
		return nil, fmt.Errorf("ID użytkownika nie może być puste")
	}

	var loans []*models.Loan

	// Pobierz wszystkie wypożyczenia użytkownika
	iter := c.Firestore.Collection(LoansCollection).
		Where("user_id", "==", userID).
		Documents(c.ctx)
	defer iter.Stop()

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("błąd iteracji po wypożyczeniach: %w", err)
		}

		var loan models.Loan
		if err := doc.DataTo(&loan); err != nil {
			return nil, fmt.Errorf("błąd parsowania wypożyczenia: %w", err)
		}

		// Dodaj tylko wypożyczenia aktywne lub oczekujące na odbiór
		if loan.Status == models.LoanStatusActive || loan.Status == models.LoanStatusPendingPickup {
			loans = append(loans, &loan)
		}
	}

	return loans, nil
}

// GetUserLoanHistory pobiera historię wypożyczeń użytkownika (zwrócone książki)
// GetUserLoanHistory pobiera historię wypożyczeń użytkownika (zwrócone książki)
func (c *Client) GetUserLoanHistory(userID string) ([]*models.Loan, error) {
	if userID == "" {
		return nil, fmt.Errorf("ID użytkownika nie może być puste")
	}

	var loans []*models.Loan

	iter := c.Firestore.Collection(LoansCollection).
		Where("user_id", "==", userID).
		Where("status", "==", string(models.LoanStatusReturned)).
		Documents(c.ctx)
	defer iter.Stop()

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("błąd iteracji po wypożyczeniach: %w", err)
		}

		var loan models.Loan
		if err := doc.DataTo(&loan); err != nil {
			return nil, fmt.Errorf("błąd parsowania wypożyczenia: %w", err)
		}

		loans = append(loans, &loan)
	}

	return loans, nil
}
