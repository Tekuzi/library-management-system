package firebase

import (
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"

	"library-management-system/internal/models"
)

const (
	// UsersCollection to nazwa kolekcji użytkowników w Firestore
	UsersCollection = "users"
)

// GetUser pobiera użytkownika po ID
func (c *Client) GetUser(id string) (*models.User, error) {
	if id == "" {
		return nil, fmt.Errorf("ID użytkownika nie może być puste")
	}

	doc, err := c.Firestore.Collection(UsersCollection).Doc(id).Get(c.ctx)
	if err != nil {
		return nil, fmt.Errorf("błąd pobierania użytkownika: %w", err)
	}

	var user models.User
	if err := doc.DataTo(&user); err != nil {
		return nil, fmt.Errorf("błąd parsowania danych użytkownika: %w", err)
	}

	return &user, nil
}

// GetUserByFirebaseUID pobiera użytkownika po Firebase UID
func (c *Client) GetUserByFirebaseUID(uid string) (*models.User, error) {
	if uid == "" {
		return nil, fmt.Errorf("Firebase UID nie może być pusty")
	}

	iter := c.Firestore.Collection(UsersCollection).
		Where("firebase_uid", "==", uid).
		Limit(1).
		Documents(c.ctx)
	defer iter.Stop()

	doc, err := iter.Next()
	if err == iterator.Done {
		return nil, fmt.Errorf("użytkownik nie został znaleziony")
	}
	if err != nil {
		return nil, fmt.Errorf("błąd wyszukiwania użytkownika: %w", err)
	}

	var user models.User
	if err := doc.DataTo(&user); err != nil {
		return nil, fmt.Errorf("błąd parsowania danych użytkownika: %w", err)
	}

	return &user, nil
}

// CreateUser tworzy nowego użytkownika
func (c *Client) CreateUser(user *models.User) error {
	if user == nil {
		return fmt.Errorf("użytkownik nie może być nil")
	}

	// Walidacja
	if user.Email == "" {
		return fmt.Errorf("email jest wymagany")
	}
	if user.FirstName == "" || user.LastName == "" {
		return fmt.Errorf("imię i nazwisko są wymagane")
	}

	// Domyślne wartości
	now := time.Now()
	user.CreatedAt = now
	user.UpdatedAt = now
	user.IsActive = true

	if user.Role == "" {
		user.Role = models.RoleReader
	}
	if user.MaxLoans == 0 {
		user.MaxLoans = 5 // Domyślnie 5 wypożyczeń
	}

	// Wygeneruj ID jeśli nie ma
	var docRef *firestore.DocumentRef
	if user.ID == "" {
		docRef = c.Firestore.Collection(UsersCollection).NewDoc()
		user.ID = docRef.ID
	} else {
		docRef = c.Firestore.Collection(UsersCollection).Doc(user.ID)
	}

	_, err := docRef.Set(c.ctx, user)
	if err != nil {
		return fmt.Errorf("błąd zapisywania użytkownika: %w", err)
	}

	return nil
}

// UpdateUser aktualizuje dane użytkownika
func (c *Client) UpdateUser(id string, user *models.User) error {
	if id == "" {
		return fmt.Errorf("ID użytkownika nie może być puste")
	}
	if user == nil {
		return fmt.Errorf("użytkownik nie może być nil")
	}

	// Sprawdź czy użytkownik istnieje
	_, err := c.GetUser(id)
	if err != nil {
		return fmt.Errorf("użytkownik nie istnieje: %w", err)
	}

	user.UpdatedAt = time.Now()
	user.ID = id

	_, err = c.Firestore.Collection(UsersCollection).Doc(id).Set(c.ctx, user)
	if err != nil {
		return fmt.Errorf("błąd aktualizacji użytkownika: %w", err)
	}

	return nil
}

// DeleteUser usuwa użytkownika
func (c *Client) DeleteUser(id string) error {
	if id == "" {
		return fmt.Errorf("ID użytkownika nie może być puste")
	}

	_, err := c.GetUser(id)
	if err != nil {
		return fmt.Errorf("użytkownik nie istnieje: %w", err)
	}

	_, err = c.Firestore.Collection(UsersCollection).Doc(id).Delete(c.ctx)
	if err != nil {
		return fmt.Errorf("błąd usuwania użytkownika: %w", err)
	}

	return nil
}

// ListUsers pobiera listę wszystkich użytkowników
func (c *Client) ListUsers() ([]*models.User, error) {
	var users []*models.User

	iter := c.Firestore.Collection(UsersCollection).
		OrderBy("last_name", firestore.Asc).
		Documents(c.ctx)
	defer iter.Stop()

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("błąd iteracji po użytkownikach: %w", err)
		}

		var user models.User
		if err := doc.DataTo(&user); err != nil {
			return nil, fmt.Errorf("błąd parsowania użytkownika: %w", err)
		}

		users = append(users, &user)
	}

	return users, nil
}

// GetActiveUsers pobiera tylko aktywnych użytkowników
func (c *Client) GetActiveUsers() ([]*models.User, error) {
	var users []*models.User

	iter := c.Firestore.Collection(UsersCollection).
		Where("is_active", "==", true).
		OrderBy("last_name", firestore.Asc).
		Documents(c.ctx)
	defer iter.Stop()

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("błąd iteracji po użytkownikach: %w", err)
		}

		var user models.User
		if err := doc.DataTo(&user); err != nil {
			return nil, fmt.Errorf("błąd parsowania użytkownika: %w", err)
		}

		users = append(users, &user)
	}

	return users, nil
}

// UpdateUserFines aktualizuje sumę kar użytkownika
func (c *Client) UpdateUserFines(userID string, amount float64) error {
	docRef := c.Firestore.Collection(UsersCollection).Doc(userID)

	_, err := docRef.Update(c.ctx, []firestore.Update{
		{Path: "total_fines", Value: firestore.Increment(amount)},
		{Path: "updated_at", Value: time.Now()},
	})

	if err != nil {
		return fmt.Errorf("błąd aktualizacji kar: %w", err)
	}

	return nil
}

// UpdateUserLoansCount aktualizuje liczbę aktywnych wypożyczeń użytkownika
func (c *Client) UpdateUserLoansCount(userID string, increment bool) error {
	docRef := c.Firestore.Collection(UsersCollection).Doc(userID)

	delta := 1
	if !increment {
		delta = -1
	}

	_, err := docRef.Update(c.ctx, []firestore.Update{
		{Path: "current_loans", Value: firestore.Increment(delta)},
		{Path: "updated_at", Value: time.Now()},
	})

	if err != nil {
		return fmt.Errorf("błąd aktualizacji liczby wypożyczeń: %w", err)
	}

	return nil
}

// CountTotalUsers zwraca całkowitą liczbę użytkowników w systemie
func (c *Client) CountTotalUsers() (int, error) {
	docs, err := c.Firestore.Collection(UsersCollection).Documents(c.ctx).GetAll()
	if err != nil {
		return 0, fmt.Errorf("błąd liczenia użytkowników: %w", err)
	}
	return len(docs), nil
}
