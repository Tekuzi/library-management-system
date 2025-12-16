package firebase

import (
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"

	"library-management-system/internal/models"
)

const (
	// ReservationsCollection to nazwa kolekcji rezerwacji w Firestore
	ReservationsCollection = "reservations"
)

// GetReservation pobiera rezerwację po ID
func (c *Client) GetReservation(id string) (*models.Reservation, error) {
	if id == "" {
		return nil, fmt.Errorf("ID rezerwacji nie może być puste")
	}

	doc, err := c.Firestore.Collection(ReservationsCollection).Doc(id).Get(c.ctx)
	if err != nil {
		return nil, fmt.Errorf("błąd pobierania rezerwacji: %w", err)
	}

	var reservation models.Reservation
	if err := doc.DataTo(&reservation); err != nil {
		return nil, fmt.Errorf("błąd parsowania danych rezerwacji: %w", err)
	}

	return &reservation, nil
}

// CreateReservation tworzy nową rezerwację
func (c *Client) CreateReservation(reservation *models.Reservation) error {
	if reservation == nil {
		return fmt.Errorf("rezerwacja nie może być nil")
	}

	// Walidacja
	if reservation.BookID == "" || reservation.UserID == "" {
		return fmt.Errorf("ID książki i użytkownika są wymagane")
	}

	// Domyślne wartości
	now := time.Now()
	reservation.CreatedAt = now
	reservation.UpdatedAt = now
	reservation.ReservationDate = now
	reservation.Status = models.ReservationStatusPending

	// Domyślnie rezerwacja wygasa po 3 dniach od powiadomienia
	if reservation.ExpiryDate.IsZero() {
		reservation.ExpiryDate = now.AddDate(0, 0, 3)
	}

	// Wygeneruj ID
	var docRef *firestore.DocumentRef
	if reservation.ID == "" {
		docRef = c.Firestore.Collection(ReservationsCollection).NewDoc()
		reservation.ID = docRef.ID
	} else {
		docRef = c.Firestore.Collection(ReservationsCollection).Doc(reservation.ID)
	}

	_, err := docRef.Set(c.ctx, reservation)
	if err != nil {
		return fmt.Errorf("błąd zapisywania rezerwacji: %w", err)
	}

	return nil
}

// UpdateReservation aktualizuje rezerwację
func (c *Client) UpdateReservation(id string, reservation *models.Reservation) error {
	if id == "" {
		return fmt.Errorf("ID rezerwacji nie może być puste")
	}
	if reservation == nil {
		return fmt.Errorf("rezerwacja nie może być nil")
	}

	_, err := c.GetReservation(id)
	if err != nil {
		return fmt.Errorf("rezerwacja nie istnieje: %w", err)
	}

	reservation.UpdatedAt = time.Now()
	reservation.ID = id

	_, err = c.Firestore.Collection(ReservationsCollection).Doc(id).Set(c.ctx, reservation)
	if err != nil {
		return fmt.Errorf("błąd aktualizacji rezerwacji: %w", err)
	}

	return nil
}

// MarkReservationReady oznacza rezerwację jako gotową do odbioru
func (c *Client) MarkReservationReady(reservationID string) error {
	reservation, err := c.GetReservation(reservationID)
	if err != nil {
		return err
	}

	if reservation.Status != models.ReservationStatusPending {
		return fmt.Errorf("rezerwacja nie jest w stanie oczekiwania")
	}

	now := time.Now()
	reservation.Status = models.ReservationStatusReady
	reservation.NotifiedDate = &now
	reservation.ExpiryDate = now.AddDate(0, 0, 3) // 3 dni na odbiór
	reservation.UpdatedAt = now

	return c.UpdateReservation(reservationID, reservation)
}

// CompleteReservation realizuje rezerwację (zamienia na wypożyczenie)
func (c *Client) CompleteReservation(reservationID string) error {
	reservation, err := c.GetReservation(reservationID)
	if err != nil {
		return err
	}

	if !reservation.CanBeCompleted() {
		return fmt.Errorf("rezerwacja nie może być zrealizowana")
	}

	reservation.Status = models.ReservationStatusCompleted
	reservation.UpdatedAt = time.Now()

	return c.UpdateReservation(reservationID, reservation)
}

// CancelReservation anuluje rezerwację
func (c *Client) CancelReservation(reservationID string) error {
	reservation, err := c.GetReservation(reservationID)
	if err != nil {
		return err
	}

	if reservation.Status == models.ReservationStatusCompleted {
		return fmt.Errorf("nie można anulować zrealizowanej rezerwacji")
	}

	reservation.Status = models.ReservationStatusCancelled
	reservation.UpdatedAt = time.Now()

	return c.UpdateReservation(reservationID, reservation)
}

// ListReservations pobiera wszystkie rezerwacje
func (c *Client) ListReservations() ([]*models.Reservation, error) {
	var reservations []*models.Reservation

	iter := c.Firestore.Collection(ReservationsCollection).
		OrderBy("reservation_date", firestore.Desc).
		Documents(c.ctx)
	defer iter.Stop()

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("błąd iteracji po rezerwacjach: %w", err)
		}

		var reservation models.Reservation
		if err := doc.DataTo(&reservation); err != nil {
			return nil, fmt.Errorf("błąd parsowania rezerwacji: %w", err)
		}

		reservations = append(reservations, &reservation)
	}

	return reservations, nil
}

