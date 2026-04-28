package main

import (
	_ "embed"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/azimjohn/jprq/server/github"
)

var oauth github.Authenticator

//go:embed static/index.html
var html string

//go:embed static/config.json
var config string

//go:embed static/install.sh
var installer string

//go:embed static/token.html
var tokenHtml string

func main() {
	clientId := os.Getenv("GITHUB_CLIENT_ID")
	clientSecret := os.Getenv("GITHUB_CLIENT_SECRET")
	if clientId == "" || clientSecret == "" {
		log.Fatalf("missing github client id/secret")
	}
	oauth = github.New(clientId, clientSecret, github.Options{})

	http.HandleFunc("/", contentHandler([]byte(html), "text/html"))
	http.HandleFunc("/config.json", contentHandler([]byte(config), "application/json"))
	http.HandleFunc("/install.sh", contentHandler([]byte(installer), "text/x-shellscript"))
	http.HandleFunc("/auth", authHandler)
	http.HandleFunc("/oauth-callback", oauthCallback)

	log.Print("Listening on 127.0.0.1:3300")
	log.Fatal(http.ListenAndServe(":3300", nil))
}

func contentHandler(content []byte, contentType string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentType)
		w.Write(content)
	}
}

func authHandler(w http.ResponseWriter, r *http.Request) {
	app := r.URL.Query().Get("app")
	callback := r.URL.Query().Get("callback")
	oauthURL := oauth.OAuthUrl()

	if app != "" {
		http.SetCookie(w, &http.Cookie{
			Name:     "jprq_app",
			Value:    app,
			Path:     "/",
			MaxAge:   300,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})
	}

	if callback != "" {
		http.SetCookie(w, &http.Cookie{
			Name:     "jprq_callback",
			Value:    callback,
			Path:     "/",
			MaxAge:   300,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})
	}

	http.Redirect(w, r, oauthURL, http.StatusFound)
}

func oauthCallback(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil || r.FormValue("code") == "" {
		http.Redirect(w, r, "/auth", http.StatusTemporaryRedirect)
		return
	}
	token, err := oauth.ObtainToken(r.FormValue("code"))
	if err != nil || token == "" {
		fmt.Printf("error obtaining token: %s\n", err)
		http.Redirect(w, r, "/auth", http.StatusTemporaryRedirect)
		return
	}

	// Subscription / allow-list checks live in jprq-web now;
	// this endpoint just hands the token back to the desktop app.

	// Check if this is an app-based authentication
	appCookie, err := r.Cookie("jprq_app")
	callbackCookie, _ := r.Cookie("jprq_callback")

	if err == nil && appCookie.Value != "" {
		// Clear cookies
		http.SetCookie(w, &http.Cookie{
			Name: "jprq_app", Value: "", Path: "/", MaxAge: -1, HttpOnly: true,
		})
		http.SetCookie(w, &http.Cookie{
			Name: "jprq_callback", Value: "", Path: "/", MaxAge: -1, HttpOnly: true,
		})

		// If callback URL provided, redirect there instead of deep link.
		// Parse the URL so we preserve any pre-existing query params (e.g. state)
		// and append `token` correctly using `&` instead of a second `?`.
		if callbackCookie != nil && callbackCookie.Value != "" {
			parsed, perr := url.Parse(callbackCookie.Value)
			if perr == nil && parsed.Scheme != "" && parsed.Host != "" {
				q := parsed.Query()
				q.Set("token", token)
				parsed.RawQuery = q.Encode()
				http.Redirect(w, r, parsed.String(), http.StatusFound)
				return
			}
			fmt.Printf("invalid callback URL %q: %v\n", callbackCookie.Value, perr)
		}

		// Fall back to deep link
		switch appCookie.Value {
		case "mac", "windows", "linux":
			appURL := fmt.Sprintf("jprq://auth/callback?token=%s", token)
			http.Redirect(w, r, appURL, http.StatusFound)
		default:
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(fmt.Sprintf(tokenHtml, token)))
		}
		return
	}

	// Default: show token in web page
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(fmt.Sprintf(tokenHtml, token)))
}

