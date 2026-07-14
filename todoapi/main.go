package main

import (
	"log"
	"net/http"

	"todoapi/internal/todo"
)

func main() {
	store := todo.NewStore()
	handler := todo.NewHandler(store)

	mux := http.NewServeMux()
	handler.Register(mux)

	// simple health check endpoint
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	addr := ":8080"
	log.Printf("listening on http://localhost%s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
