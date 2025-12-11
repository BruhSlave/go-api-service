package main

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

type ImportStats struct {
	TotalCount      int     `json:"total_count"`
	DuplicatesCount int     `json:"duplicates_count"`
	TotalItems      int     `json:"total_items"`
	TotalCategories int     `json:"total_categories"`
	TotalPrice      float64 `json:"total_price"`
}

type PriceItem struct {
	ID       int
	Name     string
	Category string
	Price    float64
	Date     time.Time
}

func InsertItem(item PriceItem) error {
	_, err := DB.Exec(`
			INSERT INTO items (name, category, price, create_date)
			VALUES ($1, $2, $3, $4)
		`,
		item.Name,
		item.Category,
		item.Price,
		item.Date,
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
		err := rows.Scan(&price.ID, &price.Name, &price.Category, &price.Price, &price.Date)
		if err != nil {
			return nil, err
		}
		items = append(items, price)
	}

	if err = rows.Err(); err != nil {
		return nil, err
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

	var (
		totalItems      int
		totalCount      int
		duplicatesCount int
		totalPrice      float64
	)

	categoriesMap := make(map[string]struct{})
	seenID := make(map[int]struct{})

	for _, f := range zipReader.File {
		if !strings.HasSuffix(f.Name, ".csv") {
			continue
		}

		csvFile, err := f.Open()
		if err != nil {
			http.Error(res, "Can't open csv file", http.StatusBadRequest)
			continue
		}

		reader := csv.NewReader(csvFile)
		records, err := reader.ReadAll()
		csvFile.Close()
		if err != nil {
			http.Error(res, "Fail to read csv file", http.StatusBadRequest)
			continue
		}

		for i, row := range records {
			if i == 0 {
				continue
			}

			if len(row) < 5 {
				continue
			}

			totalCount++

			id, err := strconv.Atoi(strings.TrimSpace(row[0]))
			if err != nil {
				fmt.Printf("Skip row %d: Invalid id %q: %v\n", i, row[0], err)
				continue
			}

			if _, ok := seenID[id]; ok {
				duplicatesCount++
			}
			seenID[id] = struct{}{}

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
				Name:     name,
				Category: category,
				Price:    price,
				Date:     createDate,
			}

			_, err = DB.Exec(`
    			INSERT INTO items (name, category, price, create_date)
    			VALUES ($1, $2, $3, $4)
			`, item.Name, item.Category, item.Price, item.Date)
			if err != nil {
				fmt.Printf("Failed to INSERT in BD in row %d and err: %v\n", i, err)
			}

			totalItems++
			categoriesMap[category] = struct{}{}
			totalPrice += price

		}
	}
	stats := ImportStats{
		TotalCount:      totalCount,
		DuplicatesCount: duplicatesCount,
		TotalItems:      totalItems,
		TotalCategories: len(categoriesMap),
		TotalPrice:      totalPrice,
	}

	res.Header().Set("Content-Type", "application/json")
	json.NewEncoder(res).Encode(stats)
}

func handleGetPrices(res http.ResponseWriter, req *http.Request) {
	startStr := req.URL.Query().Get("start")
	endStr := req.URL.Query().Get("end")
	minStr := req.URL.Query().Get("min")
	maxStr := req.URL.Query().Get("max")

	var filters []string
	var args []interface{}
	argID := 1

	if startStr != "" {
		startDate, err := time.Parse("2006-01-02", startStr)
		if err != nil {
			http.Error(res, "Invalid start date", http.StatusBadRequest)
			return
		}
		filters = append(filters, fmt.Sprintf("create_date >= $%d", argID))
		args = append(args, startDate)
		argID++
	}
	if endStr != "" {
		endDate, err := time.Parse("2006-01-02", endStr)
		if err != nil {
			http.Error(res, "Invalid start date", http.StatusBadRequest)
			return
		}
		filters = append(filters, fmt.Sprintf("create_date <= $%d", argID))
		args = append(args, endDate)
		argID++
	}
	if minStr != "" {
		minPrice, err := strconv.ParseFloat(minStr, 64)
		if err != nil {
			http.Error(res, "Invalid min price", http.StatusBadRequest)
			return
		}
		filters = append(filters, fmt.Sprintf("price >= $%d", argID))
		args = append(args, minPrice)
		argID++
	}
	if maxStr != "" {
		maxPrice, err := strconv.ParseFloat(maxStr, 64)
		if err != nil {
			http.Error(res, "Invalid max price", http.StatusBadRequest)
			return
		}
		filters = append(filters, fmt.Sprintf("price <= $%d", argID))
		args = append(args, maxPrice)
		argID++
	}

	query := "SELECT id, name, category, price, create_date FROM items"
	if len(filters) > 0 {
		query += " WHERE " + strings.Join(filters, " AND ")
	}
	query += " ORDER BY id"

	rows, err := DB.Query(query, args...)
	if err != nil {
		http.Error(res, "Failed to query database", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	items := []PriceItem{}
	for rows.Next() {
		var item PriceItem
		if err := rows.Scan(&item.ID, &item.Name, &item.Category, &item.Price, &item.Date); err != nil {
			http.Error(res, "Failed to scan row", http.StatusInternalServerError)
			return
		}
		if item.Price <= 0 || item.Name == "" || item.Category == "" || item.Date.IsZero() {
			continue
		}

		items = append(items, item)
	}

	if err = rows.Err(); err != nil {
		http.Error(res, "Row error", http.StatusMethodNotAllowed)
	}

	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)
	fileWriter, _ := zipWriter.Create("data.csv")
	csvWriter := csv.NewWriter(fileWriter)

	csvWriter.Write([]string{"id", "name", "category", "price", "create_date"})

	for _, item := range items {
		csvWriter.Write([]string{
			strconv.Itoa(item.ID),
			item.Name,
			item.Category,
			fmt.Sprintf("%.2f", item.Price),
			item.Date.Format("2006-01-02"),
		})
	}

	csvWriter.Flush()
	if err := csvWriter.Error(); err != nil {
		http.Error(res, "Failed to write CSV", http.StatusInternalServerError)
		return
	}
	zipWriter.Close()

	res.Header().Set("Content-Type", "application/zip")
	res.Write(buf.Bytes())
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
