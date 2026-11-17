package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Token represents response from OAuth server call
type Token struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Expires     int64  `json:"expires_in"`
}

// GetTokenFromFoxden returns a token from FOXDEN.
//
// The OAuth2 client_credentials token request is sent as a POST form body:
// the client secret must never appear in the URL, which proxies and access
// logs record. The HTTP client carries a timeout so a hung auth server
// cannot block service startup indefinitely.
func GetTokenFromFoxden(furl, cid, secret, scope string) string {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", cid)
	form.Set("client_secret", secret)
	if scope != "" {
		form.Set("scope", scope)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.PostForm(furl+"/oauth/token", form)
	if err != nil {
		log.Println("ERROR", err)
		return ""
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		log.Println("ERROR", err)
		return ""
	}
	if resp.StatusCode != http.StatusOK {
		log.Printf("ERROR: token endpoint returned HTTP %d: %.500s", resp.StatusCode, data)
		return ""
	}
	var response Token
	if err := json.Unmarshal(data, &response); err != nil {
		log.Println("ERROR", err)
		return ""
	}
	return response.AccessToken
}

// GetToken returns a token from either an environment variable
// or a file path (based on tokenSource value).
func GetToken(tokenSource string) string {
	// 1. Try environment variable
	if val, ok := os.LookupEnv(tokenSource); ok && strings.TrimSpace(val) != "" {
		return strings.TrimSpace(val)
	}

	// 2. Otherwise treat as file path
	data, err := os.ReadFile(tokenSource)
	if err == nil {
		token := strings.TrimSpace(string(data))
		return token
	}

	// Do NOT return tokenSource itself: if the operator configured an
	// environment-variable *name* that is unset, returning the name would
	// silently send a garbage bearer token upstream.
	return ""
}
