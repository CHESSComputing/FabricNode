package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// DIDDocument serves the W3C DID document for this node.
// GET /.well-known/did.json
func DIDDocument(s *NodeState) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/did+json")
		w.Header().Set("Link", `</.well-known/did.json>; rel="self"`)
		w.Write(s.DIDDoc.JSON())
	}
}

// DIDResolve handles W3C DID Resolution HTTP API requests.
// GET /did/{did}
// Simplified: returns this node's DID document regardless of the input DID.
// Production: resolve any did:web or did:webvh.
func DIDResolve(s *NodeState) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		_ = chi.URLParam(req, "did")
		w.Header().Set("Content-Type", "application/did+json")
		w.Write(s.DIDDoc.JSON())
	}
}
