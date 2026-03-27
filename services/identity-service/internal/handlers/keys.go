package handlers

import "net/http"

// PublicKey exports the node's public key for external verification.
// GET /keys/node-key-1
func PublicKey(s *NodeState) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"id":                 s.KeyID,
			"type":               "Ed25519VerificationKey2020",
			"publicKeyMultibase": s.KeyPair.PublicKeyMultibase,
		})
	}
}
