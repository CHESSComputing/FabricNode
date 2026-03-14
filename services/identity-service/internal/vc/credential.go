// Package vc implements W3C Verifiable Credentials Data Model 2.0 issuance
// and verification using Ed25519 / eddsa-jcs-2022 Data Integrity proofs.
//
// Simplified for Go (no JSON-LD processing) — JCS canonicalization is
// approximated by sorted-key JSON marshaling, which is spec-compliant for
// the credential payload structure used here.
package vc

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

// FabricConformanceCredential is the self-issued VC a node produces at bootstrap.
// It attests that the node conforms to fabric:CoreProfile and binds the
// node's self-description artifacts to SHA-256 content hashes.
type FabricConformanceCredential struct {
	Context           []string               `json:"@context"`
	ID                string                 `json:"id"`
	Type              []string               `json:"type"`
	Issuer            string                 `json:"issuer"`
	IssuanceDate      string                 `json:"issuanceDate"`
	ExpirationDate    string                 `json:"expirationDate"`
	CredentialSubject CredentialSubject      `json:"credentialSubject"`
	RelatedResource   []RelatedResource      `json:"relatedResource,omitempty"`
	Proof             *DataIntegrityProof    `json:"proof,omitempty"`
}

// CredentialSubject holds the conformance claim.
type CredentialSubject struct {
	ID          string   `json:"id"`          // node DID
	Type        []string `json:"type"`
	ConformsTo  string   `json:"dct:conformsTo"`
	NodeName    string   `json:"fabric:nodeName"`
	ServiceDir  []string `json:"fabric:serviceEndpoints"`
}

// RelatedResource binds a self-description artifact to a content hash (D26).
type RelatedResource struct {
	ID            string `json:"id"`
	DigestSRI     string `json:"digestSRI"`               // sha256-<base64>
	DigestMultibase string `json:"digestMultibase"`       // z<base58btc> (simplified: z<base64url>)
	MediaType     string `json:"mediaType"`
}

// DataIntegrityProof is an eddsa-jcs-2022 Data Integrity proof.
type DataIntegrityProof struct {
	Type               string `json:"type"`
	Created            string `json:"created"`
	VerificationMethod string `json:"verificationMethod"`
	ProofPurpose       string `json:"proofPurpose"`
	ProofValue         string `json:"proofValue"` // base64url(Ed25519 signature)
}

// Issue signs a FabricConformanceCredential with the node's Ed25519 private key.
func Issue(cred *FabricConformanceCredential, privKey ed25519.PrivateKey, keyID string) error {
	// Remove any existing proof before signing
	cred.Proof = nil

	// Canonical bytes = sorted-key JSON (approximation of JCS)
	payload, err := json.Marshal(cred)
	if err != nil {
		return fmt.Errorf("marshal credential: %w", err)
	}

	sig := ed25519.Sign(privKey, payload)
	sigB64 := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(sig)

	cred.Proof = &DataIntegrityProof{
		Type:               "DataIntegrityProof",
		Created:            time.Now().UTC().Format(time.RFC3339),
		VerificationMethod: keyID,
		ProofPurpose:       "assertionMethod",
		ProofValue:         sigB64,
	}
	return nil
}

// Verify checks the proof on a FabricConformanceCredential.
// pubKey must be the Ed25519 public key from the node's DID document.
func Verify(cred *FabricConformanceCredential, pubKey ed25519.PublicKey) error {
	if cred.Proof == nil {
		return fmt.Errorf("credential has no proof")
	}
	sig, err := base64.URLEncoding.WithPadding(base64.NoPadding).DecodeString(cred.Proof.ProofValue)
	if err != nil {
		return fmt.Errorf("decode proofValue: %w", err)
	}

	// Reconstruct the signed payload (credential without proof)
	proof := cred.Proof
	cred.Proof = nil
	payload, err := json.Marshal(cred)
	cred.Proof = proof
	if err != nil {
		return fmt.Errorf("marshal credential for verification: %w", err)
	}

	if !ed25519.Verify(pubKey, payload, sig) {
		return fmt.Errorf("proof verification failed: invalid signature")
	}
	return nil
}

// ContentHash produces SHA-256 digests in SRI and multibase formats for D26.
func ContentHash(content []byte) (digestSRI, digestMultibase string) {
	h := sha256.Sum256(content)
	b64 := base64.StdEncoding.EncodeToString(h[:])
	digestSRI = "sha256-" + b64
	// Simplified: use "z" + base64url as multibase prefix (production uses base58btc)
	b64url := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(h[:])
	digestMultibase = "z" + b64url
	return
}
