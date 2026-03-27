// Package client provides a typed Go client for the data-service API.
// It is used by catalog-service handlers to delegate data operations.
//
// All dataset addresses are expressed as model.DatasetRef values; the client
// handles URL encoding of the DID path parameter automatically.
package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/CHESSComputing/FabricNode/pkg/model"
)

// DataClient is a typed HTTP client for the data-service.
type DataClient struct {
	baseURL    string
	httpClient *http.Client
}

// New creates a DataClient pointed at baseURL (e.g. "http://localhost:8082").
func New(baseURL string) *DataClient {
	return &DataClient{
		baseURL: baseURL,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Triple write
// ──────────────────────────────────────────────────────────────────────────────

// Triple is a minimal representation matching store.Triple for serialisation.
type Triple struct {
	Subject    string `json:"subject"`
	Predicate  string `json:"predicate"`
	Object     string `json:"object"`
	ObjectType string `json:"objectType,omitempty"`
	Datatype   string `json:"datatype,omitempty"`
	Lang       string `json:"lang,omitempty"`
}

// InsertResponse is the JSON body returned by a successful POST /triples.
type InsertResponse struct {
	Inserted int    `json:"inserted"`
	Conforms bool   `json:"conforms"`
	GraphIRI string `json:"graphIRI"`
}

// InsertTriples POSTs triples into the dataset identified by ref.
//
// Calls: POST <baseURL>/beamlines/{beamline}/datasets/{did}/triples
func (c *DataClient) InsertTriples(ref model.DatasetRef, triples []Triple) (*InsertResponse, error) {
	body, err := json.Marshal(triples)
	if err != nil {
		return nil, fmt.Errorf("client: marshal triples: %w", err)
	}

	resp, err := c.httpClient.Post(c.datasetURL(ref, "triples"), "application/json",
		bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("client: POST triples: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("client: data-service returned %d: %s", resp.StatusCode, raw)
	}

	var result InsertResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("client: decode response: %w", err)
	}
	return &result, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// SPARQL query
// ──────────────────────────────────────────────────────────────────────────────

// SPARQLResponse is the SPARQL JSON Results format subset used by data-service.
type SPARQLResponse struct {
	Head    map[string][]string `json:"head"`
	Results struct {
		Bindings []map[string]map[string]string `json:"bindings"`
	} `json:"results"`
}

// QueryDataset issues a SPARQL-style query against a single dataset.
//
// Calls: GET <baseURL>/beamlines/{beamline}/datasets/{did}/sparql?s=…&p=…&o=…
func (c *DataClient) QueryDataset(ref model.DatasetRef, subject, predicate, object string) (*SPARQLResponse, error) {
	u := c.datasetURL(ref, "sparql")
	req, _ := http.NewRequest(http.MethodGet, u, nil)
	q := req.URL.Query()
	if subject != "" {
		q.Set("s", subject)
	}
	if predicate != "" {
		q.Set("p", predicate)
	}
	if object != "" {
		q.Set("o", object)
	}
	req.URL.RawQuery = q.Encode()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("client: GET sparql: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("client: data-service returned %d: %s", resp.StatusCode, raw)
	}
	var result SPARQLResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("client: decode sparql response: %w", err)
	}
	return &result, nil
}

// QueryBeamline issues a SPARQL-style query scoped to all datasets of a beamline.
//
// Calls: GET <baseURL>/beamlines/{beamline}/sparql?s=…&p=…&o=…
func (c *DataClient) QueryBeamline(bl model.BeamlineID, subject, predicate, object string) (*SPARQLResponse, error) {
	u := fmt.Sprintf("%s/beamlines/%s/sparql", c.baseURL, bl)
	req, _ := http.NewRequest(http.MethodGet, u, nil)
	q := req.URL.Query()
	if subject != "" {
		q.Set("s", subject)
	}
	if predicate != "" {
		q.Set("p", predicate)
	}
	if object != "" {
		q.Set("o", object)
	}
	req.URL.RawQuery = q.Encode()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("client: GET beamline sparql: %w", err)
	}
	defer resp.Body.Close()

	var result SPARQLResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("client: decode sparql response: %w", err)
	}
	return &result, nil
}

// GraphsForBeamline returns the named-graph IRIs (= dataset DIDs) for a beamline.
//
// Calls: GET <baseURL>/beamlines/{beamline}/graphs
func (c *DataClient) GraphsForBeamline(bl model.BeamlineID) ([]string, error) {
	u := fmt.Sprintf("%s/beamlines/%s/graphs", c.baseURL, bl)
	resp, err := c.httpClient.Get(u)
	if err != nil {
		return nil, fmt.Errorf("client: GET beamline graphs: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Graphs []string `json:"graphs"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("client: decode graphs response: %w", err)
	}
	return result.Graphs, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// URL construction
// ──────────────────────────────────────────────────────────────────────────────

// datasetURL builds the base URL for a dataset endpoint.
// The DID is URL-encoded because it contains slashes.
func (c *DataClient) datasetURL(ref model.DatasetRef, endpoint string) string {
	encodedDID := url.PathEscape(string(ref.DID))
	return fmt.Sprintf("%s/beamlines/%s/datasets/%s/%s",
		c.baseURL, ref.Beamline, encodedDID, endpoint)
}
