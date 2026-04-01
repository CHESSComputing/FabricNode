// Package server provides shared HTTP utilities used by every FabricNode
// service.  Placing these here removes the four identical copies that lived
// in each service's main package.
package server

import (
	"net/http"
	"os"
)

// GetEnv returns the value of the environment variable named key, or fallback
// if the variable is unset or empty.
func GetEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// CORSMiddleware adds permissive CORS headers suitable for all FabricNode
// services.  methods controls which HTTP methods are advertised; pass nil to
// use the read-only default ("GET, OPTIONS").
//
// Usage in main.go:
//
//	r.Use(server.CORSMiddleware(nil))                        // read-only service
//	r.Use(server.CORSMiddleware([]string{"GET","POST","OPTIONS"}))  // read-write
func CORSMiddleware(methods []string) func(http.Handler) http.Handler {
	allowed := "GET, OPTIONS"
	if len(methods) > 0 {
		joined := ""
		for i, m := range methods {
			if i > 0 {
				joined += ", "
			}
			joined += m
		}
		allowed = joined
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", allowed)
			w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Authorization")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ReadWriteCORS is a convenience alias for CORSMiddleware with GET, POST, OPTIONS.
// Use for services that accept writes (data-service, notification-service).
func ReadWriteCORS() func(http.Handler) http.Handler {
	return CORSMiddleware([]string{"GET", "POST", "OPTIONS"})
}

// ReadOnlyCORS is a convenience alias for CORSMiddleware with GET, OPTIONS only.
// Use for services that only serve reads (catalog-service).
func ReadOnlyCORS() func(http.Handler) http.Handler {
	return CORSMiddleware(nil)
}
