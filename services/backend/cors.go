package main

import (
	"net/http"
	"os"
)

// withCORS lets the frontend (a different origin: Cloudflare Pages, not
// Cloud Run) call this API from the browser. Origin is a single allowed
// value read from CORS_ALLOWED_ORIGIN rather than a wildcard, since it
// differs per environment (e.g. https://blog.gorman.club vs.
// https://staging.blog.gorman.club) and CLAUDE.md calls out CORS policy as
// one of the things the Debug Endpoint Contract exists to verify.
func withCORS(next http.Handler) http.Handler {
	origin := os.Getenv("CORS_ALLOWED_ORIGIN")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}

		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
