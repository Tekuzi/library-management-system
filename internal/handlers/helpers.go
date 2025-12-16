package handlers

import (
	"library-management-system/internal/models"
	"library-management-system/internal/session"
)

// TemplateData zawiera wspólne dane dla wszystkich szablonów
type TemplateData map[string]interface{}

// NewTemplateData tworzy nowe dane szablonu z automatycznym dodaniem użytkownika
func NewTemplateData(sess *session.Session) TemplateData {
	data := make(TemplateData)

	if sess != nil {
		data["User"] = sess.User
		data["IsLoggedIn"] = true
		data["IsAdmin"] = sess.User.Role == models.RoleAdmin
	} else {
		data["User"] = nil
		data["IsLoggedIn"] = false
		data["IsAdmin"] = false
	}

	return data
}

// Set ustawia wartość w danych szablonu
func (t TemplateData) Set(key string, value interface{}) TemplateData {
	t[key] = value
	return t
}
