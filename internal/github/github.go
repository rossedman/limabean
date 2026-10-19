// Package github checks GitHub tokens against the GitHub API.
package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// KeychainService is the Keychain item `limabean github login` stores the
// token in; secrets reference it as keychain:limabean-github-token.
const KeychainService = "limabean-github-token"

// SecretName is the environment variable the token is installed as.
const SecretName = "GITHUB_TOKEN"

// Ref is the secrets reference for the stored token.
const Ref = "keychain:" + KeychainService

// TokenInfo describes a valid token.
type TokenInfo struct {
	Login   string
	Expires string // "never" when the token doesn't expire
}

// FineGrained reports whether a token is a fine-grained personal access
// token, which can be limited to specific repositories.
func FineGrained(token string) bool { return strings.HasPrefix(token, "github_pat_") }

// ErrRejected means GitHub didn't accept the token.
var ErrRejected = errors.New("GitHub rejected the token")

// Client talks to the GitHub API.
type Client struct {
	BaseURL string // https://api.github.com by default
	HTTP    *http.Client
}

func (c Client) base() string {
	if c.BaseURL == "" {
		return "https://api.github.com"
	}
	return c.BaseURL
}

func (c Client) http() *http.Client {
	if c.HTTP == nil {
		return &http.Client{Timeout: 15 * time.Second}
	}
	return c.HTTP
}

// LatestRelease returns the tag of a repository's latest release, such as
// v0.3.0. It needs no token.
func (c Client) LatestRelease(ctx context.Context, repo string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base()+"/repos/"+repo+"/releases/latest", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := c.http().Do(req)
	if err != nil {
		return "", fmt.Errorf("couldn't reach GitHub: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub returned %s", resp.Status)
	}
	var rel struct {
		Tag string `json:"tag_name"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&rel); err != nil {
		return "", fmt.Errorf("unexpected reply from GitHub: %w", err)
	}
	return rel.Tag, nil
}

// Check asks GitHub who the token belongs to, and when it expires.
func (c Client) Check(ctx context.Context, token string) (TokenInfo, error) {
	base, hc := c.base(), c.http()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/user", nil)
	if err != nil {
		return TokenInfo{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := hc.Do(req)
	if err != nil {
		return TokenInfo{}, fmt.Errorf("couldn't reach GitHub: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return TokenInfo{}, ErrRejected
	}
	if resp.StatusCode != http.StatusOK {
		return TokenInfo{}, fmt.Errorf("GitHub returned %s", resp.Status)
	}
	var user struct {
		Login string `json:"login"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return TokenInfo{}, fmt.Errorf("unexpected reply from GitHub: %w", err)
	}
	info := TokenInfo{Login: user.Login, Expires: "never"}
	if exp := resp.Header.Get("github-authentication-token-expiration"); exp != "" {
		info.Expires = strings.Fields(exp)[0]
	}
	return info, nil
}
