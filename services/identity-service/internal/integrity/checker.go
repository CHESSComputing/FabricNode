// Package integrity provides content integrity helpers for the fabric node.
// It implements the digestMultibase / digestSRI mechanism (D26) used in
// FabricConformanceCredentials to bind self-description artifacts to hashes.
package integrity

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/CHESSComputing/FabricNode/services/identity-service/internal/vc"
)

// ArtifactCheck fetches a URL and verifies its SHA-256 hash matches the
// expected DigestSRI value embedded in the conformance credential.
func ArtifactCheck(ctx context.Context, rr vc.RelatedResource) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rr.ID, nil)
	if err != nil {
		return fmt.Errorf("build request for %s: %w", rr.ID, err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetch %s: %w", rr.ID, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read body of %s: %w", rr.ID, err)
	}

	gotSRI, _ := vc.ContentHash(body)
	if gotSRI != rr.DigestSRI {
		return fmt.Errorf("integrity mismatch for %s: expected %s, got %s",
			rr.ID, rr.DigestSRI, gotSRI)
	}
	return nil
}
