package main

import (
	"log"
	"net/http"
	"os"
)

// Baked in at build time via -ldflags (see Dockerfile); images are never rebuilt for production.
var commit = "unknown"

func main() {
	environment := os.Getenv("ENVIRONMENT")
	if environment == "" {
		environment = "development"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	handler := newDebugHandler(environment, commit)

	mux := http.NewServeMux()
	mux.Handle("/health", withCORS(handler))
	mux.Handle("/debug", withCORS(handler))

	log.Printf("backend listening on :%s (environment=%s, commit=%s)", port, environment, commit)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
