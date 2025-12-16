package session

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"sync"
	"time"

	"library-management-system/internal/models"
)

const (
	sessionCookieName = "session_id"
	sessionDuration   = 24 * time.Hour
)

// Session reprezentuje sesję użytkownika
type Session struct {
	ID        string
	UserID    string
	User      *models.User
	CreatedAt time.Time
	ExpiresAt time.Time
}

// Manager zarządza sesjami użytkowników
type Manager struct {
	sessions map[string]*Session
	mu       sync.RWMutex
}

var globalManager *Manager

// Init inicjalizuje globalny manager sesji
func Init() {
	globalManager = &Manager{
		sessions: make(map[string]*Session),
	}

	// Uruchom czyszczenie wygasłych sesji co godzinę
	go globalManager.cleanupExpiredSessions()
}

// GetManager zwraca globalny manager sesji
func GetManager() *Manager {
	if globalManager == nil {
		Init()
	}
	return globalManager
}

// CreateSession tworzy nową sesję dla użytkownika
func (m *Manager) CreateSession(user *models.User) (*Session, error) {
	sessionID, err := generateSessionID()
	if err != nil {
		return nil, err
	}

	session := &Session{
		ID:        sessionID,
		UserID:    user.ID,
		User:      user,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(sessionDuration),
	}

	m.mu.Lock()
	m.sessions[sessionID] = session
	m.mu.Unlock()

	return session, nil
}

// GetSession pobiera sesję po ID
func (m *Manager) GetSession(sessionID string) (*Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return nil, false
	}

	// Sprawdź czy sesja nie wygasła
	if time.Now().After(session.ExpiresAt) {
		return nil, false
	}

	return session, true
}

// DeleteSession usuwa sesję
func (m *Manager) DeleteSession(sessionID string) {
	m.mu.Lock()
	delete(m.sessions, sessionID)
	m.mu.Unlock()
}

// SetSessionCookie ustawia cookie z ID sesji
func SetSessionCookie(w http.ResponseWriter, sessionID string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    sessionID,
		Path:     "/",
		MaxAge:   int(sessionDuration.Seconds()),
		HttpOnly: true,
		Secure:   false, // w produkcji ustaw na true (HTTPS)
		SameSite: http.SameSiteLaxMode,
	})
}

// ClearSessionCookie usuwa cookie z sesją
func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
}

// GetSessionFromRequest pobiera sesję z requesta
func GetSessionFromRequest(r *http.Request) (*Session, bool) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return nil, false
	}

	return GetManager().GetSession(cookie.Value)
}

// cleanupExpiredSessions usuwa wygasłe sesje co godzinę
func (m *Manager) cleanupExpiredSessions() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		m.mu.Lock()
		now := time.Now()
		for id, session := range m.sessions {
			if now.After(session.ExpiresAt) {
				delete(m.sessions, id)
			}
		}
		m.mu.Unlock()
	}
}

// generateSessionID generuje losowy ID sesji
func generateSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