// GetUserReservations pobiera rezerwacje użytkownika
func (c *Client) GetUserReservations(userID string) ([]*models.Reservation, error) {
	if userID == "" {
		return nil, fmt.Errorf("ID użytkownika nie może być puste")
	}

	var reservations []*models.Reservation

	iter := c.Firestore.Collection(ReservationsCollection).
		Where("user_id", "==", userID).
		OrderBy("reservation_date", firestore.Desc).
		Documents(c.ctx)
	defer iter.Stop()

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("błąd iteracji po rezerwacjach: %w", err)
		}

		var reservation models.Reservation
		if err := doc.DataTo(&reservation); err != nil {
			return nil, fmt.Errorf("błąd parsowania rezerwacji: %w", err)
		}

		reservations = append(reservations, &reservation)
	}

	return reservations, nil
}

// GetBookReservations pobiera rezerwacje książki
func (c *Client) GetBookReservations(bookID string) ([]*models.Reservation, error) {
	if bookID == "" {
		return nil, fmt.Errorf("ID książki nie może być puste")
	}

	var reservations []*models.Reservation

	iter := c.Firestore.Collection(ReservationsCollection).
		Where("book_id", "==", bookID).
		OrderBy("reservation_date", firestore.Asc).
		Documents(c.ctx)
	defer iter.Stop()

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("błąd iteracji po rezerwacjach: %w", err)
		}

		var reservation models.Reservation
		if err := doc.DataTo(&reservation); err != nil {
			return nil, fmt.Errorf("błąd parsowania rezerwacji: %w", err)
		}

		reservations = append(reservations, &reservation)
	}

	return reservations, nil
}

// GetPendingReservations pobiera oczekujące rezerwacje
func (c *Client) GetPendingReservations() ([]*models.Reservation, error) {
	var reservations []*models.Reservation

	iter := c.Firestore.Collection(ReservationsCollection).
		Where("status", "==", string(models.ReservationStatusPending)).
		OrderBy("reservation_date", firestore.Asc).
		Documents(c.ctx)
	defer iter.Stop()

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("błąd iteracji po rezerwacjach: %w", err)
		}

		var reservation models.Reservation
		if err := doc.DataTo(&reservation); err != nil {
			return nil, fmt.Errorf("błąd parsowania rezerwacji: %w", err)
		}

		reservations = append(reservations, &reservation)
	}

	return reservations, nil
}

// GetReadyReservations pobiera gotowe do odbioru rezerwacje
func (c *Client) GetReadyReservations() ([]*models.Reservation, error) {
	var reservations []*models.Reservation

	iter := c.Firestore.Collection(ReservationsCollection).
		Where("status", "==", string(models.ReservationStatusReady)).
		OrderBy("expiry_date", firestore.Asc).
		Documents(c.ctx)
	defer iter.Stop()

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("błąd iteracji po rezerwacjach: %w", err)
		}

		var reservation models.Reservation
		if err := doc.DataTo(&reservation); err != nil {
			return nil, fmt.Errorf("błąd parsowania rezerwacji: %w", err)
		}

		reservations = append(reservations, &reservation)
	}

	return reservations, nil
}

// GetUserActiveReservations pobiera aktywne rezerwacje użytkownika
func (c *Client) GetUserActiveReservations(userID string) ([]*models.Reservation, error) {
	if userID == "" {
		return nil, fmt.Errorf("ID użytkownika nie może być puste")
	}

	var reservations []*models.Reservation

	// Pobierz wszystkie rezerwacje użytkownika i filtruj po stronie aplikacji
	iter := c.Firestore.Collection(ReservationsCollection).
		Where("user_id", "==", userID).
		Documents(c.ctx)
	defer iter.Stop()

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("błąd iteracji po rezerwacjach: %w", err)
		}

		var reservation models.Reservation
		if err := doc.DataTo(&reservation); err != nil {
			return nil, fmt.Errorf("błąd parsowania rezerwacji: %w", err)
		}

		// Filtruj tylko aktywne (pending i ready)
		if reservation.Status == models.ReservationStatusPending ||
			reservation.Status == models.ReservationStatusReady {
			reservations = append(reservations, &reservation)
		}
	}

	return reservations, nil
}

// GetNextReservation pobiera pierwszą oczekującą rezerwację dla książki (najstarsza pending)
func (c *Client) GetNextReservation(bookID string) (*models.Reservation, error) {
	if bookID == "" {
		return nil, fmt.Errorf("ID książki nie może być puste")
	}

	// Pobierz wszystkie rezerwacje dla książki (bez OrderBy aby uniknąć composite index)
	var pendingReservations []*models.Reservation

	iter := c.Firestore.Collection(ReservationsCollection).
		Where("book_id", "==", bookID).
		Documents(c.ctx)
	defer iter.Stop()

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("błąd pobierania rezerwacji: %w", err)
		}

		var reservation models.Reservation
		if err := doc.DataTo(&reservation); err != nil {
			return nil, fmt.Errorf("błąd parsowania rezerwacji: %w", err)
		}

		// Filtruj tylko pending
		if reservation.Status == models.ReservationStatusPending {
			pendingReservations = append(pendingReservations, &reservation)
		}
	}

	// Jeśli brak pending rezerwacji, zwróć nil
	if len(pendingReservations) == 0 {
		return nil, nil
	}

	// Sortuj po created_at (najstarsza pierwsza - FIFO)
	var oldest *models.Reservation
	for _, r := range pendingReservations {
		if oldest == nil || r.CreatedAt.Before(oldest.CreatedAt) {
			oldest = r
		}
	}

	return oldest, nil
}
