package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/azimjohn/jprq/server/config"
	"github.com/azimjohn/jprq/server/events"
	"github.com/azimjohn/jprq/server/github"
	"github.com/azimjohn/jprq/server/server"
	"github.com/azimjohn/jprq/server/tunnel"
)

const dateFormat = "2006/01/02 15:04:05"

type Jprq struct {
	mu              sync.RWMutex
	config          config.Config
	eventServer     server.TCPServer
	publicServer    server.TCPServer
	publicServerTLS server.TCPServer
	allowedUsers    map[string]string
	allowedLastMod  time.Time
	authenticator   github.Authenticator
	cnameMap        map[string]string
	tcpTunnels      map[uint16]*tunnel.TCPTunnel
	httpTunnels     map[string]*tunnel.HTTPTunnel
	userTunnels     map[string]map[string]tunnel.Tunnel
}

func (j *Jprq) Init(conf config.Config, oauth github.Authenticator) error {
	j.config = conf
	j.authenticator = oauth
	j.allowedUsers = make(map[string]string)
	j.cnameMap = make(map[string]string)
	j.tcpTunnels = make(map[uint16]*tunnel.TCPTunnel)
	j.httpTunnels = make(map[string]*tunnel.HTTPTunnel)
	j.userTunnels = make(map[string]map[string]tunnel.Tunnel)

	if err := j.eventServer.Init(conf.EventServerPort, "speedtunnel_event_server"); err != nil {
		return err
	}
	if err := j.publicServer.Init(conf.PublicServerPort, "speedtunnel_public_server"); err != nil {
		return err
	}
	err := j.publicServerTLS.InitTLS(conf.PublicServerTLSPort, "speedtunnel_public_server_tls", conf.TLSCertFile, conf.TLSKeyFile)
	return err
}

func (j *Jprq) Start() {
	go j.eventServer.Start(j.serveEventConn)
	go j.publicServer.Start(j.servePublicConn)
	go j.publicServerTLS.Start(j.servePublicConn)

	go func() { // periodically load allowed users
		j.loadAllowedUsers()
		for range time.Tick(5 * time.Second) {
			j.loadAllowedUsers()
		}
	}()
}

func (j *Jprq) Stop() error {
	if err := j.eventServer.Stop(); err != nil {
		return err
	}
	if err := j.publicServer.Stop(); err != nil {
		return err
	}
	err := j.publicServerTLS.Stop()
	return err
}

func (j *Jprq) servePublicConn(conn net.Conn) error {
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	host, buffer, err := parseHost(conn)
	if err != nil || host == "" {
		writeResponse(conn, 400, "Bad Request", "Bad Request")
		return nil
	}

	j.mu.RLock()
	if tunnelHost, ok := j.cnameMap[host]; ok && tunnelHost != "" {
		host = tunnelHost
	}
	host = strings.ToLower(host)
	t, found := j.httpTunnels[host]
	j.mu.RUnlock()

	if !found {
		writeResponse(conn, 404, "Not Found", "tunnel not found")
		return fmt.Errorf("unknown host requested %s", host)
	}
	return t.PublicConnectionHandler(conn, buffer)
}

