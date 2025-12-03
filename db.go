package main

import (
	"database/sql"

	_ "github.com/lib/pq"
)

var DB *sql.DB

func InitDB() error {
	connStr := "postgres://validator:validator@postgresql:5432/project_sem_1?sslmode=disable"

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return err
	}

	if err := db.Ping(); err != nil {
		return err
	}

	DB = db
	return nil
}
