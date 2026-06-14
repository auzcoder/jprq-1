package cli

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"strings"

	"github.com/azimjohn/jprq/cli/debugger"
	"github.com/azimjohn/jprq/server/events"
	"github.com/azimjohn/jprq/server/tunnel"
)

type TunnelClient struct {
	config        Config
	protocol      string
	subdomain     string
	cname         string
	localServer   string
	remoteServer  string
	publicServer  string
	httpDebugger  debugger.Debugger
	eventCon      net.Conn
	LogCallback   func(string)
	ErrorCallback func(error)
}

func NewTunnelClient(conf Config, protocol string, subdomain string, cname string) *TunnelClient {
	return &TunnelClient{
		config:    conf,
		protocol:  protocol,
		subdomain: subdomain,
		cname:     cname,
	}
}

func (j *TunnelClient) logFatal(format string, v ...interface{}) {
	err := fmt.Errorf(format, v...)
	if j.ErrorCallback != nil {
		j.ErrorCallback(err)
		return
	}
	log.Fatal(err)
}

func (j *TunnelClient) logInfo(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	if j.LogCallback != nil {
		j.LogCallback(msg)
		return
	}
	log.Println(msg)
}

func (j *TunnelClient) Stop() {
	if j.eventCon != nil {
		j.eventCon.Close()
	}
}

func (j *TunnelClient) Start(port int, debug bool) {
	var err error
	j.eventCon, err = net.Dial("tcp", j.config.Remote.Events)
	if err != nil {
		j.logFatal("failed to connect to event server: %s", err)
		return
	}
	defer j.eventCon.Close()

	request := events.Event[events.TunnelRequested]{
		Data: &events.TunnelRequested{
			Protocol:   j.protocol,
			Subdomain:  j.subdomain,
			CanonName:  j.cname,
			AuthToken:  j.config.Local.AuthToken,
			CliVersion: version,
		},
	}
	if err := request.Write(j.eventCon); err != nil {
		j.logFatal("failed to send request: %s", err)
		return
	}

	var t events.Event[events.TunnelOpened]
	if err := t.Read(j.eventCon); err != nil {
		j.logFatal("failed to receive tunnel info: %s", err)
		return
	}
	if t.Data.ErrorMessage != "" {
		j.logFatal(t.Data.ErrorMessage)
		return
	}

	j.localServer = fmt.Sprintf("localhost:%d", port)
	j.remoteServer = fmt.Sprintf("jprq.%s:%d", j.config.Remote.Domain, t.Data.PrivateServer)
	j.publicServer = fmt.Sprintf("%s:%d", t.Data.Hostname, t.Data.PublicServer)

	if j.protocol == "http" {
		j.publicServer = fmt.Sprintf("https://%s", t.Data.Hostname)
	}

	j.logInfo("Status: \t Online")
	j.logInfo("Protocol: \t %s", strings.ToUpper(j.protocol))
	j.logInfo("Forwarded: \t %s -> %s", strings.TrimSuffix(j.publicServer, ":80"), j.localServer)

	if j.protocol == "http" && debug {
		j.httpDebugger = debugger.New()
		if port, err := j.httpDebugger.Run(0); err == nil {
			j.logInfo("Http Debugger: \t http://127.0.0.1:%d", port)
		}
	}

	var event events.Event[events.ConnectionReceived]
	for {
		if err := event.Read(j.eventCon); err != nil {
			j.logFatal("failed to receive connection-received event: %s", err)
			return
		}
		go j.handleEvent(*event.Data)
	}
}

func (j *TunnelClient) handleEvent(event events.ConnectionReceived) {
	localCon, err := net.Dial("tcp", j.localServer)
	if err != nil {
		j.logInfo("failed to connect to local server: %s", err)
		return
	}
	defer localCon.Close()

	remoteCon, err := net.Dial("tcp", j.remoteServer)
	if err != nil {
		j.logInfo("failed to connect to remote server: %s", err)
		return
	}
	defer remoteCon.Close()

	buffer := make([]byte, 2)
	binary.LittleEndian.PutUint16(buffer, event.ClientPort)
	remoteCon.Write(buffer)

	if j.httpDebugger == nil {
		go tunnel.Bind(localCon, remoteCon, nil)
		tunnel.Bind(remoteCon, localCon, nil)
		return
	}

	debugCon := j.httpDebugger.Connection(event.ClientPort)
	go tunnel.Bind(localCon, remoteCon, debugCon.Response())
	tunnel.Bind(remoteCon, localCon, debugCon.Request())
}
