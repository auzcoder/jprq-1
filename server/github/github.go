package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const tokenPrefix = "gho_"

type User struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Login      string `json:"login"`
	Allowed    bool   `json:"allowed"`
	Tier       string `json:"tier,omitempty"`
	JoinedDate string `json:"created_at"`
}

type Authenticator interface {
	OAuthUrl() string
	ObtainToken(code string) (string, error)
	Authenticate(token string) (User, error)
}

type github struct {
	clientId          string
	clientSecret      string
	defaultScope      string
	userEndpoint      string
	qir2Endpoint      string
	jprqWebEndpoint   string
	jprqWebAuthSecret string
	redirectUri       string
	httpClient        *http.Client
}

// Options configures optional behavior on the Authenticator.
type Options struct {
	// JprqWebURL is the base URL of jprq-web. When set, Authenticate()
	// asks jprq-web whether the user has an active subscription and
	// upgrades user.Allowed accordingly. Empty string disables the check.
	JprqWebURL string
	// JprqInternalToken is the shared secret jprq-io uses to authenticate
	// itself to jprq-web's internal endpoints. Required when JprqWebURL is set.
	JprqInternalToken string
}

func New(clientId, clientSecret string, opts Options) Authenticator {
	g := github{
		clientId:          clientId,
		clientSecret:      clientSecret,
		defaultScope:      "user:email",
		userEndpoint:      "https://api.github.com/user",
		redirectUri:       "https://jprq.io/oauth-callback",
		qir2Endpoint:      "https://api.42.uz/api/profile/jprq/",
		jprqWebAuthSecret: opts.JprqInternalToken,
		httpClient:        &http.Client{Timeout: 10 * time.Second},
	}
	if opts.JprqWebURL != "" {
		g.jprqWebEndpoint = strings.TrimRight(opts.JprqWebURL, "/") + "/api/auth/validate"
	}
	return g
}

// client returns the configured HTTP client, or http.DefaultClient if the
// struct was instantiated directly (e.g. in tests) without going through New.
func (g github) client() *http.Client {
	if g.httpClient != nil {
		return g.httpClient
	}
	return http.DefaultClient
}

func (g github) OAuthUrl() string {
	return fmt.Sprintf("https://github.com/login/oauth/authorize?"+
		"client_id=%s&redirect_uri=%s&scope=%s", g.clientId, url.QueryEscape(g.redirectUri), g.defaultScope)
}

func (g github) ObtainToken(code string) (string, error) {
	payload := url.Values{}
	payload.Add("code", code)
	payload.Add("client_id", g.clientId)
	payload.Add("client_secret", g.clientSecret)

	req, err := http.NewRequest(
		"POST",
		"https://github.com/login/oauth/access_token",
		strings.NewReader(payload.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to perform http request: %v", err)
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Accept", "application/json")

	resp, err := g.client().Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to perform obtain token request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to obtain access token: http %d", resp.StatusCode)
	}

	var response struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("failed to decode github response: %v", err)
	}
	return strings.TrimLeft(response.AccessToken, tokenPrefix), nil
}

func (g github) Authenticate(token string) (User, error) {
	user, err := g.authenticate(g.userEndpoint, token)
	if err != nil {
		user, err = g.authenticate(g.qir2Endpoint, token)
		if err != nil {
			return user, err
		}
	}

	// If GitHub/42.uz already marked the user as allowed, skip the jprq-web
	// lookup — those identities are already trusted.
	if user.Allowed || g.jprqWebEndpoint == "" {
		return user, nil
	}

	// Ask jprq-web whether this login has an active subscription.
	// Failure here is non-fatal: we fall back to whatever GitHub returned
	// (Allowed=false), and the tunnel server's allow-list file still applies.
	if allowed, tier, jerr := g.validateWithJprqWeb(user.Login); jerr == nil {
		if allowed {
			user.Allowed = true
			user.Tier = tier
		}
	} else {
		fmt.Printf("jprq-web validate failed for %s: %v\n", user.Login, jerr)
	}
	return user, nil
}

func (g github) authenticate(endpoint, token string) (User, error) {
	user := User{}

	req, _ := http.NewRequest("GET", endpoint, nil)
	req.Header.Set("Authorization", fmt.Sprintf("token %s%s", tokenPrefix, token))
	resp, err := g.client().Do(req)

	if err != nil {
		return user, fmt.Errorf("authentication request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return user, fmt.Errorf("invalid token %v", token)
	}
	err = json.NewDecoder(resp.Body).Decode(&user)
	if err != nil {
		return user, fmt.Errorf("failed to decode user data: %v", err)
	}
	user.Login = strings.ToLower(user.Login)
	return user, nil
}

// validateWithJprqWeb asks jprq-web whether the given GitHub login has an
// active jprq subscription. Returns (allowed, tier, error). The endpoint is
// guarded by a shared service token so only jprq-io can call it.
func (g github) validateWithJprqWeb(login string) (bool, string, error) {
	body, err := json.Marshal(map[string]string{"github_login": login})
	if err != nil {
		return false, "", fmt.Errorf("marshal request: %v", err)
	}

	req, err := http.NewRequest("POST", g.jprqWebEndpoint, bytes.NewReader(body))
	if err != nil {
		return false, "", fmt.Errorf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+g.jprqWebAuthSecret)

	resp, err := g.client().Do(req)
	if err != nil {
		return false, "", fmt.Errorf("call jprq-web: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, "", fmt.Errorf("jprq-web returned http %d", resp.StatusCode)
	}

	var out struct {
		Allowed bool   `json:"allowed"`
		Tier    string `json:"tier"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return false, "", fmt.Errorf("decode jprq-web response: %v", err)
	}
	return out.Allowed, out.Tier, nil
}
