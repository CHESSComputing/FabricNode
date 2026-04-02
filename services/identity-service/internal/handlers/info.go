package handlers

import "net/http"

// Health returns service liveness and the node's DID.
// GET /health
func Health(s *NodeState) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"status":  "ok",
			"service": "identity-service",
			"did":     s.DIDDoc.ID,
		})
	}
}

// Index returns the service directory.
// GET /
func Index(s *NodeState) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"service": "identity-service",
			"did":     s.DIDDoc.ID,
			"endpoints": map[string]string{
				"didDocument":   "/.well-known/did.json",
				"conformanceVC": "/credentials/conformance",
				"verifyVC":      "POST /credentials/verify",
				"resolveAnyDID": "/did/{did}",
				"publicKey":     "/keys/node-key-1",
				"health":        "/health",
			},
		})
	}
}
