package handlers

import (
	"html/template"
	"log"
	"net/http"
	"time"

	"library-management-system/internal/firebase"
	"library-management-system/internal/middleware"
	"library-management-system/internal/models"
)

type UserHandler struct {
	dashboardTemplate    *template.Template
	feesTemplate         *template.Template
	historyTemplate      *template.Template
	reservationsTemplate *template.Template
	fbClient             *firebase.Client
}

type LoanView struct {
	BookTitle  string
	BookAuthor string
	LoanDate   time.Time
	DueDate    time.Time
	Status     string
	PickupCode string
	IsOverdue  bool
}

type FeeView struct {
	BookTitle  string
	BookAuthor string
	Reason     string
	Date       time.Time
	Amount     float64
}

type HistoryView struct {
	BookTitle  string
	BookAuthor string
	LoanDate   time.Time
	ReturnDate *time.Time
	WasOverdue bool
}

type ReservationView struct {
	ID              string
	BookTitle       string
	BookAuthor      string
	ReservationDate time.Time
	ExpiryDate      time.Time
	Status          string
	QueuePosition   int
}

func NewUserHandler(fbClient *firebase.Client) *UserHandler {
	dashboardTmpl, err := template.ParseFiles("internal/templates/user/dashboard.html")
	if err != nil {
		log.Printf("Błąd ładowania szablonu user/dashboard.html: %v", err)
	}

	historyTmpl, err := template.ParseFiles("internal/templates/user/history.html")
	if err != nil {
		log.Printf("Błąd ładowania szablonu user/history.html: %v", err)
	}

	reservationsTmpl, err := template.ParseFiles("internal/templates/user/reservations.html")
	if err != nil {
		log.Printf("Błąd ładowania szablonu user/reservations.html: %v", err)
	}

	return &UserHandler{
		dashboardTemplate:    dashboardTmpl,
		historyTemplate:      historyTmpl,
		reservationsTemplate: reservationsTmpl,
		fbClient:             fbClient,
	}
}

