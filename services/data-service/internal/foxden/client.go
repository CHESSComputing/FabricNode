// Package foxden provides a typed HTTP client for FOXDEN metadata services
// and the service request/response structs that mirror the FOXDEN API.
//
// The service structs (ServiceQuery, ServiceRequest, ServiceResponse) will
// eventually be imported from golib/services; they are kept here until that
// package is available.
package foxden

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// ──────────────────────────────────────────────────────────────────────────────
// Service request / response types  (TODO: replace with golib/services import)
// ──────────────────────────────────────────────────────────────────────────────

// ServiceQuery describes the query parameters sent to a FOXDEN service.
type ServiceQuery struct {
	Query      string         `json:"query"`
	Spec       map[string]any `json:"spec"`
	Projection map[string]any `json:"projection"`
	SQL        string         `json:"sql"`
	Idx        int            `json:"idx"`
	Limit      int            `json:"limit"`
	SortKeys   []string       `json:"sort_keys"`
	SortOrder  int            `json:"sort_order"`
}

// ServiceRequest is the top-level envelope sent to a FOXDEN service endpoint.
type ServiceRequest struct {
	Client       string       `json:"client"`
	ServiceQuery ServiceQuery `json:"service_query"`
}

// ServiceResults holds the paginated record payload from a FOXDEN response.
type ServiceResults struct {
	NRecords int              `json:"nrecords"`
	Records  []map[string]any `json:"records"`
}

// ServiceResponse is the top-level envelope returned by a FOXDEN service.
type ServiceResponse struct {
	HttpCode int            `json:"http_code"`
	Status   string         `json:"status"`
	Error    string         `json:"error"`
	Results  ServiceResults `json:"results"`
}

// ──────────────────────────────────────────────────────────────────────────────
// Record helper — typed accessors for well-known FOXDEN metadata fields
// ──────────────────────────────────────────────────────────────────────────────

// Record wraps a raw FOXDEN metadata map and provides typed accessors for the
// fields used by data-service routing and RDF conversion.
type Record map[string]any

// DID returns the dataset identifier string, e.g.
// "/beamline=3a/btr=test-123-a/cycle=2026-1/sample_name=PAT-..."
func (r Record) DID() string {
	v, _ := r["did"].(string)
	return v
}

// BeamlineID returns the canonical lower-case beamline identifier extracted
// from the DID's leading segment (e.g. "3a" from "/beamline=3a/...").
// Falls back to lower-casing the first element of the "beamline" array field
// if the DID is absent.
func (r Record) BeamlineID() string {
	if did := r.DID(); did != "" {
		// DID starts with /beamline=<id>/...
		trimmed := strings.TrimPrefix(did, "/")
		seg := strings.SplitN(trimmed, "/", 2)[0] // "beamline=3a"
		if idx := strings.IndexByte(seg, '='); idx >= 0 {
			return strings.ToLower(seg[idx+1:])
		}
	}
	// Fallback: first element of the beamline array
	if arr, ok := r["beamline"].([]any); ok && len(arr) > 0 {
		if s, ok := arr[0].(string); ok {
			return strings.ToLower(s)
		}
	}
	return ""
}

// BTR returns the beam-time request identifier.
func (r Record) BTR() string {
	v, _ := r["btr"].(string)
	return v
}

// Cycle returns the run cycle string, e.g. "2026-1".
func (r Record) Cycle() string {
	v, _ := r["cycle"].(string)
	return v
}

// SampleName returns the sample name.
func (r Record) SampleName() string {
	v, _ := r["sample_name"].(string)
	return v
}

// PI returns the principal investigator name.
func (r Record) PI() string {
	v, _ := r["pi"].(string)
	return v
}

// Schema returns the metadata schema name (e.g. "ID3A").
func (r Record) Schema() string {
	v, _ := r["schema"].(string)
	return v
}

// ──────────────────────────────────────────────────────────────────────────────
// Client
// ──────────────────────────────────────────────────────────────────────────────

