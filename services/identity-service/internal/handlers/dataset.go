package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/CHESSComputing/FabricNode/services/identity-service/internal/vc"
	"github.com/google/uuid"
)

// DatasetPublicationRequest is the JSON body sent by FOXDEN's DOI service
// when it wants the FabricNode to issue a publication credential.
//
// Minimum required fields: DID, DOI, DOIUrl.
// GraphIRI and SPARQLEndpoint are derived from DID if omitted.
type DatasetPublicationRequest struct {
	// DID is the canonical dataset identifier.
	// e.g. "/beamline=3a/btr=test-123-a/cycle=2026-1/sample_name=PAT-..."
	DID string `json:"did"`

	// DOI is the minted identifier, e.g. "10.5281/zenodo.123456".
	DOI string `json:"doi"`

	// DOIUrl is the resolvable URL, e.g. "https://doi.org/10.5281/zenodo.123456".
	DOIUrl string `json:"doi_url"`

	// GraphIRI overrides the derived named-graph IRI.
	// If empty it is derived from DID using the node's configured IRIBase:
	//   <IRIBase>graph/<beamline>/<did-segments>
	GraphIRI string `json:"graph_iri,omitempty"`

	// SPARQLEndpoint overrides the derived query URL.
	// If empty it is built from the node base URL + DID.
	SPARQLEndpoint string `json:"sparql_endpoint,omitempty"`
}

// IssueDatasetCredential issues a signed DatasetPublicationCredential for
// a dataset that has just been assigned a DOI by FOXDEN.
//
//	POST /credentials/dataset
//
// Request body: DatasetPublicationRequest (JSON)
// Response:     DatasetPublicationCredential (application/vc+json)
//
// FOXDEN calls this endpoint after minting a DOI so the credential can be:
//   - stored alongside the DOI record in FOXDEN
//   - returned to the dataset depositor as proof of publication
//   - used by external parties to verify the dataset's provenance
func IssueDatasetCredential(s *NodeState) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		var r DatasetPublicationRequest
		if err := json.NewDecoder(req.Body).Decode(&r); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "invalid JSON body: " + err.Error(),
			})
			return
		}

		// ── Validate required fields ──────────────────────────────────────────
		var missing []string
		if r.DID == "" {
			missing = append(missing, "did")
		}
		if r.DOI == "" {
			missing = append(missing, "doi")
		}
		if r.DOIUrl == "" {
			missing = append(missing, "doi_url")
		}
		if len(missing) > 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "missing required fields: " + strings.Join(missing, ", "),
			})
			return
		}

		// ── Derive optional fields ────────────────────────────────────────────
		graphIRI := r.GraphIRI
		if graphIRI == "" {
			graphIRI = graphIRIFromDID(s, r.DID)
		}

		sparqlEndpoint := r.SPARQLEndpoint
		if sparqlEndpoint == "" {
			sparqlEndpoint = sparqlEndpointFromDID(s, r.DID)
		}

		beamline := beamlineFromDID(r.DID)

		// ── Build credential ──────────────────────────────────────────────────
		now := time.Now().UTC()
		cred := &vc.DatasetPublicationCredential{
			Context: []string{
				"https://www.w3.org/2018/credentials/v1",
				"https://w3id.org/cogitarelink/fabric/v1",
				"https://schema.org/",
				"http://www.w3.org/ns/dcat#",
				"http://www.w3.org/ns/prov#",
			},
			ID:           s.DIDDoc.ID + "/credentials/dataset/" + uuid.NewString(),
			Type:         []string{"VerifiableCredential", "DatasetPublicationCredential"},
			Issuer:       s.DIDDoc.ID,
			IssuanceDate: now.Format(time.RFC3339),
			Subject: vc.DatasetPublicationSubject{
				ID:             r.DID,
				Type:           []string{"schema:Dataset", "dcat:Dataset"},
				GraphIRI:       graphIRI,
				DOI:            r.DOI,
				DOIUrl:         r.DOIUrl,
				SPARQLEndpoint: sparqlEndpoint,
				Beamline:       beamline,
				PublishedBy:    s.DIDDoc.ID,
				PublishedAt:    now.Format(time.RFC3339),
			},
		}

		// ── Sign ──────────────────────────────────────────────────────────────
		if err := vc.IssueDatasetCredential(cred, s.KeyPair.PrivateKey, s.KeyID); err != nil {
			log.Printf("IssueDatasetCredential: sign error: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "failed to sign credential",
			})
			return
		}

		log.Printf("issued DatasetPublicationCredential id=%s did=%s doi=%s", cred.ID, r.DID, r.DOI)

		w.Header().Set("Content-Type", "application/vc+json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(cred)
	}
}

