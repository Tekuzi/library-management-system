package firebase

import (
	"context"
	"fmt"
	"log"
	"os"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"google.golang.org/api/option"
)

// UserToCreate reprezentuje parametry do utworzenia użytkownika w Firebase Auth
type UserToCreate auth.UserToCreate

// Email ustawia email dla nowego użytkownika
func (u *UserToCreate) Email(email string) *UserToCreate {
	(*auth.UserToCreate)(u).Email(email)
	return u
}

// Password ustawia hasło dla nowego użytkownika
func (u *UserToCreate) Password(password string) *UserToCreate {
	(*auth.UserToCreate)(u).Password(password)
	return u
}

// DisplayName ustawia nazwę wyświetlaną dla nowego użytkownika
func (u *UserToCreate) DisplayName(displayName string) *UserToCreate {
	(*auth.UserToCreate)(u).DisplayName(displayName)
	return u
}

// Client zawiera klientów Firebase
type Client struct {
	App       *firebase.App
	Auth      *auth.Client
	Firestore *firestore.Client
	ctx       context.Context
}

var (
	// GlobalClient to globalna instancja klienta Firebase
	GlobalClient *Client
)

// InitFirebase inicjalizuje klienta Firebase
func InitFirebase() (*Client, error) {
	ctx := context.Background()

	var app *firebase.App
	var err error

	// Sprawdź czy jest plik credentials (rozwój lokalny)
	credentialsPath := os.Getenv("FIREBASE_CREDENTIALS_PATH")
	if credentialsPath != "" {
		// Tryb lokalny - użyj pliku
		if _, err := os.Stat(credentialsPath); os.IsNotExist(err) {
			return nil, fmt.Errorf("plik credentials nie istnieje: %s", credentialsPath)
		}
		opt := option.WithCredentialsFile(credentialsPath)
		app, err = firebase.NewApp(ctx, nil, opt)
		if err != nil {
			return nil, fmt.Errorf("błąd inicjalizacji Firebase App: %w", err)
		}
	} else {
		// Tryb produkcyjny - użyj JSON z zmiennej środowiskowej
		credentialsJSON := os.Getenv("FIREBASE_CREDENTIALS_JSON")
		if credentialsJSON == "" {
			return nil, fmt.Errorf("brak zmiennej środowiskowej FIREBASE_CREDENTIALS_PATH lub FIREBASE_CREDENTIALS_JSON")
		}
		opt := option.WithCredentialsJSON([]byte(credentialsJSON))
		app, err = firebase.NewApp(ctx, nil, opt)
		if err != nil {
			return nil, fmt.Errorf("błąd inicjalizacji Firebase App: %w", err)
		}
	}

	// Inicjalizacja Auth Client
	authClient, err := app.Auth(ctx)
	if err != nil {
		return nil, fmt.Errorf("błąd inicjalizacji Firebase Auth: %w", err)
	}

	// Inicjalizacja Firestore Client
	firestoreClient, err := app.Firestore(ctx)
	if err != nil {
		return nil, fmt.Errorf("błąd inicjalizacji Firestore: %w", err)
	}

	client := &Client{
		App:       app,
		Auth:      authClient,
		Firestore: firestoreClient,
		ctx:       ctx,
	}

	// Ustaw globalnego klienta
	GlobalClient = client

	log.Println("Firebase zainicjalizowany pomyślnie")
	return client, nil
}

// Close zamyka połączenia z Firebase
func (c *Client) Close() error {
	if c.Firestore != nil {
		return c.Firestore.Close()
	}
	return nil
}

// GetContext zwraca kontekst klienta
func (c *Client) GetContext() context.Context {
	return c.ctx
}
