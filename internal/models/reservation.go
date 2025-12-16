package models

import "time"

// ReservationStatus określa status rezerwacji
type ReservationStatus string

const (
	ReservationStatusPending   ReservationStatus = "pending"   // Oczekująca
	ReservationStatusReady     ReservationStatus = "ready"     // Gotowa do odbioru
	ReservationStatusCompleted ReservationStatus = "completed" // Zrealizowana
	ReservationStatusCancelled ReservationStatus = "cancelled" // Anulowana
	ReservationStatusExpired   ReservationStatus = "expired"   // Wygasła
)

// Reservation reprezentuje rezerwację książki
type Reservation struct {
	ID              string            `json:"id" firestore:"id"`
	BookID          string            `json:"book_id" firestore:"book_id"`
	UserID          string            `json:"user_id" firestore:"user_id"`
	BookTitle       string            `json:"book_title" firestore:"book_title"` // Denormalizacja
	UserName        string            `json:"user_name" firestore:"user_name"`   // Denormalizacja
	Status          ReservationStatus `json:"status" firestore:"status"`
	ReservationDate time.Time         `json:"reservation_date" firestore:"reservation_date"`
	ExpiryDate      time.Time         `json:"expiry_date" firestore:"expiry_date"`                         // Data wygaśnięcia rezerwacji
	NotifiedDate    *time.Time        `json:"notified_date,omitempty" firestore:"notified_date,omitempty"` // Kiedy powiadomiono użytkownika
	Notes           string            `json:"notes" firestore:"notes"`
	CreatedAt       time.Time         `json:"created_at" firestore:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at" firestore:"updated_at"`
}

// IsExpired sprawdza czy rezerwacja wygasła
func (r *Reservation) IsExpired() bool {
	return r.Status == ReservationStatusReady && time.Now().After(r.ExpiryDate)
}

// CanBeCompleted sprawdza czy rezerwacja może być zrealizowana
func (r *Reservation) CanBeCompleted() bool {
	return r.Status == ReservationStatusReady && !r.IsExpired()
}

// DaysUntilExpiry zwraca liczbę dni do wygaśnięcia rezerwacji
func (r *Reservation) DaysUntilExpiry() int {
	if r.Status != ReservationStatusReady {
		return 0
	}

	days := int(time.Until(r.ExpiryDate).Hours() / 24)
	if days < 0 {
		return 0
	}
	return days
}
