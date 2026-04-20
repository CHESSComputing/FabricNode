package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/CHESSComputing/FabricNode/services/identity-service/internal/did"
	"github.com/CHESSComputing/FabricNode/services/identity-service/internal/vc"
)

// NodeState holds the keying material, pre-issued conformance credential,
// and node-wide IRI base generated at startup. Passed to every handler constructor.
type NodeState struct {
	KeyID     string
	KeyPair   *did.KeyPair
	DIDDoc    *did.Document
	ConformVC *vc.FabricConformanceCredential
	IRIBase   string // from Config.Node.IRIBase, e.g. "http://chess.cornell.edu/"
}

// writeJSON writes a JSON-encoded value with the given HTTP status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
