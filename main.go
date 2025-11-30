package main

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

type ImportStats struct {
	TotalItems      int `json:"total_items"`
	TotalCategories int `json:"total_categories"`
	TotalPrice      int `json:"total_price"`
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

func PricesHandler(res http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodPost:
		handlePostPrices(res, req)
	case http.MethodGet:
		handleGetPrices(res, req)
	default:
		http.Error(res, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func handlePostPrices(res http.ResponseWriter, req *http.Request) {
	if req.URL.Query().Get("type") != "zip" {
		http.Error(res, "Only zip aviable", http.StatusBadRequest)
		return
	}

	file, _, err := req.FormFile("file")
	if err != nil {
		http.Error(res, "File not found", http.StatusBadRequest)
		return
	}
	defer file.Close()

	buf, err := io.ReadAll(file)
	if err != nil {
		http.Error(res, "Failed to read file", http.StatusBadRequest)
		return
	}

	zipReader, err := zip.NewReader(bytes.NewReader(buf), int64(len(buf)))
	if err != nil {
		http.Error(res, "Invalid zip", http.StatusBadRequest)
		return
	}

	for _, f := range zipReader.File {
		if !strings.HasSuffix(f.Name, ".csv") {
			continue
		}

		csvFile, err := f.Open()
		if err != nil {
			http.Error(res, "Can't open csv file", http.StatusBadRequest)
			return
		}

		reader := csv.NewReader(csvFile)
		records, err := reader.ReadAll()
		csvFile.Close()
		if err != nil {
			http.Error(res, "Fail to read csv file", http.StatusBadRequest)
			return
		}

		for i, row := range records {
			if i == 0 {
				continue
			}

			if len(row) < 5 {
				continue
			}

			id, err := strconv.Atoi(strings.TrimSpace(row[0]))
			if err != nil {
				fmt.Printf("Skip row %d: Invalid id %q: %v\n", i, row[0], err)
				continue
			}

			name := strings.TrimSpace(row[1])
			category := strings.TrimSpace(row[2])

			price, err := strconv.ParseFloat(strings.TrimSpace(row[3]), 64)
			if err != nil {
				fmt.Printf("Skip row %d: invalid price %q: %v\n", i, row[3], err)
				continue
			}

			createDate, err := time.Parse("2006-01-02", strings.TrimSpace(row[4]))
			if err != nil {
				fmt.Printf("Skip row %d: invalid date %q: %v\n", i, row[4], err)
				continue
			}

			item := PriceItem{
				ID:         id,
				Name:       name,
				Category:   category,
				Price:      price,
				CreateDate: createDate,
			}

			fmt.Printf("Parsed itmes: %+v\n", item)
		}
	}
}

func handleGetPrices(res http.ResponseWriter, req *http.Request) {
	res.Write([]byte("GET works"))
}

func main() {
	if err := InitDB(); err != nil {
		panic(err)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/api/v0/prices", PricesHandler)

	fmt.Println("Server started at :8080")
	err := http.ListenAndServe(":8080", mux)
	if err != nil {
		panic(err)
	}
}
