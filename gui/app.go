package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/azimjohn/jprq/cli"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx      context.Context
	client   *cli.TunnelClient
	clientMu sync.Mutex
	exiting  bool
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// GetToken loads the local config and returns the saved auth token
func (a *App) GetToken() string {
	var conf cli.Config
	if err := conf.Load(); err != nil {
		// If error (e.g. file doesn't exist), we try to read it directly or return empty
		return ""
	}
	return conf.Local.AuthToken
}

// SetToken saves the auth token to the config file
func (a *App) SetToken(token string) error {
	config := cli.Config{}
	config.Local.AuthToken = strings.TrimSpace(token)
	return config.Write()
}

// StartTunnel starts a tunnel for the given protocol, port, and subdomain in the background
func (a *App) StartTunnel(protocol string, portStr string, subdomain string) error {
	a.clientMu.Lock()
	defer a.clientMu.Unlock()

	if a.client != nil {
		return fmt.Errorf("a tunnel is already running. Please stop it first")
	}

	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 {
		return fmt.Errorf("invalid port number: %s", portStr)
	}

	var conf cli.Config
	if err := conf.Load(); err != nil {
		return fmt.Errorf("configuration error: %s", err)
	}

	// Override or verify token is set
	if conf.Local.AuthToken == "" {
		return fmt.Errorf("auth token is not set. Please set it in settings")
	}

	client := cli.NewTunnelClient(conf, protocol, subdomain, "")
	client.LogCallback = func(msg string) {
		runtime.EventsEmit(a.ctx, "tunnel:log", msg)

		// Parse public URL from forwarded log
		// E.g. "Forwarded:      https://xyz.speedtunnel.io -> localhost:8000"
		if strings.Contains(msg, "Forwarded:") {
			parts := strings.Split(msg, "->")
			if len(parts) > 0 {
				urlPart := strings.TrimSpace(strings.Replace(parts[0], "Forwarded:", "", 1))
				runtime.EventsEmit(a.ctx, "tunnel:status", map[string]interface{}{
					"status": "online",
					"url":    urlPart,
				})
			}
		}
	}
	client.ErrorCallback = func(err error) {
		runtime.EventsEmit(a.ctx, "tunnel:log", fmt.Sprintf("Error: %s", err.Error()))
		runtime.EventsEmit(a.ctx, "tunnel:status", map[string]interface{}{
			"status": "error",
			"error":  err.Error(),
		})
		a.clientMu.Lock()
		a.client = nil
		a.clientMu.Unlock()
	}

	a.client = client

	// Run tunnel in a goroutine
	go func() {
		runtime.EventsEmit(a.ctx, "tunnel:status", map[string]interface{}{
			"status": "connecting",
		})
		client.Start(port, false)
	}()

	return nil
}

// StopTunnel stops the active tunnel
func (a *App) StopTunnel() error {
	a.clientMu.Lock()
	defer a.clientMu.Unlock()

	if a.client == nil {
		return fmt.Errorf("no active tunnel to stop")
	}

	a.client.Stop()
	a.client = nil

	runtime.EventsEmit(a.ctx, "tunnel:log", "Tunnel stopped by user.")
	runtime.EventsEmit(a.ctx, "tunnel:status", map[string]interface{}{
		"status": "offline",
	})

	return nil
}

// Quit terminates the application safely
func (a *App) Quit() {
	a.clientMu.Lock()
	a.exiting = true
	a.clientMu.Unlock()

	a.StopTunnel()
	runtime.Quit(a.ctx)
}
