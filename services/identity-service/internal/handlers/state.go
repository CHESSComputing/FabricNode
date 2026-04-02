package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/CHESSComputing/FabricNode/services/identity-service/internal/did"
	"github.com/CHESSComputing/FabricNode/services/identity-service/internal/vc"
)

// NodeState holds the keying material and pre-issued conformance credential
// generated at startup. Passed to every handler constructor.
type NodeState struct {
	KeyID     string
	KeyPair   *did.KeyPair
	DIDDoc    *did.Document
	ConformVC *vc.FabricConformanceCredential
}

// writeJSON writes a JSON-encoded value with the given HTTP status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
