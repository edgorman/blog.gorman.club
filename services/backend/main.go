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

	// OAuth 2.0 client ID the frontend signs in with; ID tokens are only accepted if they were
	// minted for it. Unset means authentication is unconfigured, and every authenticated request
	// fails with a 500 rather than silently accepting anything.
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

	// Every blog and user route requires a verified caller: this service holds the only
	// credentials for those collections, so it is where read and write access is decided.
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

	// CORS wraps the whole mux, not individual routes: every route below is registered under a
	// specific method (e.g. "GET /blogs"), so an OPTIONS preflight matches none of them and
	// ServeMux itself would 405 it before a per-route wrapper ever ran. Wrapping here intercepts
	// OPTIONS ahead of that method-based routing.
	log.Printf("backend listening on :%s (environment=%s, commit=%s)", port, environment, commit)
	return http.ListenAndServe(":"+port, withCORS(mux))
}
