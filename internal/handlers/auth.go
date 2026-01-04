package handlers

import (
	"html/template"
	"log"
	"net/http"

	"library-management-system/internal/firebase"
	"library-management-system/internal/models"
	"library-management-system/internal/session"

	"firebase.google.com/go/v4/auth"
)

// AuthHandler obsługuje logowanie i rejestrację
type AuthHandler struct {
	loginTemplate    *template.Template
	registerTemplate *template.Template
}

// NewAuthHandler tworzy nowy handler autoryzacji
func NewAuthHandler() *AuthHandler {
	loginTmpl, err := template.ParseFiles("internal/templates/auth/login.html")
	if err != nil {
		log.Printf("Błąd ładowania szablonu login.html: %v", err)
	}

	registerTmpl, err := template.ParseFiles("internal/templates/auth/register.html")
	if err != nil {
		log.Printf("Błąd ładowania szablonu register.html: %v", err)
	}

	return &AuthHandler{
		loginTemplate:    loginTmpl,
		registerTemplate: registerTmpl,
	}
}

// ShowLoginPage wyświetla stronę logowania (GET /login)
func (h *AuthHandler) ShowLoginPage(w http.ResponseWriter, r *http.Request) {
	if h.loginTemplate == nil {
		http.Error(w, "Szablon logowania nie został załadowany", http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"Error": nil,
	}

	if err := h.loginTemplate.Execute(w, data); err != nil {
		log.Printf("Błąd renderowania strony logowania: %v", err)
		http.Error(w, "Błąd renderowania strony", http.StatusInternalServerError)
	}
}

// HandleLogin obsługuje logowanie (POST /login)
func (h *AuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	email := r.FormValue("email")
	password := r.FormValue("password")

	if email == "" || password == "" {
		h.renderLoginError(w, "Email i hasło są wymagane")
		return
	}

	// Sprawdź czy Firebase jest zainicjalizowany
	if firebase.GlobalClient == nil {
		h.renderLoginError(w, "System autoryzacji nie jest dostępny")
		return
	}

	// Weryfikuj email i hasło przez Firebase Authentication REST API
	firebaseUID, err := firebase.GlobalClient.VerifyPassword(email, password)
	if err != nil {
		log.Printf("Błąd weryfikacji hasła: %v", err)
		h.renderLoginError(w, err.Error())
		return
	}

	// Pobierz użytkownika z Firestore po Firebase UID
	dbUser, err := firebase.GlobalClient.GetUserByFirebaseUID(firebaseUID)
	if err != nil {
		log.Printf("Użytkownik nie znaleziony w bazie: %v", err)
		h.renderLoginError(w, "Użytkownik nie istnieje w systemie")
		return
	}

	if !dbUser.IsActive {
		h.renderLoginError(w, "Konto zostało dezaktywowane")
		return
	}

	// Utwórz sesję
	sess, err := session.GetManager().CreateSession(dbUser)
	if err != nil {
		log.Printf("Błąd tworzenia sesji: %v", err)
		h.renderLoginError(w, "Błąd logowania")
		return
	}

	// Ustaw cookie z sesją
	session.SetSessionCookie(w, sess.ID)

	log.Printf("Użytkownik zalogowany: %s (%s)", email, dbUser.Role)

	// Przekieruj w zależności od roli
	if dbUser.Role == models.RoleAdmin {
		http.Redirect(w, r, "/staff", http.StatusSeeOther)
	} else {
		http.Redirect(w, r, "/books", http.StatusSeeOther)
	}
}

// ShowRegisterPage wyświetla stronę rejestracji (GET /register)
func (h *AuthHandler) ShowRegisterPage(w http.ResponseWriter, r *http.Request) {
	if h.registerTemplate == nil {
		http.Error(w, "Szablon rejestracji nie został załadowany", http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"Error": nil,
	}

	if err := h.registerTemplate.Execute(w, data); err != nil {
		log.Printf("Błąd renderowania strony rejestracji: %v", err)
		http.Error(w, "Błąd renderowania strony", http.StatusInternalServerError)
	}
}

// HandleRegister obsługuje rejestrację (POST /register)
func (h *AuthHandler) HandleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/register", http.StatusSeeOther)
		return
	}

	// Pobierz dane z formularza
	firstName := r.FormValue("first_name")
	lastName := r.FormValue("last_name")
	email := r.FormValue("email")
	phone := r.FormValue("phone")
	password := r.FormValue("password")

	// Walidacja
	if firstName == "" || lastName == "" || email == "" || password == "" {
		h.renderRegisterError(w, "Imię, nazwisko, email i hasło są wymagane")
		return
	}

	if len(password) < 6 {
		h.renderRegisterError(w, "Hasło musi mieć minimum 6 znaków")
		return
	}

	// Sprawdź czy Firebase jest zainicjalizowany
	if firebase.GlobalClient == nil {
		h.renderRegisterError(w, "System autoryzacji nie jest dostępny")
		return
	}

	// Utwórz użytkownika w Firebase Auth
	params := (&auth.UserToCreate{}).
		Email(email).
		Password(password).
		DisplayName(firstName + " " + lastName)

	firebaseUser, err := firebase.GlobalClient.Auth.CreateUser(r.Context(), params)
	if err != nil {
		log.Printf("Błąd tworzenia użytkownika w Firebase Auth: %v", err)
		h.renderRegisterError(w, "Użytkownik z tym adresem email już istnieje lub hasło jest za słabe")
		return
	}

	// Utwórz użytkownika w Firestore
	user := &models.User{
		FirebaseUID: firebaseUser.UID,
		Email:       email,
		FirstName:   firstName,
		LastName:    lastName,
		Phone:       phone,
		Role:        models.RoleReader,
		IsActive:    true,
		MaxLoans:    5,
	}

	if err := firebase.GlobalClient.CreateUser(user); err != nil {
		log.Printf("Błąd tworzenia użytkownika w Firestore: %v", err)
		// Próba usunięcia użytkownika z Auth jeśli nie udało się dodać do Firestore
		firebase.GlobalClient.Auth.DeleteUser(r.Context(), firebaseUser.UID)
		h.renderRegisterError(w, "Błąd tworzenia konta użytkownika")
		return
	}

	log.Printf("Nowy użytkownik zarejestrowany: %s %s (%s)", firstName, lastName, email)

	// Automatycznie zaloguj użytkownika
	sess, err := session.GetManager().CreateSession(user)
	if err != nil {
		log.Printf("Błąd tworzenia sesji: %v", err)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	session.SetSessionCookie(w, sess.ID)
	log.Printf("Użytkownik automatycznie zalogowany po rejestracji")

	// Przekieruj na stronę książek
	http.Redirect(w, r, "/books", http.StatusSeeOther)
}

func (h *AuthHandler) renderLoginError(w http.ResponseWriter, errorMsg string) {
	if h.loginTemplate == nil {
		http.Error(w, errorMsg, http.StatusBadRequest)
		return
	}

	data := map[string]interface{}{
		"Error": errorMsg,
	}

	h.loginTemplate.Execute(w, data)
}

func (h *AuthHandler) renderRegisterError(w http.ResponseWriter, errorMsg string) {
	if h.registerTemplate == nil {
		http.Error(w, errorMsg, http.StatusBadRequest)
		return
	}

	data := map[string]interface{}{
		"Error": errorMsg,
	}

	h.registerTemplate.Execute(w, data)
}

// HandleLogout obsługuje wylogowanie
func (h *AuthHandler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	sess, exists := session.GetSessionFromRequest(r)
	if exists {
		session.GetManager().DeleteSession(sess.ID)
	}

	session.ClearSessionCookie(w)
	log.Println("Użytkownik wylogowany")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
