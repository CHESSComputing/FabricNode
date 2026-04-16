// Package did generates W3C Decentralized Identifier (DID) documents.
// This implementation uses did:web — a DID method where the DID document is
// hosted at a well-known HTTPS URL. For production, upgrade to did:webvh
// (which adds a verifiable history log) using the Credo-TS sidecar pattern
// described in the cogitarelink-fabric architecture.
package did

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

// KeyPair holds a node's signing key material.
type KeyPair struct {
	PublicKey  ed25519.PublicKey
	PrivateKey ed25519.PrivateKey
	// PublicKeyMultibase is the base64url-encoded public key with multibase prefix 'u'.
	PublicKeyMultibase string
}

// NewKeyPair generates a fresh Ed25519 key pair.
func NewKeyPair() (*KeyPair, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	encoded := "u" + base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(pub)
	return &KeyPair{
		PublicKey:          pub,
		PrivateKey:         priv,
		PublicKeyMultibase: encoded,
	}, nil
}

// Document is a W3C DID document (serialisable to JSON-LD).
type Document struct {
	Context            []string             `json:"@context"`
	ID                 string               `json:"id"`
	Created            string               `json:"created"`
	VerificationMethod []VerificationMethod `json:"verificationMethod"`
	Authentication     []string             `json:"authentication"`
	AssertionMethod    []string             `json:"assertionMethod"`
	Service            []Service            `json:"service"`
}

// VerificationMethod is a cryptographic key declaration in a DID document.
type VerificationMethod struct {
	ID                 string `json:"id"`
	Type               string `json:"type"`
	Controller         string `json:"controller"`
	PublicKeyMultibase string `json:"publicKeyMultibase"`
}

// Service is a service endpoint declaration in a DID document.
type Service struct {
	ID              string `json:"id"`
	Type            string `json:"type"`
	ServiceEndpoint string `json:"serviceEndpoint"`
}

// New builds a DID document for a CHESS fabric node.
func New(baseURL, nodeID string, kp *KeyPair, services []Service) *Document {
	did := fmt.Sprintf("did:web:%s", domainFromURL(baseURL))
	keyID := did + "#node-key-1"

	return &Document{
		Context: []string{
			"https://www.w3.org/ns/did/v1",
			"https://w3id.org/security/suites/ed25519-2020/v1",
		},
		ID:      did,
		Created: time.Now().UTC().Format(time.RFC3339),
		VerificationMethod: []VerificationMethod{{
			ID:                 keyID,
			Type:               "Ed25519VerificationKey2020",
			Controller:         did,
			PublicKeyMultibase: kp.PublicKeyMultibase,
		}},
		Authentication:  []string{keyID},
		AssertionMethod: []string{keyID},
		Service:         services,
	}
}

// JSON serialises the document.
func (d *Document) JSON() []byte {
	b, _ := json.MarshalIndent(d, "", "  ")
	return b
}

func domainFromURL(u string) string {
	u = stripScheme(u)
	// Replace port colon with %3A for did:web (spec requirement)
	// e.g. localhost:8783 → localhost%3A8783
	for i, c := range u {
		if c == ':' {
			return u[:i] + "%3A" + u[i+1:]
		}
		if c == '/' {
			return u[:i]
		}
	}
	return u
}

func stripScheme(u string) string {
	for _, prefix := range []string{"https://", "http://"} {
		if len(u) > len(prefix) && u[:len(prefix)] == prefix {
			return u[len(prefix):]
		}
	}
	return u
}
