package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go/v4"
)

// Baked in at build time via -ldflags (see Dockerfile); images are never rebuilt for production.
var commit = "unknown"

func main() {
	// run() owns the deferred cleanup; log.Fatal here would skip it via os.Exit.
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	environment := os.Getenv("ENVIRONMENT")
	if environment == "" {
		environment = "development"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	ctx := context.Background()

	firestoreClient, err := firestore.NewClient(ctx, firestore.DetectProjectID)
	if err != nil {
		return fmt.Errorf("firestore client: %w", err)
	}
	defer firestoreClient.Close()

	firebaseApp, err := firebase.NewApp(ctx, nil)
	if err != nil {
		return fmt.Errorf("firebase app: %w", err)
	}
	firebaseAuth, err := firebaseApp.Auth(ctx)
	if err != nil {
		return fmt.Errorf("firebase auth client: %w", err)
	}
	verifier := &firebaseTokenVerifier{client: firebaseAuth}

	blogs := newBlogHandler(newFirestoreBlogStore(firestoreClient))
	debugHandler := newDebugHandler(environment, commit)

	// Every blog route requires a verified caller: this service holds the only credentials for
	// the collection, so it is where read and write access is decided.
	authed := func(h http.HandlerFunc) http.Handler {
		return withCORS(requireAuth(verifier, h))
	}

	mux := http.NewServeMux()
	mux.Handle("/health", withCORS(debugHandler))
	mux.Handle("/debug", withCORS(debugHandler))
	mux.Handle("GET /blogs", authed(blogs.List))
	mux.Handle("GET /blogs/{id}", authed(blogs.Get))
	mux.Handle("POST /blogs", authed(blogs.Create))
	mux.Handle("PUT /blogs/{id}", authed(blogs.Update))
	mux.Handle("DELETE /blogs/{id}", authed(blogs.Delete))

	log.Printf("backend listening on :%s (environment=%s, commit=%s)", port, environment, commit)
	return http.ListenAndServe(":"+port, mux)
}
