package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go/v4"
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

	ctx := context.Background()

	firestoreClient, err := firestore.NewClient(ctx, firestore.DetectProjectID)
	if err != nil {
		log.Fatalf("firestore client: %v", err)
	}
	defer firestoreClient.Close()

	firebaseApp, err := firebase.NewApp(ctx, nil)
	if err != nil {
		log.Fatalf("firebase app: %v", err)
	}
	firebaseAuth, err := firebaseApp.Auth(ctx)
	if err != nil {
		log.Fatalf("firebase auth client: %v", err)
	}
	verifier := &firebaseTokenVerifier{client: firebaseAuth}

	users := newUserHandler(newFirestoreUserStore(firestoreClient))
	blogs := newBlogHandler(newFirestoreBlogStore(firestoreClient))
	debugHandler := newDebugHandler(environment, commit)

	mux := http.NewServeMux()
	mux.Handle("/health", withCORS(debugHandler))
	mux.Handle("/debug", withCORS(debugHandler))
	mux.Handle("GET /users/{id}", withCORS(requireAuth(verifier, users.Get)))
	mux.Handle("PUT /users/{id}", withCORS(requireAuth(verifier, users.Put)))
	mux.Handle("GET /blogs", withCORS(requireAuth(verifier, blogs.List)))
	mux.Handle("POST /blogs", withCORS(requireAuth(verifier, blogs.Create)))
	mux.Handle("GET /blogs/{id}", withCORS(requireAuth(verifier, blogs.Get)))
	mux.Handle("PUT /blogs/{id}", withCORS(requireAuth(verifier, blogs.Update)))
	mux.Handle("DELETE /blogs/{id}", withCORS(requireAuth(verifier, blogs.Delete)))

	log.Printf("backend listening on :%s (environment=%s, commit=%s)", port, environment, commit)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
