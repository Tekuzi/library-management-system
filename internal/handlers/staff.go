package handlers

import (
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"library-management-system/internal/firebase"
	"library-management-system/internal/middleware"
	"library-management-system/internal/models"
)

type StaffHandler struct {
	dashboardTemplate      *template.Template
	loansTemplate          *template.Template
	usersTemplate          *template.Template
	userEditTemplate       *template.Template
	reportsTemplate        *template.Template
	pendingPickupsTemplate *template.Template
	fbClient               *firebase.Client
}

type LoanDisplay struct {
	ID          string
	BookTitle   string
	BookAuthor  string
	UserName    string
	UserEmail   string
	LoanDate    time.Time
	DueDate     time.Time
	ReturnDate  *time.Time
	Status      string
	FineAmount  float64
	IsOverdue   bool
	DaysOverdue int
}

func NewStaffHandler(fbClient *firebase.Client) *StaffHandler {
	dashboardTmpl, err := template.ParseFiles("internal/templates/staff/dashboard.html")
	if err != nil {
		log.Printf("Błąd ładowania szablonu staff/dashboard.html: %v", err)
	}

	loansTmpl, err := template.ParseFiles("internal/templates/staff/loans.html")
	if err != nil {
		log.Printf("Błąd ładowania szablonu staff/loans.html: %v", err)
	}

	usersTmpl, err := template.ParseFiles("internal/templates/staff/users.html")
	if err != nil {
		log.Printf("Błąd ładowania szablonu staff/users.html: %v", err)
	}

	userEditTmpl, err := template.ParseFiles("internal/templates/staff/user_edit.html")
	if err != nil {
		log.Printf("Błąd ładowania szablonu staff/user_edit.html: %v", err)
	}

	reportsTmpl, err := template.ParseFiles("internal/templates/staff/reports.html")
	if err != nil {
		log.Printf("Błąd ładowania szablonu staff/reports.html: %v", err)
	}

	pendingPickupsTmpl, err := template.ParseFiles("internal/templates/staff/pending_pickups.html")
	if err != nil {
		log.Printf("Błąd ładowania szablonu staff/pending_pickups.html: %v", err)
	}

	return &StaffHandler{
		dashboardTemplate:      dashboardTmpl,
		loansTemplate:          loansTmpl,
		usersTemplate:          usersTmpl,
		userEditTemplate:       userEditTmpl,
		reportsTemplate:        reportsTmpl,
		pendingPickupsTemplate: pendingPickupsTmpl,
		fbClient:               fbClient,
	}
}

