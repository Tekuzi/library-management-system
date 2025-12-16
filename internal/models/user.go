package models

import "time"

// UserRole określa rolę użytkownika w systemie
type UserRole string

const (
	RoleReader UserRole = "reader" // Czytelnik - może wypożyczać książki
	RoleAdmin  UserRole = "admin"  // Administrator - pełny dostęp do panelu staff
)

// User reprezentuje użytkownika systemu
type User struct {
	ID           string    `json:"id" firestore:"id"`
	FirebaseUID  string    `json:"firebase_uid" firestore:"firebase_uid"` // UID z Firebase Auth
	Email        string    `json:"email" firestore:"email"`
	FirstName    string    `json:"first_name" firestore:"first_name"`
	LastName     string    `json:"last_name" firestore:"last_name"`
	Role         UserRole  `json:"role" firestore:"role"`
	Phone        string    `json:"phone" firestore:"phone"`
	IsActive     bool      `json:"is_active" firestore:"is_active"`
	MaxLoans     int       `json:"max_loans" firestore:"max_loans"`         // Maksymalna liczba wypożyczeń
	CurrentLoans int       `json:"current_loans" firestore:"current_loans"` // Aktualna liczba wypożyczeń
	TotalFines   float64   `json:"total_fines" firestore:"total_fines"`     // Suma kar
	CreatedAt    time.Time `json:"created_at" firestore:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" firestore:"updated_at"`
}

// CanBorrow sprawdza czy użytkownik może wypożyczyć książkę
func (u *User) CanBorrow() bool {
	return u.IsActive && u.CurrentLoans < u.MaxLoans
}

// IsAdmin sprawdza czy użytkownik jest administratorem
func (u *User) IsAdmin() bool {
	return u.Role == RoleAdmin
}

// FullName zwraca pełne imię i nazwisko użytkownika
func (u *User) FullName() string {
	return u.FirstName + " " + u.LastName
}
