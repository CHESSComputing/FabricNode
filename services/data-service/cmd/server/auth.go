package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

// Token represents response from OAuth server call
type Token struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Expires     int64  `json:"expires_in"`
}

// GetTokenFromFoxden returns a token from FOXDEN
func GetTokenFromFoxden(furl, cid, secret, scope string) string {
	rurl := fmt.Sprintf(
		"%s/oauth/token?client_id=%s&response&client_secret=%s&grant_type=client_credentials&scope=%s", furl, cid, secret, scope)
	resp, err := http.Get(rurl)
	if err != nil {
		log.Println("ERROR", err)
		return ""
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	var response Token
	err = json.Unmarshal(data, &response)
	if err != nil {
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

	return tokenSource
}