func (h *UserHandler) ShowDashboard(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSessionFromContext(r.Context())
	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if h.dashboardTemplate == nil {
		http.Error(w, "Szablon dashboardu nie został załadowany", http.StatusInternalServerError)
		return
	}

	// Pobierz aktywne wypożyczenia użytkownika
	var activeLoans []LoanView
	if h.fbClient != nil {
		loans, err := h.fbClient.GetUserActiveLoans(session.UserID)
		if err != nil {
			log.Printf("Błąd pobierania wypożyczeń: %v", err)
		} else {
			for _, loan := range loans {
				// Pobierz informacje o książce
				book, err := h.fbClient.GetBook(loan.BookID)
				if err != nil {
					log.Printf("Błąd pobierania książki %s: %v", loan.BookID, err)
					continue
				}

				activeLoans = append(activeLoans, LoanView{
					BookTitle:  book.Title,
					BookAuthor: book.Author,
					LoanDate:   loan.LoanDate,
					DueDate:    loan.DueDate,
					Status:     string(loan.Status),
					PickupCode: loan.PickupCode,
					IsOverdue:  loan.IsOverdue(),
				})
			}
		}
	}

	// Pobierz aktywne rezerwacje użytkownika
	activeReservationsCount := 0
	if h.fbClient != nil {
		reservations, err := h.fbClient.GetUserActiveReservations(session.UserID)
		if err != nil {
			log.Printf("Błąd pobierania rezerwacji: %v", err)
		} else {
			activeReservationsCount = len(reservations)
		}
	}

	stats := map[string]interface{}{
		"currentLoans":       len(activeLoans),
		"maxLoans":           5,
		"totalFines":         0.0,
		"activeReservations": activeReservationsCount,
	}

	data := NewTemplateData(session)
	data["ActiveLoans"] = activeLoans
	data["Stats"] = stats

	if err := h.dashboardTemplate.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *UserHandler) ShowFees(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSessionFromContext(r.Context())
	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if h.feesTemplate == nil {
		http.Error(w, "Szablon nie został załadowany", http.StatusInternalServerError)
		return
	}

	// Placeholder - przykładowe opłaty
	fees := []FeeView{
		{
			BookTitle:  "Przykładowa książka 1",
			BookAuthor: "Jan Kowalski",
			Reason:     "Przetrzymanie (7 dni)",
			Date:       time.Now().AddDate(0, 0, -7),
			Amount:     14.00,
		},
	}

	totalFees := 0.0
	for _, fee := range fees {
		totalFees += fee.Amount
	}

	data := NewTemplateData(session)
	data["Fees"] = fees
	data["TotalFees"] = totalFees

	if err := h.feesTemplate.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *UserHandler) ShowHistory(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSessionFromContext(r.Context())
	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if h.historyTemplate == nil {
		http.Error(w, "Szablon nie został załadowany", http.StatusInternalServerError)
		return
	}

	// Pobierz historię wypożyczeń użytkownika
	var history []HistoryView
	if h.fbClient != nil {
		loans, err := h.fbClient.GetUserLoanHistory(session.UserID)
		if err != nil {
			log.Printf("Błąd pobierania historii: %v", err)
		} else {
			for _, loan := range loans {
				// Pobierz informacje o książce
				book, err := h.fbClient.GetBook(loan.BookID)
				if err != nil {
					log.Printf("Błąd pobierania książki %s: %v", loan.BookID, err)
					continue
				}

				history = append(history, HistoryView{
					BookTitle:  book.Title,
					BookAuthor: book.Author,
					LoanDate:   loan.LoanDate,
					ReturnDate: loan.ReturnDate,
					WasOverdue: loan.IsOverdue(),
				})
			}
		}
	}

	data := NewTemplateData(session)
	data["History"] = history

	if err := h.historyTemplate.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *UserHandler) ShowReservations(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSessionFromContext(r.Context())
	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if h.reservationsTemplate == nil {
		http.Error(w, "Szablon nie został załadowany", http.StatusInternalServerError)
		return
	}

	// Pobierz rezerwacje użytkownika
	var reservations []ReservationView
	if h.fbClient != nil {
		res, err := h.fbClient.GetUserActiveReservations(session.UserID)
		if err != nil {
			log.Printf("Błąd pobierania rezerwacji: %v", err)
		} else {
			for _, reservation := range res {
				// Pobierz informacje o książce
				book, err := h.fbClient.GetBook(reservation.BookID)
				if err != nil {
					log.Printf("Błąd pobierania książki %s: %v", reservation.BookID, err)
					continue
				}

				// Oblicz pozycję w kolejce (tylko dla pending)
				queuePos := 0
				if reservation.Status == models.ReservationStatusPending {
					allReservations, _ := h.fbClient.GetBookReservations(reservation.BookID)
					for i, r := range allReservations {
						if r.Status == models.ReservationStatusPending && r.ID == reservation.ID {
							queuePos = i + 1
							break
						}
					}
				}

				reservations = append(reservations, ReservationView{
					ID:              reservation.ID,
					BookTitle:       book.Title,
					BookAuthor:      book.Author,
					ReservationDate: reservation.CreatedAt,
					ExpiryDate:      reservation.ExpiryDate,
					Status:          string(reservation.Status),
					QueuePosition:   queuePos,
				})
			}
		}
	}

	data := NewTemplateData(session)
	data["Reservations"] = reservations

	if err := h.reservationsTemplate.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// BorrowFromReservation wypożycza książkę z rezerwacji
func (h *UserHandler) BorrowFromReservation(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSessionFromContext(r.Context())
	if session == nil {
		http.Error(w, "Nie jesteś zalogowany", http.StatusUnauthorized)
		return
	}

	// Pobierz ID rezerwacji z URL
	reservationID := r.PathValue("id")
	if reservationID == "" {
		http.Error(w, "Brak ID rezerwacji", http.StatusBadRequest)
		return
	}

	if h.fbClient == nil {
		http.Error(w, "Błąd serwera", http.StatusInternalServerError)
		return
	}

	// Pobierz rezerwację
	reservation, err := h.fbClient.GetReservation(reservationID)
	if err != nil {
		log.Printf("Błąd pobierania rezerwacji: %v", err)
		http.Error(w, "Nie znaleziono rezerwacji", http.StatusNotFound)
		return
	}

	// Sprawdź czy rezerwacja należy do użytkownika
	if reservation.UserID != session.UserID {
		http.Error(w, "To nie Twoja rezerwacja", http.StatusForbidden)
		return
	}

	// Sprawdź czy rezerwacja jest gotowa do wypożyczenia
	if !reservation.CanBeCompleted() {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<div class="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded">
			Rezerwacja nie jest gotowa do wypożyczenia lub wygasła.
		</div>`))
		return
	}

	// Pobierz użytkownika
	user, err := h.fbClient.GetUser(session.UserID)
	if err != nil {
		log.Printf("Błąd pobierania użytkownika: %v", err)
		http.Error(w, "Błąd pobierania danych użytkownika", http.StatusInternalServerError)
		return
	}

	// Sprawdź czy użytkownik może wypożyczyć (nie przekroczył limitu)
	if !user.CanBorrow() {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<div class="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded">
			Osiągnięto limit wypożyczeń lub konto jest nieaktywne.
		</div>`))
		return
	}

	// Utwórz wypożyczenie (CreateLoan automatycznie wygeneruje kod odbioru i ustawi status pending_pickup)
	loan := &models.Loan{
		BookID:    reservation.BookID,
		UserID:    session.UserID,
		BookTitle: reservation.BookTitle,                // Denormalizacja
		UserName:  user.FirstName + " " + user.LastName, // Denormalizacja
	}

	if err := h.fbClient.CreateLoan(loan); err != nil {
		log.Printf("Błąd tworzenia wypożyczenia: %v", err)
		http.Error(w, "Nie udało się wypożyczyć książki", http.StatusInternalServerError)
		return
	}

	// Zwiększ licznik wypożyczeń użytkownika
	user.CurrentLoans++
	user.UpdatedAt = time.Now()
	if err := h.fbClient.UpdateUser(session.UserID, user); err != nil {
		log.Printf("Błąd aktualizacji użytkownika: %v", err)
	}

	// Oznacz rezerwację jako zrealizowaną
	if err := h.fbClient.CompleteReservation(reservationID); err != nil {
		log.Printf("Błąd oznaczania rezerwacji jako zrealizowanej: %v", err)
	}

	// Zwróć komunikat sukcesu z kodem odbioru (htmx zastąpi element)
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`<div class="bg-green-100 border border-green-400 text-green-700 px-4 py-3 rounded">
		✓ Rezerwacja przekształcona w zamówienie!<br>
		<span class="font-bold">Kod odbioru: ` + loan.PickupCode + `</span><br>
		Podaj ten kod w bibliotece, aby odebrać książkę.
		<a href="/user" class="underline ml-2">Zobacz moje wypożyczenia</a>
	</div>`))
}

