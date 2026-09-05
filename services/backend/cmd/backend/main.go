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
	"time"

	fs "cloud.google.com/go/firestore"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository/firestore"
	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository/gemini"
	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository/google"
	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository/stripe"
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

	// The model is reached with the runtime service account's own credentials, so there is no key
	// to configure here - only which model, in which project, and where. GCP_PROJECT_ID is passed
	// explicitly rather than detected, because the deployment already knows it (see
	// infrastructure/env/cloud_run.tf) and a metadata lookup would only be a way of being told
	// something Terraform could have said.
	assistant := gemini.NewAssistant(gemini.Config{
		Model:     os.Getenv("ASSISTANT_MODEL"),
		ProjectID: os.Getenv("GCP_PROJECT_ID"),
		Location:  envOr("ASSISTANT_LOCATION", "global"),
	})

	// A deployment with no model entitles nobody, whatever an account has paid for: the capability
	// /users/me reports is then accurate, and a client never offers an assistant that could only
	// answer 503. Whether a model is configured is the whole of what this passes in - who is
	// entitled is each account's own subscription, which lives in Firestore rather than here.
	if !assistant.Configured() {
		log.Print("warning: ASSISTANT_MODEL or GCP_PROJECT_ID is unset, so the writing assistant is disabled")
	}
	entitlement := entity.NewAssistantEntitlement(assistant.Configured())

	// What that entitlement is bought with. The two secrets are the only long-lived credentials
	// this process holds - Stripe has no federated equivalent of the Workload Identity Federation
	// everything else here authenticates with - so they are mounted from GCP Secret Manager by
	// Cloud Run rather than baked into the image (see infrastructure/env/stripe.tf), and they are
	// per-environment: staging holds test-mode keys that cannot move real money.
	//
	// A deployment missing any of them sells nothing and is told about nothing, which is a
	// perfectly good deployment: every route but the two billing ones is unaffected, and the
	// subscriptions already recorded in Firestore keep granting the assistant until they run out.
	payments := stripe.NewPayments(stripe.Config{
		SecretKey:     os.Getenv("STRIPE_SECRET_KEY"),
		WebhookSecret: os.Getenv("STRIPE_WEBHOOK_SECRET"),
		PriceID:       os.Getenv("STRIPE_PRICE_ID"),
	})
	if !payments.Configured() {
		log.Print("warning: STRIPE_SECRET_KEY, STRIPE_WEBHOOK_SECRET or STRIPE_PRICE_ID is unset, so subscriptions cannot be bought")
	}

	api := service.New(
		service.Config{
			Environment:          environment,
			Commit:               commit,
			AllowedOrigin:        os.Getenv("CORS_ALLOWED_ORIGIN"),
			AssistantEntitlement: entitlement,
		},
		firestore.NewBlogRepository(client),
		firestore.NewUserRepository(client),
		firestore.NewChatRepository(client),
		firestore.NewCommentRepository(client),
		firestore.NewReactionRepository(client),
		google.NewTokenVerifier(googleClientID),
		assistant,
		payments,
	)

	// The server is built explicitly rather than handed to http.ListenAndServe, which supplies a
	// zero-value http.Server: one with no timeouts of any kind. A connection that opens and then
	// sends nothing, or dribbles a request a byte at a time, would hold a goroutine and a file
	// descriptor until the peer gave up - which costs the sender nothing and is the cheapest denial
	// of service there is. Cloud Run bounds the request it forwards but not the connection beneath
	// it, so the bound has to be here.
	//
	// WriteTimeout is the loose one, deliberately: it spans the whole response, and an assistant
	// turn legitimately runs to gemini.replyTimeout (two minutes) before it has anything to write.
	// Setting it below that would cut off precisely the requests that took the longest to earn an
	// answer, so it sits above replyTimeout with room, and still well inside Cloud Run's own limit.
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           api.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      3 * time.Minute,
		IdleTimeout:       2 * time.Minute,
	}

	log.Printf("backend listening on :%s (environment=%s, commit=%s)", port, environment, commit)
	return server.ListenAndServe()
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