// VerifyDatasetCredential verifies a submitted DatasetPublicationCredential
// against this node's public key.
//
//	POST /credentials/dataset/verify
func VerifyDatasetCredential(s *NodeState) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		var cred vc.DatasetPublicationCredential
		if err := json.NewDecoder(req.Body).Decode(&cred); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
			return
		}
		if err := vc.VerifyDatasetCredential(&cred, s.KeyPair.PublicKey); err != nil {
			writeJSON(w, http.StatusOK, map[string]any{
				"verified": false,
				"error":    err.Error(),
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"verified":    true,
			"issuer":      cred.Issuer,
			"did":         cred.Subject.ID,
			"doi":         cred.Subject.DOI,
			"graphIRI":    cred.Subject.GraphIRI,
			"publishedAt": cred.Subject.PublishedAt,
		})
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// URL derivation helpers
// ──────────────────────────────────────────────────────────────────────────────

// graphIRIFromDID derives the named-graph IRI from a dataset DID using the
// node's configured IRIBase.
// /beamline=3a/btr=test-123-a/... → <IRIBase>graph/3a/btr=test-123-a/...
func graphIRIFromDID(s *NodeState, did string) string {
	base := strings.TrimSuffix(s.IRIBase, "/")
	trimmed := strings.TrimPrefix(did, "/")
	parts := strings.SplitN(trimmed, "/", 2)
	if len(parts) < 2 {
		return fmt.Sprintf("%s/graph/unknown/%s", base, trimmed)
	}
	bl := ""
	if idx := strings.IndexByte(parts[0], '='); idx >= 0 {
		bl = strings.ToLower(parts[0][idx+1:])
	}
	return fmt.Sprintf("%s/graph/%s/%s", base, bl, parts[1])
}

// sparqlEndpointFromDID builds the data-service SPARQL URL for a dataset.
// It reads the SPARQL service endpoint from the node's DID document services.
func sparqlEndpointFromDID(s *NodeState, did string) string {
	// Find the SPARQLEndpoint service in the DID document
	sparqlBase := ""
	for _, svc := range s.DIDDoc.Service {
		if svc.Type == "SPARQLEndpoint" {
			// Strip /sparql suffix to get the data-service base URL
			sparqlBase = strings.TrimSuffix(svc.ServiceEndpoint, "/sparql")
			break
		}
	}
	if sparqlBase == "" {
		sparqlBase = "http://localhost:8782"
	}

	bl := beamlineFromDID(did)
	encoded := strings.NewReplacer("/", "%2F", "=", "%3D").Replace(did)
	return fmt.Sprintf("%s/beamlines/%s/datasets/%s/sparql", sparqlBase, bl, encoded)
}

// beamlineFromDID extracts the lower-case beamline ID from a DID.
func beamlineFromDID(did string) string {
	trimmed := strings.TrimPrefix(did, "/")
	seg := strings.SplitN(trimmed, "/", 2)[0] // "beamline=3a"
	if idx := strings.IndexByte(seg, '='); idx >= 0 {
		return strings.ToLower(seg[idx+1:])
	}
	return ""
}
