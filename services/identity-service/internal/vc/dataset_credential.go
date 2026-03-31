// dataset_credential.go — W3C Verifiable Credential for dataset publication.
//
// A DatasetPublicationCredential is issued by the FabricNode identity-service
// at the moment FOXDEN mints a DOI for a dataset.  It binds together:
//
//   - the dataset DID (canonical identifier within the knowledge graph)
//   - the named-graph IRI (where RDF triples live in the data-service)
//   - the DOI minted by FOXDEN
//   - the SPARQL endpoint where the dataset is queryable
//   - the issuing node's DID (for external verification)
//
// The credential is signed with the node's Ed25519 private key so any
// party who can resolve the node's DID document can independently verify it.
package vc

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

// DatasetPublicationCredential is the VC issued when a dataset is published
// via FOXDEN's DOI service.
type DatasetPublicationCredential struct {
	Context        []string                       `json:"@context"`
	ID             string                         `json:"id"`
	Type           []string                       `json:"type"`
	Issuer         string                         `json:"issuer"`
	IssuanceDate   string                         `json:"issuanceDate"`
	ExpirationDate string                         `json:"expirationDate,omitempty"`
	Subject        DatasetPublicationSubject      `json:"credentialSubject"`
	Proof          *DataIntegrityProof            `json:"proof,omitempty"`
}

// DatasetPublicationSubject carries the dataset-specific claims.
type DatasetPublicationSubject struct {
	// ID is the dataset DID, e.g.
	// /beamline=3a/btr=test-123-a/cycle=2026-1/sample_name=PAT-...
	ID string `json:"id"`

	// Type identifies this as a published CHESS dataset.
	Type []string `json:"type"`

	// GraphIRI is the named-graph IRI in the FabricNode data-service where
	// the dataset's RDF triples are stored.
	GraphIRI string `json:"fabric:graphIRI"`

	// DOI is the minted Digital Object Identifier, e.g. "10.5281/zenodo.123456".
	DOI string `json:"schema:identifier"`

	// DOIUrl is the resolvable DOI URL, e.g. "https://doi.org/10.5281/zenodo.123456".
	DOIUrl string `json:"schema:url"`

	// SPARQLEndpoint is the URL where the dataset can be queried.
	SPARQLEndpoint string `json:"dcat:accessURL"`

	// Beamline is the canonical beamline identifier, e.g. "3a".
	Beamline string `json:"chess:beamline"`

	// PublishedBy is the DID of the FabricNode that issued this credential.
	PublishedBy string `json:"prov:wasAttributedTo"`

	// PublishedAt is the RFC3339 timestamp of publication.
	PublishedAt string `json:"prov:generatedAtTime"`
}

// IssueDatasetCredential signs a DatasetPublicationCredential with the
// node's Ed25519 private key.  The proof is attached in-place.
func IssueDatasetCredential(cred *DatasetPublicationCredential, privKey ed25519.PrivateKey, keyID string) error {
	cred.Proof = nil
	payload, err := json.Marshal(cred)
	if err != nil {
		return fmt.Errorf("marshal dataset credential: %w", err)
	}
	sig := ed25519.Sign(privKey, payload)
	cred.Proof = &DataIntegrityProof{
		Type:               "DataIntegrityProof",
		Created:            time.Now().UTC().Format(time.RFC3339),
		VerificationMethod: keyID,
		ProofPurpose:       "assertionMethod",
		ProofValue:         base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(sig),
	}
	return nil
}

// VerifyDatasetCredential checks the proof on a DatasetPublicationCredential.
func VerifyDatasetCredential(cred *DatasetPublicationCredential, pubKey ed25519.PublicKey) error {
	if cred.Proof == nil {
		return fmt.Errorf("credential has no proof")
	}
	sig, err := base64.URLEncoding.WithPadding(base64.NoPadding).DecodeString(cred.Proof.ProofValue)
	if err != nil {
		return fmt.Errorf("decode proofValue: %w", err)
	}
	proof := cred.Proof
	cred.Proof = nil
	payload, err := json.Marshal(cred)
	cred.Proof = proof
	if err != nil {
		return fmt.Errorf("marshal for verification: %w", err)
	}
	if !ed25519.Verify(pubKey, payload, sig) {
		return fmt.Errorf("dataset credential proof verification failed")
	}
	return nil
}
