package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type Subj struct {
	Items int `json:"items"`
	Categories int `json:"categories"`
	Price   int    `json:"price"`
}


func mainPage(res http.ResponseWriter, req *http.Request) {
	//
	body := fmt.Sprintf("Method: %s\r\n", req.Method)
	body += "Header =====================================\r\n"
	for k, v := range req.Header {
		body += fmt.Sprintf("%s: %v\r\n", k, v)
	}
	body += "Query =====================================\r\n"
	if err := req.ParseForm(); err != nil {
		res.Write([]byte(err.Error()))
		return
	}
	body += "File main.go ================================"

	for k, v := range req.Form {
		body += fmt.Sprintf("%s: %v\r\n", k, v)
	}

	res.Write([]byte(body))
}

func apiPage(res http.ResponseWriter, req *http.Request) {
	res.Write([]byte("This is /api"))
}

func JSONHandler(w http.ResponseWriter, req *http.Request) {
	subj := Subj{"Milk", 150}

	resp, err := json.Marshal(subj)
		if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.Header().Set("content-type", "application-json")
	w.WriteHeader(http.StatusOK)
	w.Write(resp)
}

func main() {
	// var h MyHandler
	mux := http.NewServeMux()

	mux.HandleFunc(`/api/`, apiPage)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request)) {
		http.ServeFile(w, r, "./main.go")
	}

	mux.HandleFunc("/json", JSONHandler)

	err := http.ListenAndServe(`:8080`, mux)
	if err != nil {
		panic(err)
	}
}