func (j *Jprq) serveEventConn(conn net.Conn) error {
	defer conn.Close()

	var event events.Event[events.TunnelRequested]
	if err := event.Read(conn); err != nil {
		return err
	}

	request := event.Data
	if request.Protocol != events.HTTP && request.Protocol != events.TCP {
		return events.WriteError(conn, "invalid protocol %s", request.Protocol)
	}
	user, err := j.authenticator.Authenticate(request.AuthToken)
	if err != nil {
		return events.WriteError(conn, "authentication failed %s", "\n\tobtain auth token from https://jprq.io/auth\n")
	}

	j.mu.RLock()
	_, allowedFound := j.allowedUsers[user.Login]
	j.mu.RUnlock()
	if !allowedFound && !user.Allowed {
		return events.WriteError(conn, "SpeedTunnel is now invite-only service %s\n", "\n\tbuy membership - https://buymeacoffee.com/azimjon \n")
	}

	j.mu.RLock()
	userTunnelCount := len(j.userTunnels[user.Login])
	j.mu.RUnlock()
	if userTunnelCount >= j.config.MaxTunnelsPerUser {
		return events.WriteError(conn, "tunnels limit reached for %s", user.Login)
	}

	if request.Subdomain == "" {
		request.Subdomain = user.Login
	}
	if err := validate(&request.Subdomain); err != nil {
		return events.WriteError(conn, "invalid subdomain %s: %s", request.Subdomain, err.Error())
	}
	hostname := fmt.Sprintf("%s.%s", request.Subdomain, j.config.DomainName)

	j.mu.RLock()
	_, httpBusy := j.httpTunnels[hostname]
	j.mu.RUnlock()
	if httpBusy {
		return events.WriteError(conn, "subdomain is busy: %s, try another one", request.Subdomain)
	}

	cname := request.CanonName
	j.mu.RLock()
	_, cnameBusy := j.cnameMap[cname]
	j.mu.RUnlock()
	if cnameBusy && cname != "" {
		return events.WriteError(conn, "cname is busy: %s, try another one", request.CanonName)
	}

	var t tunnel.Tunnel
	maxConsLimit := j.config.MaxConsPerTunnel

	switch request.Protocol {
	case events.HTTP:
		tn, err := tunnel.NewHTTP(hostname, conn, maxConsLimit)
		if err != nil {
			return events.WriteError(conn, "failed to create http tunnel: %s", err.Error())
		}
		j.mu.Lock()
		j.cnameMap[cname] = hostname
		j.httpTunnels[hostname] = tn
		j.mu.Unlock()
		defer func() {
			j.mu.Lock()
			delete(j.cnameMap, cname)
			delete(j.httpTunnels, hostname)
			j.mu.Unlock()
		}()
		t = tn
	case events.TCP:
		tn, err := tunnel.NewTCP(hostname, conn, maxConsLimit)
		if err != nil {
			return events.WriteError(conn, "failed to create tcp tunnel: %s", err.Error())
		}
		j.mu.Lock()
		j.tcpTunnels[tn.PublicServerPort()] = tn
		j.mu.Unlock()
		defer func() {
			j.mu.Lock()
			delete(j.tcpTunnels, tn.PublicServerPort())
			j.mu.Unlock()
		}()
		t = tn
	}

	j.mu.Lock()
	if len(j.userTunnels[user.Login]) == 0 {
		j.userTunnels[user.Login] = make(map[string]tunnel.Tunnel)
	}
	tunnelId := fmt.Sprintf("%s:%d", t.Hostname(), t.PublicServerPort())
	j.userTunnels[user.Login][tunnelId] = t
	j.mu.Unlock()
	defer func() {
		j.mu.Lock()
		delete(j.userTunnels[user.Login], tunnelId)
		j.mu.Unlock()
	}()

	t.Open()
	defer t.Close()
	opened := events.Event[events.TunnelOpened]{
		Data: &events.TunnelOpened{
			Hostname:      t.Hostname(),
			Protocol:      t.Protocol(),
			PublicServer:  t.PublicServerPort(),
			PrivateServer: t.PrivateServerPort(),
		},
	}
	if err := opened.Write(conn); err != nil {
		return err
	}

	fmt.Printf("%s [tunnel-opened] %s: %s\n", time.Now().Format(dateFormat), user.Login, tunnelId)

	buffer := make([]byte, 8) // wait until connection is closed
	for {
		_ = conn.SetReadDeadline(time.Now().Add(time.Minute))
		if _, err := conn.Read(buffer); err == io.EOF {
			break
		}
		j.mu.RLock()
		_, stillAllowed := j.allowedUsers[user.Login]
		j.mu.RUnlock()
		if !stillAllowed && !user.Allowed {
			break
		}
	}
	fmt.Printf("%s [tunnel-closed] %s: %s\n", time.Now().Format(dateFormat), user.Login, tunnelId)
	return nil
}

func (j *Jprq) loadAllowedUsers() {
	stat, err := os.Stat(j.config.AllowedUsersFile)
	if err != nil {
		log.Printf("failed to stat allowed users file: %s", err)
		return
	}
	if !stat.ModTime().After(j.allowedLastMod) {
		return
	}
	file, err := os.Open(j.config.AllowedUsersFile)
	if err != nil {
		log.Printf("failed to read allowed users file: %s", err)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	newAllowed := make(map[string]string)

	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), ",")
		if len(fields) >= 2 {
			login := strings.TrimSpace(fields[0])
			newAllowed[login] = strings.ToLower(fields[1])
		}
	}

	j.mu.Lock()
	j.allowedUsers = newAllowed
	j.mu.Unlock()

	j.allowedLastMod = stat.ModTime()
	log.Println("allow-list updated")
}
