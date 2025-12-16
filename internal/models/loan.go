package models

import "time"

// LoanStatus określa status wypożyczenia
type LoanStatus string

const (
	LoanStatusPendingPickup LoanStatus = "pending_pickup" // Oczekuje na odbiór
	LoanStatusActive        LoanStatus = "active"         // Aktywne wypożyczenie
	LoanStatusReturned      LoanStatus = "returned"       // Zwrócone
	LoanStatusOverdue       LoanStatus = "overdue"        // Przeterminowane
)

// Loan reprezentuje wypożyczenie książki
type Loan struct {
	ID         string     `json:"id" firestore:"id"`
	BookID     string     `json:"book_id" firestore:"book_id"`
	UserID     string     `json:"user_id" firestore:"user_id"`
	BookTitle  string     `json:"book_title" firestore:"book_title"`   // Denormalizacja dla łatwiejszego wyświetlania
	UserName   string     `json:"user_name" firestore:"user_name"`     // Denormalizacja dla łatwiejszego wyświetlania
	PickupCode string     `json:"pickup_code" firestore:"pickup_code"` // Kod odbioru
	Status     LoanStatus `json:"status" firestore:"status"`
	LoanDate   time.Time  `json:"loan_date" firestore:"loan_date"`
	DueDate    time.Time  `json:"due_date" firestore:"due_date"`
	ReturnDate *time.Time `json:"return_date,omitempty" firestore:"return_date,omitempty"`
	FineAmount float64    `json:"fine_amount" firestore:"fine_amount"` // Kara za opóźnienie
	Notes      string     `json:"notes" firestore:"notes"`
	CreatedAt  time.Time  `json:"created_at" firestore:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at" firestore:"updated_at"`
}

// IsOverdue sprawdza czy wypożyczenie jest przeterminowane
func (l *Loan) IsOverdue() bool {
	return l.Status == LoanStatusActive && time.Now().After(l.DueDate)
}

// CalculateFine oblicza karę za opóźnienie (1 zł za każdy dzień)
func (l *Loan) CalculateFine() float64 {
	if !l.IsOverdue() {
		return 0
	}

	daysOverdue := int(time.Since(l.DueDate).Hours() / 24)
	if daysOverdue < 0 {
		return 0
	}

	// 1 zł za każdy dzień opóźnienia
	return float64(daysOverdue) * 1.0
}

// DaysUntilDue zwraca liczbę dni do terminu zwrotu
func (l *Loan) DaysUntilDue() int {
	if l.Status != LoanStatusActive {
		return 0
	}

	days := int(time.Until(l.DueDate).Hours() / 24)
	return days
}
