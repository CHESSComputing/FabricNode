package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"

	"github.com/CHESSComputing/FabricNode/services/identity-service/internal/did"
	"github.com/CHESSComputing/FabricNode/services/identity-service/internal/vc"
)

// nodeState holds the keying material and pre-issued conformance credential
// generated at startup.  In production, persist keys to a secret store.
type nodeState struct {
	keyPair    *did.KeyPair
	didDoc     *did.Document
	conformVC  *vc.FabricConformanceCredential
}

func main() {
	baseURL := getEnv("NODE_BASE_URL", "http://localhost:8083")
	nodeID  := getEnv("NODE_ID", "chess-node")
	nodeName := getEnv("NODE_NAME", "CHESS Federated Knowledge Fabric Node")
	catalogURL := getEnv("CATALOG_URL", "http://localhost:8081")

	// ── Generate key pair at startup ────────────────────────────────────────
	kp, err := did.NewKeyPair()
	if err != nil {
		log.Fatalf("generate key pair: %v", err)
	}

	// ── Build DID document ───────────────────────────────────────────────────
	services := []did.Service{
		{
			ID:              fmt.Sprintf("did:web:%s#sparql", nodeID),
			Type:            "SPARQLEndpoint",
			ServiceEndpoint: getEnv("DATA_URL", "http://localhost:8082") + "/sparql",
		},
		{
			ID:              fmt.Sprintf("did:web:%s#void", nodeID),
			Type:            "VoIDDescription",
			ServiceEndpoint: catalogURL + "/.well-known/void",
		},
		{
			ID:              fmt.Sprintf("did:web:%s#shacl", nodeID),
			Type:            "SHACLShapes",
			ServiceEndpoint: catalogURL + "/.well-known/shacl",
		},
		{
			ID:              fmt.Sprintf("did:web:%s#ldnInbox", nodeID),
			Type:            "LDNInbox",
			ServiceEndpoint: getEnv("NOTIFICATION_URL", "http://localhost:8084") + "/inbox",
		},
	}
	didDoc := did.New(baseURL, nodeID, kp, services)
	keyID  := didDoc.ID + "#node-key-1"

	// ── Self-issue FabricConformanceCredential ───────────────────────────────
	conformVC := &vc.FabricConformanceCredential{
		Context: []string{
			"https://www.w3.org/2018/credentials/v1",
			"https://w3id.org/cogitarelink/fabric/v1",
		},
		ID:             baseURL + "/credentials/conformance/" + uuid.NewString(),
		Type:           []string{"VerifiableCredential", "FabricConformanceCredential"},
		Issuer:         didDoc.ID,
		IssuanceDate:   time.Now().UTC().Format(time.RFC3339),
		ExpirationDate: time.Now().AddDate(1, 0, 0).UTC().Format(time.RFC3339),
		CredentialSubject: vc.CredentialSubject{
			ID:         didDoc.ID,
			Type:       []string{"FabricNode"},
			ConformsTo: "https://w3id.org/cogitarelink/fabric#CoreProfile",
			NodeName:   nodeName,
			ServiceDir: []string{
				catalogURL + "/.well-known/void",
				catalogURL + "/.well-known/shacl",
				catalogURL + "/.well-known/sparql-examples",
			},
		},
	}
	if err := vc.Issue(conformVC, kp.PrivateKey, keyID); err != nil {
		log.Fatalf("issue conformance credential: %v", err)
	}

	state := &nodeState{
		keyPair:   kp,
		didDoc:    didDoc,
		conformVC: conformVC,
	}

	// ── Router ───────────────────────────────────────────────────────────────
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(cors)

	// W3C DID document
	r.Get("/.well-known/did.json", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/did+json")
		w.Header().Set("Link", `</.well-known/did.json>; rel="self"`)
		w.Write(state.didDoc.JSON())
	})

	// Conformance credential (self-issued VC)
	r.Get("/credentials/conformance", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/vc+json")
		json.NewEncoder(w).Encode(state.conformVC)
	})

	// Verify a submitted VC against this node's public key
	r.Post("/credentials/verify", func(w http.ResponseWriter, req *http.Request) {
		var cred vc.FabricConformanceCredential
		if err := json.NewDecoder(req.Body).Decode(&cred); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
			return
		}
		if err := vc.Verify(&cred, state.keyPair.PublicKey); err != nil {
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
	})

	// DID resolution (W3C DID Resolution HTTP API)
	r.Get("/did/{did}", func(w http.ResponseWriter, req *http.Request) {
		resolved := chi.URLParam(req, "did")
		_ = resolved
		// Simplified: return this node's DID document regardless of input DID
		// Production: resolve any did:web or did:webvh
		w.Header().Set("Content-Type", "application/did+json")
		w.Write(state.didDoc.JSON())
	})

	// Public key export (for external verification)
	r.Get("/keys/node-key-1", func(w http.ResponseWriter, req *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"id":                 keyID,
			"type":               "Ed25519VerificationKey2020",
			"publicKeyMultibase": state.keyPair.PublicKeyMultibase,
		})
	})

	r.Get("/health", func(w http.ResponseWriter, req *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"status":  "ok",
			"service": "identity-service",
			"did":     state.didDoc.ID,
		})
	})

	r.Get("/", func(w http.ResponseWriter, req *http.Request) {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"service": "identity-service",
			"did":     state.didDoc.ID,
			"endpoints": map[string]string{
				"didDocument":         "/.well-known/did.json",
				"conformanceVC":       "/credentials/conformance",
				"verifyVC":            "POST /credentials/verify",
				"resolveAnyDID":       "/did/{did}",
				"publicKey":           "/keys/node-key-1",
				"health":              "/health",
			},
		})
	})

	port := getEnv("PORT", "8083")
	log.Printf("identity-service listening on :%s (DID: %s)", port, state.didDoc.ID)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
