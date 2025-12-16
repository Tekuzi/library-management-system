package models

import "time"

// Book reprezentuje książkę w systemie bibliotecznym
type Book struct {
	ID              string    `json:"id" firestore:"id"`
	ISBN            string    `json:"isbn" firestore:"isbn"`
	Title           string    `json:"title" firestore:"title"`
	Author          string    `json:"author" firestore:"author"`
	Publisher       string    `json:"publisher" firestore:"publisher"`
	PublicationYear int       `json:"publication_year" firestore:"publication_year"`
	Category        string    `json:"category" firestore:"category"`
	Description     string    `json:"description" firestore:"description"`
	TotalCopies     int       `json:"total_copies" firestore:"total_copies"`
	AvailableCopies int       `json:"available_copies" firestore:"available_copies"`
	ShelfLocation   string    `json:"shelf_location" firestore:"shelf_location"`
	CoverImageURL   string    `json:"cover_image_url" firestore:"cover_image_url"`
	CreatedAt       time.Time `json:"created_at" firestore:"created_at"`
	UpdatedAt       time.Time `json:"updated_at" firestore:"updated_at"`
}

// IsAvailable sprawdza czy książka jest dostępna do wypożyczenia
func (b *Book) IsAvailable() bool {
	return b.AvailableCopies > 0
}

// DecrementAvailableCopies zmniejsza liczbę dostępnych egzemplarzy
func (b *Book) DecrementAvailableCopies() {
	if b.AvailableCopies > 0 {
		b.AvailableCopies--
	}
}

// IncrementAvailableCopies zwiększa liczbę dostępnych egzemplarzy
func (b *Book) IncrementAvailableCopies() {
	if b.AvailableCopies < b.TotalCopies {
		b.AvailableCopies++
	}
}
