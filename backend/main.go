package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"

	_ "modernc.org/sqlite"

	"kuchta.dev/codename-maps-edit-service/api"
	"kuchta.dev/codename-maps-edit-service/data"
)

//go:generate go run github.com/sqlc-dev/sqlc/cmd/sqlc@latest generate
//go:generate go run github.com/ogen-go/ogen/cmd/ogen@latest --target api --clean ./openapi.yml

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func main() {
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "edits.db"
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if err := migrate(db); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	q := data.New(db)
	handler := NewHandler(q)

	srv, err := api.NewServer(handler)
	if err != nil {
		log.Fatal(err)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = ":8000"
	}

	// Wrap server with CORS middleware
	corsHandler := corsMiddleware(srv)

	log.Printf("Edit Service started on %s\n", port)
	if err := http.ListenAndServe(port, corsHandler); err != nil {
		log.Fatal(err)
	}
}