// CancelReservation anuluje rezerwację
func (h *UserHandler) CancelReservation(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSessionFromContext(r.Context())
	if session == nil {
		http.Error(w, "Nie jesteś zalogowany", http.StatusUnauthorized)
		return
	}

	// Pobierz ID rezerwacji z URL
	reservationID := r.PathValue("id")
	if reservationID == "" {
		http.Error(w, "Brak ID rezerwacji", http.StatusBadRequest)
		return
	}

	if h.fbClient == nil {
		http.Error(w, "Błąd serwera", http.StatusInternalServerError)
		return
	}

	// Pobierz rezerwację
	reservation, err := h.fbClient.GetReservation(reservationID)
	if err != nil {
		log.Printf("Błąd pobierania rezerwacji: %v", err)
		http.Error(w, "Nie znaleziono rezerwacji", http.StatusNotFound)
		return
	}

	// Sprawdź czy rezerwacja należy do użytkownika
	if reservation.UserID != session.UserID {
		http.Error(w, "To nie Twoja rezerwacja", http.StatusForbidden)
		return
	}

	bookID := reservation.BookID

	// Anuluj rezerwację
	if err := h.fbClient.CancelReservation(reservationID); err != nil {
		log.Printf("Błąd anulowania rezerwacji: %v", err)
		http.Error(w, "Nie udało się anulować rezerwacji", http.StatusInternalServerError)
		return
	}

	// Sprawdź czy są kolejne rezerwacje w kolejce
	nextReservation, err := h.fbClient.GetNextReservation(bookID)
	if err != nil {
		log.Printf("Błąd sprawdzania kolejki rezerwacji: %v", err)
	}

	if nextReservation != nil {
		// Jest kolejna rezerwacja - aktywuj ją
		if err := h.fbClient.MarkReservationReady(nextReservation.ID); err != nil {
			log.Printf("Błąd aktywacji kolejnej rezerwacji: %v", err)
		}
	} else {
		// Brak kolejnych rezerwacji - zwróć książkę do katalogu (zwiększ dostępność)
		book, err := h.fbClient.GetBook(bookID)
		if err != nil {
			log.Printf("Błąd pobierania książki: %v", err)
		} else {
			book.AvailableCopies++
			book.UpdatedAt = time.Now()
			if err := h.fbClient.UpdateBook(bookID, book); err != nil {
				log.Printf("Błąd aktualizacji dostępności książki: %v", err)
			}
		}
	}

	// Zwróć komunikat sukcesu (htmx usunie element)
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`<div class="bg-blue-100 border border-blue-400 text-blue-700 px-4 py-3 rounded">
		Rezerwacja została anulowana.
	</div>`))
}
