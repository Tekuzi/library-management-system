package middleware

import (
	"context"
	"net/http"

	"library-management-system/internal/models"
	"library-management-system/internal/session"
)

const sessionContextKey contextKey = "session"

// SessionMiddleware dodaje sesję do kontekstu jeśli istnieje
func SessionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, exists := session.GetSessionFromRequest(r)
		if exists {
			ctx := context.WithValue(r.Context(), sessionContextKey, sess)
			r = r.WithContext(ctx)
		}
		next.ServeHTTP(w, r)
	})
}

// RequireAuth wymaga zalogowania użytkownika
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess := GetSessionFromContext(r.Context())
		if sess == nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireAuthRole wymaga zalogowania i określonej roli
func RequireAuthRole(role models.UserRole) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sess := GetSessionFromContext(r.Context())
			if sess == nil {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}

			if sess.User.Role != role {
				http.Error(w, "Brak uprawnień", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// GetSessionFromContext pobiera sesję z kontekstu
func GetSessionFromContext(ctx context.Context) *session.Session {
	sess, ok := ctx.Value(sessionContextKey).(*session.Session)
	if !ok {
		return nil
	}
	return sess
}
