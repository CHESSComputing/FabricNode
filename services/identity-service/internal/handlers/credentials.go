package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/CHESSComputing/FabricNode/services/identity-service/internal/vc"
)

// ConformanceVC serves the pre-issued FabricConformanceCredential.
// GET /credentials/conformance
func ConformanceVC(s *NodeState) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/vc+json")
		json.NewEncoder(w).Encode(s.ConformVC)
	}
}

// VerifyVC verifies a submitted VC against this node's public key.
// POST /credentials/verify
func VerifyVC(s *NodeState) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		var cred vc.FabricConformanceCredential
		if err := json.NewDecoder(req.Body).Decode(&cred); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
			return
		}
		if err := vc.Verify(&cred, s.KeyPair.PublicKey); err != nil {
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"verified": false,
				"error":    err.Error(),
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"verified":           true,
			"issuer":             cred.Issuer,
			"verificationMethod": cred.Proof.VerificationMethod,
		})
	}
}
