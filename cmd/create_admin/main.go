package main

import (
	"fmt"
	"log"

	"github.com/joho/godotenv"

	"library-management-system/internal/firebase"
	"library-management-system/internal/models"

	firebaseAuth "firebase.google.com/go/v4/auth"
)

func main() {
	// Wczytaj zmienne środowiskowe
	if err := godotenv.Load(); err != nil {
		log.Println("Brak pliku .env - używam zmiennych systemowych")
	}

	// Inicjalizacja Firebase
	client, err := firebase.InitFirebase()
	if err != nil {
		log.Fatalf("Błąd inicjalizacji Firebase: %v", err)
	}

	fmt.Println("=== Tworzenie użytkownika admina ===")

	// Dane admina
	email := "admin@biblioteka.pl"
	password := "admin123"
	firstName := "Admin"
	lastName := "System"

	// Utwórz użytkownika w Firebase Auth
	params := (&firebaseAuth.UserToCreate{}).
		Email(email).
		Password(password).
		DisplayName(firstName + " " + lastName)

	firebaseUser, err := client.Auth.CreateUser(client.GetContext(), params)
	if err != nil {
		log.Fatalf("Błąd tworzenia użytkownika w Firebase Auth: %v", err)
	}

	fmt.Printf("✓ Utworzono użytkownika Auth: %s (UID: %s)\n", email, firebaseUser.UID)

	// Utwórz użytkownika w Firestore z rolą admin
	user := &models.User{
		FirebaseUID: firebaseUser.UID,
		Email:       email,
		FirstName:   firstName,
		LastName:    lastName,
		Phone:       "",
		Role:        models.RoleAdmin,
		IsActive:    true,
		MaxLoans:    10, // Admin może mieć więcej wypożyczeń
	}

	if err := client.CreateUser(user); err != nil {
		log.Fatalf("Błąd tworzenia użytkownika w Firestore: %v", err)
	}

	fmt.Printf("✓ Utworzono użytkownika Firestore: %s\n", user.ID)
	fmt.Println("\n=== Użytkownik admin utworzony pomyślnie ===")
	fmt.Printf("Email: %s\n", email)
	fmt.Printf("Hasło: %s\n", password)
	fmt.Printf("Rola: %s\n", user.Role)
	fmt.Println("\nMożesz teraz zalogować się do panelu admina.")
}
