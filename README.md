# System Zarządzania Biblioteką

Aplikacja webowa do zarządzania biblioteką zbudowana w Go + htmx.

## Stack Technologiczny

- **Backend**: Go (chi router)
- **Frontend**: htmx + Tailwind CSS + Alpine.js
- **Baza danych**: Firebase Firestore
- **Autentykacja**: Firebase Auth
- **Deployment**: Lokalnie (później Cloud Run)

## Wymagania

- Go 1.21 lub nowszy
- Konto Firebase z konfiguracją projektu
- Plik `serviceAccountKey.json` z danymi uwierzytelniającymi Firebase

## Instalacja

1. Sklonuj repozytorium
2. Skopiuj `.env.example` do `.env` i wypełnij dane Firebase
3. Pobierz zależności:
```bash
go mod download
```

## Konfiguracja Firebase

1. Utwórz projekt w [Firebase Console](https://console.firebase.google.com/)
2. Włącz Firebase Authentication i Firestore
3. Pobierz plik `serviceAccountKey.json` z ustawieniami projektu
4. Umieść plik w głównym katalogu projektu
5. Zaktualizuj plik `.env` z odpowiednimi danymi

## Uruchomienie

```bash
go run cmd/server/main.go
```

Aplikacja będzie dostępna pod adresem: `http://localhost:8080`

## Struktura Projektu

```
/library-management-system
├── cmd/
│   └── server/          # Punkt wejścia aplikacji
├── internal/
│   ├── models/          # Struktury danych (Book, User, Loan, Reservation)
│   ├── firebase/        # Klient Firebase (Auth + Firestore)
│   ├── handlers/        # HTTP handlers
│   ├── middleware/      # Middleware (auth, logging)
│   └── templates/       # Szablony HTML
├── static/
│   ├── css/             # Pliki CSS (Tailwind)
│   └── js/              # Pliki JavaScript
├── go.mod
└── README.md
```

## Role Użytkowników

- **Czytelnik**: Wyszukiwanie książek, wypożyczanie, rezerwacje
- **Bibliotekarz**: Pełny dostęp do systemu, zarządzanie użytkownikami, wypożyczeniami, zwrotami, katalogiem 

## Funkcjonalności

- [ ] Zarządzanie katalogiem książek
- [ ] Wyszukiwanie i filtrowanie
- [ ] Wypożyczenia i zwroty
- [ ] System rezerwacji
- [ ] Zarządzanie użytkownikami
- [ ] Raporty i statystyki

## Licencja

Praca inżynierska - wszystkie prawa zastrzeżone.
