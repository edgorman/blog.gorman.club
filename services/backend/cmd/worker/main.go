// Command worker handles the Firestore document events Eventarc delivers from the blog's
// collections. It is not publicly reachable: see infrastructure/env/worker.tf.
package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	fs "cloud.google.com/go/firestore"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository/firestore"
	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository/gemini"
	"github.com/edgorman/blog.gorman.club/services/backend/internal/worker"
)

// backfillTimeout bounds the reconcile run before the worker starts listening, well inside Cloud
// Run's default four-minute startup probe.
const backfillTimeout = 2 * time.Minute

// Baked in at build time via -ldflags (see Dockerfile); images are never rebuilt for production.
var commit = "unknown"

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// JSON on stdout under the key names Cloud Logging reads, so each line is one structured entry
	// with its severity rather than a text payload.
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			switch a.Key {
			case slog.MessageKey:
				a.Key = "message"
			case slog.LevelKey:
				a.Key = "severity"
			}
			return a
		},
	})).With("commit", commit)

	ctx := context.Background()
	client, err := fs.NewClient(ctx, fs.DetectProjectID)
	if err != nil {
		log.Fatalf("firestore client: %v", err)
	}
	defer client.Close()

	dimension, _ := strconv.Atoi(os.Getenv("EMBEDDING_DIMENSION"))
	embedder := gemini.NewEmbedder(gemini.EmbedderConfig{
		Config: gemini.Config{
			Model:     os.Getenv("EMBEDDING_MODEL"),
			ProjectID: os.Getenv("GCP_PROJECT_ID"),
			Location:  os.Getenv("EMBEDDING_LOCATION"),
		},
		Dimension: dimension,
	})

	var blogHandler worker.Handler
	if embedder.Configured() {
		blogs := firestore.NewBlogRepository(client)
		embeddings := worker.Embeddings{
			Blogs:      blogs,
			Embeddings: firestore.NewEmbeddingRepository(client),
			Embedder:   embedder,
		}
		blogHandler = embeddings.Handle
		backfill(ctx, logger, blogs, embeddings)
	} else {
		logger.Warn("EMBEDDING_MODEL, EMBEDDING_DIMENSION or GCP_PROJECT_ID is unset, so posts are not embedded")
	}

	// The moderation model is configured like the backend's assistant (see cmd/backend) and
	// reached as the worker's own runtime service account. Without one, comments go unscreened.
	var commentHandler worker.Handler
	if moderator := gemini.NewModerator(gemini.Config{
		Model:     os.Getenv("MODERATION_MODEL"),
		ProjectID: os.Getenv("GCP_PROJECT_ID"),
		Location:  os.Getenv("MODERATION_LOCATION"),
	}); moderator.Configured() {
		commentHandler = worker.ModerateComment(logger, firestore.NewCommentRepository(client), moderator)
	} else {
		logger.Warn("MODERATION_MODEL or GCP_PROJECT_ID is unset, so comments are not moderated")
	}

	// Timeouts for the same reason as cmd/backend's server. An event makes at most one model call.
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           worker.New(logger, blogHandler, commentHandler),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      time.Minute,
		IdleTimeout:       2 * time.Minute,
	}

	logger.Info("worker listening", "port", port)
	log.Fatal(server.ListenAndServe())
}

// backfill syncs every post's embedding before the worker takes events, so posts written before
// embeddings existed, or while the worker was failing, catch up on the next start - and a deploy
// always starts one. A post already in step costs two reads and no model call, and a failure is
// logged rather than fatal: the next write to that post, or the next start, tries again.
//
// ponytail: reads every post on each cold start; move to a Cloud Run job if the collection grows
// past a few thousand posts.
func backfill(ctx context.Context, logger *slog.Logger, blogs *firestore.BlogRepository, embeddings worker.Embeddings) {
	ctx, cancel := context.WithTimeout(ctx, backfillTimeout)
	defer cancel()

	slugs, err := blogs.Slugs(ctx)
	if err != nil {
		logger.Error("backfill: list posts", "error", err)
		return
	}
	failed := 0
	for _, slug := range slugs {
		if err := embeddings.Sync(ctx, slug); err != nil {
			failed++
			logger.Error("backfill: sync embedding", "document", "blogs/"+slug, "error", err)
		}
	}
	logger.Info("backfill done", "posts", len(slugs), "failed", failed)
}
