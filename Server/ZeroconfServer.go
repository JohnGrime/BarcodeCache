package main

import (
	"github.com/grandcat/zeroconf"
)

//
// Zeroconf wrapper struct to aid system modularity
//
type ZeroconfServer struct {
	server *zeroconf.Server
}

// Advertises service using zeroconf; dnsTXT contains "key=value" for DNS-DS TXT entries
func (s *ZeroconfServer) Startup(name string, port int, dnsTXT []string) error {
	s.Shutdown()

	server, err := zeroconf.Register(name, "_http._tcp", "local.", port, dnsTXT, nil)
	if err == nil {
		s.server = server
	}

	return err
}

// Remove service from zeroconf
func (s *ZeroconfServer) Shutdown() {
	if s.server == nil { return }
	s.server.Shutdown()
}
