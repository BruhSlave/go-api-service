package main

import (
	"fmt"
	"net/http"
	"time"

	_ "github.com/lib/pq"
)

type ImportStats struct {
	Items      int `json:"items"`
	Categories int `json:"categories"`
	Price      int `json:"price"`
}

type PriceItem struct {
	ID         int
	Name       string
	Category   string
	Price      float64
	CreateDate time.Time
}

func InsertItem(item PriceItem) error {
	_, err := DB.Exec(`
			INSERT INTO items (id, name, category, price, create_date)
			VALUES ($1, $2, $3, $4, $5)
		`,
		item.ID,
		item.Name,
		item.Category,
		item.Price,
		item.CreateDate,
	)
	return err
}

func GetItems() ([]PriceItem, error) {
	rows, err := DB.Query(`
			SELECT id, name, category, price, create_date
			FROM items
			ORDER BY id
		`)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	items := []PriceItem{}

	for rows.Next() {
		var price PriceItem
		err := rows.Scan(&price.ID, &price.Name, &price.Category, &price.Price, &price.CreateDate)
		if err != nil {
			return nil, err
		}
		items = append(items, price)
	}

	return items, nil
}

func main() {
	if err := InitDB(); err != nil {
		panic(err)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/api/v0/prices", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Endpoint works"))
	})

	fmt.Println("Server started at :8080")
	err := http.ListenAndServe(":8080", mux)
	if err != nil {
		panic(err)
	}
}
