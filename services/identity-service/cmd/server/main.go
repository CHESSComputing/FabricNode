package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"

	"github.com/CHESSComputing/FabricNode/services/identity-service/internal/did"
	"github.com/CHESSComputing/FabricNode/services/identity-service/internal/handlers"
	"github.com/CHESSComputing/FabricNode/services/identity-service/internal/vc"
)

func main() {
	baseURL    := getEnv("NODE_BASE_URL", "http://localhost:8083")
	nodeID     := getEnv("NODE_ID", "chess-node")
	nodeName   := getEnv("NODE_NAME", "CHESS Federated Knowledge Fabric Node")
	catalogURL := getEnv("CATALOG_URL", "http://localhost:8081")

	// ── Generate key pair at startup ─────────────────────────────────────────
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

	state := &handlers.NodeState{
		KeyID:     keyID,
		KeyPair:   kp,
		DIDDoc:    didDoc,
		ConformVC: conformVC,
	}

	// ── Router ───────────────────────────────────────────────────────────────
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(cors)

	r.Get("/.well-known/did.json",     handlers.DIDDocument(state))
	r.Get("/credentials/conformance",  handlers.ConformanceVC(state))
	r.Post("/credentials/verify",      handlers.VerifyVC(state))
	r.Get("/did/{did}",                handlers.DIDResolve(state))
	r.Get("/keys/node-key-1",          handlers.PublicKey(state))
	r.Get("/health",                   handlers.Health(state))
	r.Get("/",                         handlers.Index(state))

	port := getEnv("PORT", "8083")
	log.Printf("identity-service listening on :%s (DID: %s)", port, didDoc.ID)
	log.Fatal(http.ListenAndServe(":"+port, r))
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
