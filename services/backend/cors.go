package main

import (
	"net/http"
	"os"
)

// withCORS allows the frontend's origin (CORS_ALLOWED_ORIGIN, which differs per environment) to
// call this API from the browser.
func withCORS(next http.Handler) http.Handler {
	origin := os.Getenv("CORS_ALLOWED_ORIGIN")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}

		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Authorization-Provider, Content-Type")
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
