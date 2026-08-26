// Command backend serves the blog.gorman.club API.
//
// It resolves configuration from the environment, builds the concrete adapters the service
// depends on (Firestore repositories, a Google token verifier), and hands them to the service
// package, which owns the routes and business logic.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	fs "cloud.google.com/go/firestore"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository/firestore"
	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository/google"
	"github.com/edgorman/blog.gorman.club/services/backend/internal/service"
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
	environment := envOr("ENVIRONMENT", "development")
	port := envOr("PORT", "8080")

	// The OAuth 2.0 client ID ID tokens must be minted for; unset means no request can authenticate.
	googleClientID := os.Getenv("GOOGLE_CLIENT_ID")
	if googleClientID == "" {
		log.Print("warning: GOOGLE_CLIENT_ID is unset, so no request can authenticate")
	}

	ctx := context.Background()

	client, err := fs.NewClient(ctx, fs.DetectProjectID)
	if err != nil {
		return fmt.Errorf("firestore client: %w", err)
	}
	defer client.Close()

	api := service.New(
		service.Config{
			Environment:   environment,
			Commit:        commit,
			AllowedOrigin: os.Getenv("CORS_ALLOWED_ORIGIN"),
		},
		firestore.NewBlogRepository(client),
		firestore.NewUserRepository(client),
		google.NewTokenVerifier(googleClientID),
	)

	log.Printf("backend listening on :%s (environment=%s, commit=%s)", port, environment, commit)
	return http.ListenAndServe(":"+port, api.Handler())
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
