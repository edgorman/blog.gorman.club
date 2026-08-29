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
	"strings"

	fs "cloud.google.com/go/firestore"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository/firestore"
	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository/gemini"
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

	// The Gemini API is reached with the runtime service account's own credentials, so there is no
	// key to configure here - only which model, and which project to bill it to. GCP_PROJECT_ID is
	// passed explicitly rather than detected, because the deployment already knows it (see
	// infrastructure/env/cloud_run.tf) and a metadata lookup would only be a way of being told
	// something Terraform could have said.
	assistant := gemini.NewAssistant(gemini.Config{
		Model:     os.Getenv("ASSISTANT_MODEL"),
		ProjectID: os.Getenv("GCP_PROJECT_ID"),
	})

	// A deployment with no model has nobody on the allowlist, whatever the allowlist says: the
	// capability /users/me reports is then accurate, and a client never offers an assistant that
	// could only answer 503.
	allowlist := entity.NewAssistantAllowlist(splitList(os.Getenv("ASSISTANT_ALLOWED_EMAILS")))
	if !assistant.Configured() {
		log.Print("warning: ASSISTANT_MODEL is unset, so the writing assistant is disabled")
		allowlist = entity.NewAssistantAllowlist(nil)
	} else if allowlist.Empty() {
		log.Print("warning: ASSISTANT_ALLOWED_EMAILS is unset, so the writing assistant is enabled for nobody")
	}

	api := service.New(
		service.Config{
			Environment:        environment,
			Commit:             commit,
			AllowedOrigin:      os.Getenv("CORS_ALLOWED_ORIGIN"),
			AssistantAllowlist: allowlist,
		},
		firestore.NewBlogRepository(client),
		firestore.NewUserRepository(client),
		firestore.NewChatRepository(client),
		google.NewTokenVerifier(googleClientID),
		assistant,
	)

	log.Printf("backend listening on :%s (environment=%s, commit=%s)", port, environment, commit)
	return http.ListenAndServe(":"+port, api.Handler())
}

// splitList reads a comma-separated environment variable. Blanks are left in for the consumer to
// drop, since what counts as an empty entry is the consumer's rule rather than this function's.
func splitList(value string) []string {
	if value == "" {
		return nil
	}
	return strings.Split(value, ",")
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