func (h *StaffHandler) ShowDashboard(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSessionFromContext(r.Context())
	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if h.dashboardTemplate == nil {
		http.Error(w, "Szablon dashboardu nie został załadowany", http.StatusInternalServerError)
		return
	}

	// Pobierz rzeczywiste statystyki z Firebase
	stats := map[string]interface{}{
		"totalBooks":     0,
		"activeLoans":    0,
		"pendingPickups": 0,
		"overdueLoans":   0,
		"totalUsers":     0,
	}

	if h.fbClient != nil {
		// Liczba książek
		if totalBooks, err := h.fbClient.CountTotalBooks(); err == nil {
			stats["totalBooks"] = totalBooks
		} else {
			log.Printf("Błąd pobierania liczby książek: %v", err)
		}

		// Liczba aktywnych wypożyczeń
		if activeLoans, err := h.fbClient.CountActiveLoans(); err == nil {
			stats["activeLoans"] = activeLoans
		} else {
			log.Printf("Błąd pobierania liczby aktywnych wypożyczeń: %v", err)
		}

		// Liczba wypożyczeń oczekujących na odbiór
		if allLoans, err := h.fbClient.ListLoans(); err == nil {
			pendingCount := 0
			for _, loan := range allLoans {
				if loan.Status == models.LoanStatusPendingPickup {
					pendingCount++
				}
			}
			stats["pendingPickups"] = pendingCount
		} else {
			log.Printf("Błąd pobierania liczby oczekujących odbiorów: %v", err)
		}

		// Liczba przeterminowanych wypożyczeń
		if overdueLoans, err := h.fbClient.CountOverdueLoans(); err == nil {
			stats["overdueLoans"] = overdueLoans
		} else {
			log.Printf("Błąd pobierania liczby przeterminowanych wypożyczeń: %v", err)
		}

		// Liczba użytkowników
		if totalUsers, err := h.fbClient.CountTotalUsers(); err == nil {
			stats["totalUsers"] = totalUsers
		} else {
			log.Printf("Błąd pobierania liczby użytkowników: %v", err)
		}
	}

	data := NewTemplateData(session)
	data["Stats"] = stats

	if err := h.dashboardTemplate.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *StaffHandler) ShowLoans(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSessionFromContext(r.Context())
	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if h.loansTemplate == nil {
		http.Error(w, "Szablon nie został załadowany", http.StatusInternalServerError)
		return
	}

	// Pobierz filtr z parametrów URL
	filter := r.URL.Query().Get("filter")
	if filter == "" {
		filter = "all"
	}

	var loans []*models.Loan
	var err error

	if h.fbClient != nil {
		switch filter {
		case "active":
			loans, err = h.fbClient.GetActiveLoans()
		case "overdue":
			loans, err = h.fbClient.GetOverdueLoans()
		case "returned":
			// Pobierz zwrócone wypożyczenia
			allLoans, e := h.fbClient.ListLoans()
			if e == nil {
				for _, loan := range allLoans {
					if loan.Status == models.LoanStatusReturned {
						loans = append(loans, loan)
					}
				}
			} else {
				err = e
			}
		default:
			loans, err = h.fbClient.ListLoans()
		}

		if err != nil {
			log.Printf("Błąd pobierania wypożyczeń: %v", err)
		}
	}

	// Przygotuj dane do wyświetlenia
	var loansDisplay []LoanDisplay
	if h.fbClient != nil {
		for _, loan := range loans {
			// Pobierz dane książki
			book, err := h.fbClient.GetBook(loan.BookID)
			if err != nil {
				log.Printf("Błąd pobierania książki %s: %v", loan.BookID, err)
				continue
			}

			// Pobierz dane użytkownika
			user, err := h.fbClient.GetUser(loan.UserID)
			if err != nil {
				log.Printf("Błąd pobierania użytkownika %s: %v", loan.UserID, err)
				continue
			}

			daysOverdue := 0
			if loan.IsOverdue() {
				daysOverdue = int(time.Since(loan.DueDate).Hours() / 24)
			}

			loansDisplay = append(loansDisplay, LoanDisplay{
				ID:          loan.ID,
				BookTitle:   book.Title,
				BookAuthor:  book.Author,
				UserName:    user.FullName(),
				UserEmail:   user.Email,
				LoanDate:    loan.LoanDate,
				DueDate:     loan.DueDate,
				ReturnDate:  loan.ReturnDate,
				Status:      string(loan.Status),
				FineAmount:  loan.FineAmount,
				IsOverdue:   loan.IsOverdue(),
				DaysOverdue: daysOverdue,
			})
		}
	}

	data := NewTemplateData(session)
	data["Loans"] = loansDisplay
	data["Filter"] = filter

	if err := h.loansTemplate.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *StaffHandler) ShowUsers(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSessionFromContext(r.Context())
	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if h.usersTemplate == nil {
		http.Error(w, "Szablon nie został załadowany", http.StatusInternalServerError)
		return
	}

	var users []*models.User
	if h.fbClient != nil {
		var err error
		users, err = h.fbClient.ListUsers()
		if err != nil {
			log.Printf("Błąd pobierania użytkowników: %v", err)
			data := NewTemplateData(session)
			data["Error"] = "Błąd pobierania użytkowników z bazy danych"
			h.usersTemplate.Execute(w, data)
			return
		}
	}

	data := NewTemplateData(session)
	data["Users"] = users

	if err := h.usersTemplate.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// SearchUsers wyszukuje użytkowników po imieniu, nazwisku lub emailu
func (h *StaffHandler) SearchUsers(w http.ResponseWriter, r *http.Request) {
	searchTerm := strings.ToLower(r.URL.Query().Get("search"))

	var users []*models.User
	if h.fbClient != nil {
		allUsers, err := h.fbClient.ListUsers()
		if err != nil {
			log.Printf("Błąd pobierania użytkowników: %v", err)
			w.Write([]byte("<p class='p-6 text-center text-red-600'>Błąd wyszukiwania</p>"))
			return
		}

		// Filtruj użytkowników
		if searchTerm != "" {
			for _, user := range allUsers {
				if strings.Contains(strings.ToLower(user.FirstName), searchTerm) ||
					strings.Contains(strings.ToLower(user.LastName), searchTerm) ||
					strings.Contains(strings.ToLower(user.Email), searchTerm) {
					users = append(users, user)
				}
			}
		} else {
			users = allUsers
		}
	}

	// Renderuj tylko tabelę
	h.renderUsersTable(w, users)
}

// ShowEditUser wyświetla formularz edycji użytkownika
func (h *StaffHandler) ShowEditUser(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSessionFromContext(r.Context())
	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if h.userEditTemplate == nil {
		http.Error(w, "Szablon nie został załadowany", http.StatusInternalServerError)
		return
	}

	userID := chi.URLParam(r, "id")
	if userID == "" {
		http.Error(w, "Brak ID użytkownika", http.StatusBadRequest)
		return
	}

	var user *models.User
	if h.fbClient != nil {
		var err error
		user, err = h.fbClient.GetUser(userID)
		if err != nil {
			log.Printf("Błąd pobierania użytkownika: %v", err)
			http.Error(w, "Nie znaleziono użytkownika", http.StatusNotFound)
			return
		}
	}

	data := NewTemplateData(session)
	data["EditUser"] = user

	if err := h.userEditTemplate.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// UpdateUser aktualizuje dane użytkownika
func (h *StaffHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	if userID == "" {
		http.Error(w, "Brak ID użytkownika", http.StatusBadRequest)
		return
	}

	// Parsuj formularz
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Błąd parsowania formularza", http.StatusBadRequest)
		return
	}

	maxLoans, err := strconv.Atoi(r.FormValue("max_loans"))
	if err != nil || maxLoans < 1 {
		http.Error(w, "Nieprawidłowa liczba maksymalnych wypożyczeń", http.StatusBadRequest)
		return
	}

	isActive := r.FormValue("is_active") == "true"

	if h.fbClient != nil {
		// Pobierz aktualnego użytkownika
		user, err := h.fbClient.GetUser(userID)
		if err != nil {
			log.Printf("Błąd pobierania użytkownika: %v", err)
			http.Error(w, "Nie znaleziono użytkownika", http.StatusNotFound)
			return
		}

		// Zaktualizuj tylko edytowalne pola
		user.MaxLoans = maxLoans
		user.IsActive = isActive
		user.UpdatedAt = time.Now()

		// Zapisz zmiany
		if err := h.fbClient.UpdateUser(userID, user); err != nil {
			log.Printf("Błąd aktualizacji użytkownika: %v", err)
			http.Error(w, "Błąd zapisywania zmian", http.StatusInternalServerError)
			return
		}
	}

	// Przekieruj z powrotem do listy użytkowników
	http.Redirect(w, r, "/staff/users", http.StatusSeeOther)
}

// ReturnLoan obsługuje zwrot książki
func (h *StaffHandler) ReturnLoan(w http.ResponseWriter, r *http.Request) {
	loanID := chi.URLParam(r, "id")
	if loanID == "" {
		http.Error(w, "Brak ID wypożyczenia", http.StatusBadRequest)
		return
	}

	if h.fbClient != nil {
		if err := h.fbClient.ReturnLoan(loanID); err != nil {
			log.Printf("Błąd zwrotu książki: %v", err)
			http.Error(w, "Błąd zwrotu książki", http.StatusInternalServerError)
			return
		}
	}

	// Zwróć pustą odpowiedź (wiersz zostanie usunięty przez htmx)
	w.WriteHeader(http.StatusOK)
}

// renderUsersTable renderuje tylko tabelę użytkowników (dla htmx)
func (h *StaffHandler) renderUsersTable(w http.ResponseWriter, users []*models.User) {
	if len(users) == 0 {
		w.Write([]byte("<p class='p-6 text-center text-gray-500'>Nie znaleziono użytkowników.</p>"))
		return
	}

	// Generuj HTML tabeli
	html := `<table class="min-w-full divide-y divide-gray-200">
		<thead class="bg-gray-50">
			<tr>
				<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Użytkownik</th>
				<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Email</th>
				<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Rola</th>
				<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Wypożyczenia</th>
				<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Max wypożyczeń</th>
				<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Status</th>
				<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Akcje</th>
			</tr>
		</thead>
		<tbody class="bg-white divide-y divide-gray-200">`

	for _, user := range users {
		roleClass := "bg-blue-100 text-blue-800"
		roleText := "Czytelnik"
		if user.Role == models.RoleAdmin {
			roleClass = "bg-purple-100 text-purple-800"
			roleText = "Administrator"
		}

		statusClass := "bg-green-100 text-green-800"
		statusText := "Aktywny"
		if !user.IsActive {
			statusClass = "bg-red-100 text-red-800"
			statusText = "Nieaktywny"
		}

		phone := ""
		if user.Phone != "" {
			phone = `<div class="text-sm text-gray-500">` + user.Phone + `</div>`
		}

		html += `<tr class="hover:bg-gray-50">
			<td class="px-6 py-4 whitespace-nowrap">
				<div class="text-sm font-medium text-gray-900">` + user.FirstName + ` ` + user.LastName + `</div>
				` + phone + `
			</td>
			<td class="px-6 py-4 whitespace-nowrap">
				<div class="text-sm text-gray-900">` + user.Email + `</div>
			</td>
			<td class="px-6 py-4 whitespace-nowrap">
				<span class="px-2 inline-flex text-xs leading-5 font-semibold rounded-full ` + roleClass + `">
					` + roleText + `
				</span>
			</td>
			<td class="px-6 py-4 whitespace-nowrap">
				<div class="text-sm text-gray-900">
					` + strconv.Itoa(user.CurrentLoans) + `
				</div>
			</td>
			<td class="px-6 py-4 whitespace-nowrap">
				<div class="text-sm text-gray-900">` + strconv.Itoa(user.MaxLoans) + `</div>
			</td>
			<td class="px-6 py-4 whitespace-nowrap">
				<span class="px-2 inline-flex text-xs leading-5 font-semibold rounded-full ` + statusClass + `">
					` + statusText + `
				</span>
			</td>
			<td class="px-6 py-4 whitespace-nowrap text-sm font-medium">
				<a href="/staff/users/` + user.ID + `/edit" class="text-blue-600 hover:text-blue-900 mr-4">Edytuj</a>
			</td>
		</tr>`
	}

	html += `</tbody></table>`
	w.Write([]byte(html))
}

func (h *StaffHandler) ShowReports(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSessionFromContext(r.Context())
	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if h.reportsTemplate == nil {
		http.Error(w, "Szablon nie został załadowany", http.StatusInternalServerError)
		return
	}

	data := NewTemplateData(session)
	if err := h.reportsTemplate.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// ShowPendingPickups wyświetla listę oczekujących odbiorów
func (h *StaffHandler) ShowPendingPickups(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSessionFromContext(r.Context())
	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if h.pendingPickupsTemplate == nil {
		http.Error(w, "Szablon nie został załadowany", http.StatusInternalServerError)
		return
	}

	// Pobierz wszystkie wypożyczenia oczekujące na odbiór
	allLoans, err := h.fbClient.ListLoans()
	if err != nil {
		log.Printf("Błąd pobierania wypożyczeń: %v", err)
		http.Error(w, "Błąd pobierania danych", http.StatusInternalServerError)
		return
	}

	var pendingPickups []*models.Loan
	for _, loan := range allLoans {
		if loan.Status == models.LoanStatusPendingPickup {
			pendingPickups = append(pendingPickups, loan)
		}
	}

	data := map[string]interface{}{
		"User":           session.User,
		"PendingPickups": pendingPickups,
		"Success":        r.URL.Query().Get("success"),
		"Error":          r.URL.Query().Get("error"),
	}

	if err := h.pendingPickupsTemplate.Execute(w, data); err != nil {
		log.Printf("Błąd renderowania szablonu: %v", err)
		http.Error(w, "Błąd renderowania strony", http.StatusInternalServerError)
		return
	}
}

// ConfirmPickup potwierdza odbiór książki
func (h *StaffHandler) ConfirmPickup(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSessionFromContext(r.Context())
	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/staff/pending-pickups?error="+url.QueryEscape("Błąd przetwarzania formularza"), http.StatusSeeOther)
		return
	}

	pickupCode := strings.ToUpper(strings.TrimSpace(r.FormValue("pickup_code")))
	if pickupCode == "" {
		http.Redirect(w, r, "/staff/pending-pickups?error="+url.QueryEscape("Kod odbioru nie może być pusty"), http.StatusSeeOther)
		return
	}

	// Potwierdź odbiór
	if err := h.fbClient.ConfirmPickup(pickupCode); err != nil {
		log.Printf("Błąd potwierdzania odbioru: %v", err)
		http.Redirect(w, r, "/staff/pending-pickups?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}

	log.Printf("Pracownik %s potwierdził odbiór z kodem %s", session.User.Email, pickupCode)
	http.Redirect(w, r, "/staff/pending-pickups?success="+url.QueryEscape("Odbiór potwierdzony pomyślnie"), http.StatusSeeOther)
}
