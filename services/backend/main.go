package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"cloud.google.com/go/firestore"
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

	// The OAuth 2.0 client ID ID tokens must be minted for; unset means no request can authenticate.
	googleClientID := os.Getenv("GOOGLE_CLIENT_ID")
	if googleClientID == "" {
		log.Print("warning: GOOGLE_CLIENT_ID is unset, so no request can authenticate")
	}

	ctx := context.Background()

	firestoreClient, err := firestore.NewClient(ctx, firestore.DetectProjectID)
	if err != nil {
		return fmt.Errorf("firestore client: %w", err)
	}
	defer firestoreClient.Close()

	verifier := &googleTokenVerifier{clientID: googleClientID}

	blogs := newBlogHandler(newFirestoreBlogStore(firestoreClient))
	users := newUserHandler(newFirestoreUserStore(firestoreClient))
	debugHandler := newDebugHandler(environment, commit)

	// This service holds the only credentials for the blog and user collections, so every one of
	// their routes requires a verified caller.
	authed := func(h http.HandlerFunc) http.Handler {
		return requireAuth(verifier, h)
	}

	mux := http.NewServeMux()
	mux.Handle("/health", debugHandler)
	mux.Handle("/debug", debugHandler)
	mux.Handle("GET /blogs", authed(blogs.List))
	mux.Handle("GET /blogs/{id}", authed(blogs.Get))
	mux.Handle("POST /blogs", authed(blogs.Create))
	mux.Handle("PUT /blogs/{id}", authed(blogs.Update))
	mux.Handle("DELETE /blogs/{id}", authed(blogs.Delete))
	mux.Handle("GET /users/{id}", authed(users.Get))
	mux.Handle("PUT /users/{id}", authed(users.Put))
	mux.Handle("DELETE /users/{id}", authed(users.Delete))

	// CORS wraps the whole mux rather than individual routes: routes are registered under a
	// specific method, so ServeMux would 405 an OPTIONS preflight before a per-route wrapper ran.
	log.Printf("backend listening on :%s (environment=%s, commit=%s)", port, environment, commit)
	return http.ListenAndServe(":"+port, withCORS(mux))
}
