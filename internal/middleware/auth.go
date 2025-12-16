package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"library-management-system/internal/firebase"
	"library-management-system/internal/models"
)

// Klucze do przechowywania wartości w context
type contextKey string

const (
	UserUIDKey  contextKey = "user_uid"
	UserRoleKey contextKey = "user_role"
	UserKey     contextKey = "user"
)

// AuthMiddleware weryfikuje token Firebase i dodaje dane użytkownika do kontekstu
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Pobierz token z nagłówka Authorization
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Brak nagłówka Authorization", http.StatusUnauthorized)
			return
		}

		// Sprawdź format: "Bearer <token>"
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, "Nieprawidłowy format Authorization", http.StatusUnauthorized)
			return
		}

		token := parts[1]

		// Weryfikuj token przez Firebase Admin SDK
		if firebase.GlobalClient == nil {
			http.Error(w, "Firebase nie został zainicjalizowany", http.StatusInternalServerError)
			return
		}

		decodedToken, err := firebase.GlobalClient.Auth.VerifyIDToken(r.Context(), token)
		if err != nil {
			http.Error(w, fmt.Sprintf("Nieprawidłowy token: %v", err), http.StatusUnauthorized)
			return
		}

		// Pobierz dane użytkownika z bazy na podstawie Firebase UID
		user, err := firebase.GlobalClient.GetUserByFirebaseUID(decodedToken.UID)
		if err != nil {
			http.Error(w, "Użytkownik nie został znaleziony", http.StatusUnauthorized)
			return
		}

		// Sprawdź czy użytkownik jest aktywny
		if !user.IsActive {
			http.Error(w, "Konto użytkownika jest nieaktywne", http.StatusForbidden)
			return
		}

		// Dodaj dane użytkownika do kontekstu
		ctx := context.WithValue(r.Context(), UserUIDKey, decodedToken.UID)
		ctx = context.WithValue(ctx, UserRoleKey, user.Role)
		ctx = context.WithValue(ctx, UserKey, user)

		// Przekaż żądanie dalej z nowym kontekstem
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// OptionalAuthMiddleware próbuje uwierzytelnić użytkownika, ale nie wymaga tego
func OptionalAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			next.ServeHTTP(w, r)
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			next.ServeHTTP(w, r)
			return
		}

		token := parts[1]

		if firebase.GlobalClient != nil {
			decodedToken, err := firebase.GlobalClient.Auth.VerifyIDToken(r.Context(), token)
			if err == nil {
				user, err := firebase.GlobalClient.GetUserByFirebaseUID(decodedToken.UID)
				if err == nil && user.IsActive {
					ctx := context.WithValue(r.Context(), UserUIDKey, decodedToken.UID)
					ctx = context.WithValue(ctx, UserRoleKey, user.Role)
					ctx = context.WithValue(ctx, UserKey, user)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}
		}

		next.ServeHTTP(w, r)
	})
}

// RequireRole zwraca middleware, który wymaga określonej roli
func RequireRole(role models.UserRole) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userRole, ok := r.Context().Value(UserRoleKey).(models.UserRole)
			if !ok {
				http.Error(w, "Brak danych o roli użytkownika", http.StatusUnauthorized)
				return
			}

			// Admin ma dostęp do wszystkiego
			if userRole == models.RoleAdmin {
				next.ServeHTTP(w, r)
				return
			}

			// Sprawdź czy użytkownik ma wymaganą rolę
			if userRole != role {
				http.Error(w, "Brak uprawnień", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireAdmin wymaga roli administratora
func RequireAdmin(next http.Handler) http.Handler {
	return RequireRole(models.RoleAdmin)(next)
}

// GetUserFromContext pobiera dane użytkownika z kontekstu
func GetUserFromContext(ctx context.Context) (*models.User, error) {
	user, ok := ctx.Value(UserKey).(*models.User)
	if !ok {
		return nil, fmt.Errorf("brak danych użytkownika w kontekście")
	}
	return user, nil
}

// GetUserUIDFromContext pobiera Firebase UID z kontekstu
func GetUserUIDFromContext(ctx context.Context) (string, error) {
	uid, ok := ctx.Value(UserUIDKey).(string)
	if !ok {
		return "", fmt.Errorf("brak UID użytkownika w kontekście")
	}
	return uid, nil
}

// GetUserRoleFromContext pobiera rolę użytkownika z kontekstu
func GetUserRoleFromContext(ctx context.Context) (models.UserRole, error) {
	role, ok := ctx.Value(UserRoleKey).(models.UserRole)
	if !ok {
		return "", fmt.Errorf("brak roli użytkownika w kontekście")
	}
	return role, nil
}
