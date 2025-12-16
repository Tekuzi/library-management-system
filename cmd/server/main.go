package main

import (
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"

	"library-management-system/internal/firebase"
	"library-management-system/internal/handlers"
	authmw "library-management-system/internal/middleware"
	"library-management-system/internal/models"
	"library-management-system/internal/session"
)

func main() {
	// Wczytaj zmienne środowiskowe z pliku .env
	if err := godotenv.Load(); err != nil {
		log.Println("Brak pliku .env - używam zmiennych systemowych")
	}

	// Pobierz port z zmiennych środowiskowych lub użyj domyślnego
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Inicjalizacja Firebase (opcjonalne - może nie działać bez credentials)
	fbClient, err := firebase.InitFirebase()
	if err != nil {
		log.Printf("UWAGA: Firebase nie został zainicjalizowany: %v", err)
		log.Println("Aplikacja będzie działać w trybie bez bazy danych")
	} else {
		log.Println("Firebase zainicjalizowany pomyślnie")
	}

	// Inicjalizacja systemu sesji
	session.Init()
	log.Println("System sesji zainicjalizowany")

	// Inicjalizacja routera Chi
	r := chi.NewRouter()

	// Middleware do logowania requestów
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Middleware sesji - dodaj sesję do kontekstu każdego żądania
	r.Use(authmw.SessionMiddleware)

	// Serwowanie plików statycznych (CSS, JS)
	fileServer := http.FileServer(http.Dir("./static"))
	r.Handle("/static/*", http.StripPrefix("/static/", fileServer))

	// Inicjalizacja handlerów
	indexHandler := handlers.NewIndexHandler()
	booksHandler := handlers.NewBooksHandler(fbClient)
	authHandler := handlers.NewAuthHandler()
	staffHandler := handlers.NewStaffHandler(fbClient)
	userHandler := handlers.NewUserHandler(fbClient)
	catalogHandler := handlers.NewCatalogHandler()

	// Strona główna - publiczna
	r.Get("/", indexHandler.ServeHTTP)

	// Routy dla autoryzacji
	r.Get("/login", authHandler.ShowLoginPage)
	r.Post("/login", authHandler.HandleLogin)
	r.Get("/register", authHandler.ShowRegisterPage)
	r.Post("/register", authHandler.HandleRegister)
	r.Post("/logout", authHandler.HandleLogout)

	// Grupy routów dla książek - publiczny katalog
	r.Route("/books", func(r chi.Router) {
		r.Get("/", booksHandler.ListBooksHandler)
		r.Get("/search", booksHandler.SearchBooksHandler)
		r.Get("/{id}", booksHandler.ShowBookHandler)

		// Wypożyczanie i rezerwacje (wymagają logowania)
		r.Group(func(r chi.Router) {
			r.Use(authmw.RequireAuth)
			r.Post("/{id}/borrow", booksHandler.BorrowBook)
			r.Post("/{id}/reserve", booksHandler.ReserveBook)
		})
	})

	// Panel użytkownika (dla zalogowanych czytelników)
	r.Route("/user", func(r chi.Router) {
		r.Use(authmw.RequireAuth)
		r.Get("/", userHandler.ShowDashboard)
		r.Get("/history", userHandler.ShowHistory)
		r.Get("/reservations", userHandler.ShowReservations)
		r.Post("/reservations/{id}/borrow", userHandler.BorrowFromReservation)
		r.Post("/reservations/{id}/cancel", userHandler.CancelReservation)
	})

	// Panel personelu (tylko dla adminów)
	r.Route("/staff", func(r chi.Router) {
		r.Use(authmw.RequireAuth)
		r.Use(authmw.RequireAuthRole(models.RoleAdmin))
		r.Get("/", staffHandler.ShowDashboard)

		// Zarządzanie katalogiem
		r.Get("/catalog", catalogHandler.ListBooks)
		r.Get("/catalog/search", catalogHandler.SearchBooks)
		r.Get("/catalog/new", catalogHandler.ShowNewBookForm)
		r.Post("/catalog", catalogHandler.CreateBook)
		r.Get("/catalog/{id}/edit", catalogHandler.ShowEditBookForm)
		r.Put("/catalog/{id}", catalogHandler.UpdateBook)
		r.Delete("/catalog/{id}", catalogHandler.DeleteBook)

		// Zarządzanie wypożyczeniami
		r.Get("/loans", staffHandler.ShowLoans)
		r.Post("/loans/{id}/return", staffHandler.ReturnLoan)

		// Potwierdzanie odbiorów
		r.Get("/pending-pickups", staffHandler.ShowPendingPickups)
		r.Post("/loans/confirm-pickup", staffHandler.ConfirmPickup)

		// Zarządzanie użytkownikami
		r.Get("/users", staffHandler.ShowUsers)
		r.Get("/users/search", staffHandler.SearchUsers)
		r.Get("/users/{id}/edit", staffHandler.ShowEditUser)
		r.Post("/users/{id}/update", staffHandler.UpdateUser)

		// Raporty
		r.Get("/reports", staffHandler.ShowReports)
	})

	// Start serwera
	log.Printf("Serwer uruchomiony na porcie %s", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("Nie można uruchomić serwera: %v", err)
	}
}
