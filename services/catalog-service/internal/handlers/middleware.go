package handlers

import (
	"fmt"
	"net/http"
	"time"
)

// AddCacheHeaders sets Cache-Control, Vary, and Last-Modified on the response.
func AddCacheHeaders(w http.ResponseWriter, maxAgeSeconds int) {
	w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", maxAgeSeconds))
	w.Header().Set("Vary", "Accept")
	w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
}
