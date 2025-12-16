package main

import (
	"log"
	"time"

	"library-management-system/internal/firebase"
	"library-management-system/internal/models"
)

func main() {
	// Inicjalizacja Firebase
	fbClient, err := firebase.InitFirebase()
	if err != nil {
		log.Fatalf("Błąd inicjalizacji Firebase: %v", err)
	}

	log.Println("Dodawanie przykładowych książek do bazy danych...")

	books := []models.Book{
		{
			ISBN:            "978-83-8032-464-8",
			Title:           "Wiedźmin: Ostatnie życzenie",
			Author:          "Andrzej Sapkowski",
			Publisher:       "SuperNowa",
			PublicationYear: 1993,
			Category:        "Fantasy",
			Description:     "Zbiór opowiadań o wiedźminie Geralcie z Rivii, łowcy potworów. Pierwsza książka w słynnej serii fantasy.",
			TotalCopies:     3,
			AvailableCopies: 3,
			ShelfLocation:   "A-12",
		},
		{
			ISBN:            "978-83-240-1455-5",
			Title:           "Zbrodnia i kara",
			Author:          "Fiodor Dostojewski",
			Publisher:       "Świat Książki",
			PublicationYear: 1866,
			Category:        "Klasyka",
			Description:     "Psychologiczna powieść o studencie Rodionze Raskolnikowie, który popełnia morderstwo i zmaga się z konsekwencjami swojego czynu.",
			TotalCopies:     2,
			AvailableCopies: 2,
			ShelfLocation:   "B-05",
		},
		{
			ISBN:            "978-83-7686-320-4",
			Title:           "Sapiens: Od zwierząt do bogów",
			Author:          "Yuval Noah Harari",
			Publisher:       "Wydawnictwo Literackie",
			PublicationYear: 2011,
			Category:        "Popularnonaukowa",
			Description:     "Fascynująca historia ludzkości od czasów prehistorycznych po współczesność, analizująca rozwój Homo sapiens.",
			TotalCopies:     4,
			AvailableCopies: 4,
			ShelfLocation:   "C-18",
		},
		{
			ISBN:            "978-83-7885-585-8",
			Title:           "Rok 1984",
			Author:          "George Orwell",
			Publisher:       "Muza",
			PublicationYear: 1949,
			Category:        "Science Fiction",
			Description:     "Dystopijny obraz totalitarnego społeczeństwa przyszłości, w którym rząd inwigiluje każdy aspekt życia obywateli.",
			TotalCopies:     2,
			AvailableCopies: 2,
			ShelfLocation:   "D-07",
		},
		{
			ISBN:            "978-83-8100-234-1",
			Title:           "Atomowe nawyki",
			Author:          "James Clear",
			Publisher:       "Znak Literanova",
			PublicationYear: 2018,
			Category:        "Rozwój osobisty",
			Description:     "Praktyczny przewodnik po budowaniu dobrych nawyków i eliminowaniu złych. Bestseller self-help.",
			TotalCopies:     3,
			AvailableCopies: 3,
			ShelfLocation:   "E-22",
		},
		{
			ISBN:            "978-83-240-4532-0",
			Title:           "Harry Potter i Kamień Filozoficzny",
			Author:          "J.K. Rowling",
			Publisher:       "Media Rodzina",
			PublicationYear: 1997,
			Category:        "Fantasy",
			Description:     "Pierwsza część kultowej serii o młodym czarodzieju Harry'm Potterze i jego przygodach w Szkole Magii i Czarodziejstwa Hogwart.",
			TotalCopies:     5,
			AvailableCopies: 5,
			ShelfLocation:   "A-15",
		},
		{
			ISBN:            "978-83-7686-811-7",
			Title:           "Kod da Vinci",
			Author:          "Dan Brown",
			Publisher:       "Albatros",
			PublicationYear: 2003,
			Category:        "Thriller",
			Description:     "Thriller łączący sztukę, historię i symbologię religijną. Robert Langdon rozwiązuje zagadkę morderstwa w Luwrze.",
			TotalCopies:     2,
			AvailableCopies: 2,
			ShelfLocation:   "F-09",
		},
		{
			ISBN:            "978-83-7506-651-3",
			Title:           "Władca Pierścieni: Drużyna Pierścienia",
			Author:          "J.R.R. Tolkien",
			Publisher:       "Amber",
			PublicationYear: 1954,
			Category:        "Fantasy",
			Description:     "Pierwsza część epickiej trylogii fantasy o wyprawie Drużyny Pierścienia mającej na celu zniszczenie Pierścienia Mocy.",
			TotalCopies:     3,
			AvailableCopies: 3,
			ShelfLocation:   "A-20",
		},
		{
			ISBN:            "978-83-240-5896-2",
			Title:           "Mistrz i Małgorzata",
			Author:          "Michaił Bułhakow",
			Publisher:       "Świat Książki",
			PublicationYear: 1967,
			Category:        "Klasyka",
			Description:     "Satyryczna powieść łącząca realia sowieckiej Moskwy lat 30. z opowieścią o Piłacie Ponckim i Jezusie.",
			TotalCopies:     2,
			AvailableCopies: 2,
			ShelfLocation:   "B-14",
		},
		{
			ISBN:            "978-83-8100-567-0",
			Title:           "Thinking, Fast and Slow",
			Author:          "Daniel Kahneman",
			Publisher:       "Penguin Books",
			PublicationYear: 2011,
			Category:        "Psychologia",
			Description:     "Analiza dwóch systemów myślenia: szybkiego, intuicyjnego i wolnego, analitycznego. Praca laureata Nagrody Nobla.",
			TotalCopies:     2,
			AvailableCopies: 2,
			ShelfLocation:   "C-25",
		},
	}

	now := time.Now()
	successCount := 0

	for _, book := range books {
		book.CreatedAt = now
		book.UpdatedAt = now

		if err := fbClient.CreateBook(&book); err != nil {
			log.Printf("❌ Błąd dodawania książki '%s': %v", book.Title, err)
		} else {
			log.Printf("✓ Dodano: %s - %s", book.Title, book.Author)
			successCount++
		}
	}

	log.Printf("\n✅ Pomyślnie dodano %d/%d książek do bazy danych", successCount, len(books))
}
