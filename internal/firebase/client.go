package firebase

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
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

// VerifyPassword weryfikuje email i hasło używając Firebase Authentication REST API
func (c *Client) VerifyPassword(email, password string) (string, error) {
	apiKey := os.Getenv("FIREBASE_WEB_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("brak FIREBASE_WEB_API_KEY w zmiennych środowiskowych")
	}

	// Firebase Identity Toolkit REST API endpoint
	url := fmt.Sprintf("https://identitytoolkit.googleapis.com/v1/accounts:signInWithPassword?key=%s", apiKey)

	requestBody := map[string]interface{}{
		"email":             email,
		"password":          password,
		"returnSecureToken": true,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("błąd tworzenia żądania: %w", err)
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("błąd połączenia z Firebase Auth: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("błąd odczytu odpowiedzi: %w", err)
	}

	// Sprawdź kod odpowiedzi
	if resp.StatusCode != http.StatusOK {
		var errorResp struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(body, &errorResp); err == nil {
			// Typowe błędy Firebase Auth
			switch errorResp.Error.Message {
			case "EMAIL_NOT_FOUND":
				return "", fmt.Errorf("nieprawidłowy email lub hasło")
			case "INVALID_PASSWORD":
				return "", fmt.Errorf("nieprawidłowy email lub hasło")
			case "USER_DISABLED":
				return "", fmt.Errorf("konto zostało zablokowane")
			case "INVALID_LOGIN_CREDENTIALS":
				return "", fmt.Errorf("nieprawidłowy email lub hasło")
			default:
				return "", fmt.Errorf("błąd autoryzacji: %s", errorResp.Error.Message)
			}
		}
		return "", fmt.Errorf("błąd weryfikacji hasła (status: %d)", resp.StatusCode)
	}

	// Parsuj odpowiedź
	var authResp struct {
		LocalID string `json:"localId"` // Firebase UID
		IDToken string `json:"idToken"`
		Email   string `json:"email"`
	}

	if err := json.Unmarshal(body, &authResp); err != nil {
		return "", fmt.Errorf("błąd parsowania odpowiedzi: %w", err)
	}

	// Zwróć Firebase UID
	return authResp.LocalID, nil
}