// Client is a typed HTTP client for FOXDEN metadata services.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a Client pointed at baseURL (e.g. "http://foxden:8300").
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Search sends a ServiceRequest to the FOXDEN /search endpoint and returns
// the decoded ServiceResponse.
func (c *Client) Search(req ServiceRequest) (*ServiceResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("foxden: marshal request: %w", err)
	}

	r, err := http.NewRequest(
		http.MethodPost,
		c.baseURL+"/search",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, err
	}

	// TODO: I need to introduce configuration for FabricNode which will allow
	// to setup auth client id and secret to obtain token from FOXDEN Authz service
	// for that I'll need to write dedicated code to send request to FOXDEN Authz service
	token := GetToken("FOXDEN_TOKEN")

	// Add headers
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json")
	r.Header.Set("Authorization", "Bearer "+token)

	// Execute request
	resp, err := c.httpClient.Do(r)
	/*
		resp, err := c.httpClient.Post(
			c.baseURL+"/search",
			"application/json",
			bytes.NewReader(body),
		)
	*/
	if err != nil {
		return nil, fmt.Errorf("foxden: POST /search: %w", err)
	}
	defer resp.Body.Close()

	//var out ServiceResponse
	var out []map[string]any
	err = json.NewDecoder(resp.Body).Decode(&out)
	if err != nil {
		return nil, fmt.Errorf("foxden: decode response: %w", err)
	}
	results := ServiceResults{
		Records:  out,
		NRecords: len(out),
	}
	status := "ok"
	if !strings.Contains(strings.ToLower(resp.Status), "ok") {
		status = resp.Status
	}
	sout := ServiceResponse{
		HttpCode: resp.StatusCode,
		Status:   status,
		Error:    "",
		Results:  results,
	}
	return &sout, nil
}

// QueryByBeamline returns all records for the given beamline name.
// FOXDEN stores beamline as an array of strings; the query matches on the
// lower-case canonical form (e.g. "3a") but FOXDEN itself accepts it
// case-insensitively via the spec filter.
func (c *Client) QueryByBeamline(beamline string, limit int) (*ServiceResponse, error) {
	if limit <= 0 {
		limit = 100
	}
	return c.Search(ServiceRequest{
		Client: "data-service",
		ServiceQuery: ServiceQuery{
			Spec:      map[string]any{"beamline": beamline},
			Limit:     limit,
			SortKeys:  []string{"cycle"},
			SortOrder: -1,
		},
	})
}

// QueryByDID returns the single record matching the given dataset DID.
func (c *Client) QueryByDID(did string) (*ServiceResponse, error) {
	return c.Search(ServiceRequest{
		Client: "data-service",
		ServiceQuery: ServiceQuery{
			Spec:  map[string]any{"did": did},
			Limit: 1,
		},
	})
}

// QueryByBeamlineAndCycle returns records filtered by beamline and cycle.
func (c *Client) QueryByBeamlineAndCycle(beamline, cycle string, limit int) (*ServiceResponse, error) {
	if limit <= 0 {
		limit = 100
	}
	return c.Search(ServiceRequest{
		Client: "data-service",
		ServiceQuery: ServiceQuery{
			Spec:      map[string]any{"beamline": beamline, "cycle": cycle},
			Limit:     limit,
			SortKeys:  []string{"cycle"},
			SortOrder: -1,
		},
	})
}

// GetToken returns a token from either an environment variable
// or a file path (based on tokenSource value).
func GetToken(tokenSource string) string {
	if tokenSource == "" {
		panic("tokenSource is empty")
	}

	// 1. Try environment variable
	if val, ok := os.LookupEnv(tokenSource); ok && strings.TrimSpace(val) != "" {
		return strings.TrimSpace(val)
	}

	// 2. Otherwise treat as file path
	data, err := os.ReadFile(tokenSource)
	if err != nil {
		panic(fmt.Sprintf("failed to read token from env or file (%s): %v", tokenSource, err))
	}

	token := strings.TrimSpace(string(data))
	if token == "" {
		panic(fmt.Sprintf("token is empty in file: %s", tokenSource))
	}

	return token
}
