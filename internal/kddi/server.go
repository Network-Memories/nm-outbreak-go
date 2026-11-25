package kddi

import (
	"context"
	"log"
	"net"
	"strconv"

	"networkmemories/nm-outbreak-go/internal/core"
	"networkmemories/nm-outbreak-go/internal/reports"
	"networkmemories/nm-outbreak-go/internal/world"
)

// Server handles KDDI TCP Lobby (8200/8300) and GameServer (8590/8690) streams
// directly backed by the shared in-memory world.Registry.
type Server struct {
	gameAddress string
	reports     reports.Sink
	name        string
	game        world.Game
	addr        string
	engine      *core.Engine
}

func NewTransport(name string, game world.Game, host string, port int, engine *core.Engine) *Server {
	return &Server{name: name, game: game, addr: net.JoinHostPort(host, strconv.Itoa(port)), engine: engine}
}

func (s *Server) Start(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	log.Printf("[nm.kddi.%s] TCP listener bound at %s", s.name, s.addr)

	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				continue
			}
			go s.serveConn(ctx, conn)
		}
	}()
	return nil
}

func (s *Server) serveConn(ctx context.Context, conn net.Conn) { s.serveNative(ctx, conn) }

// WithReports is configured before listeners start. No upload is acknowledged without durable storage.
func (s *Server) WithReports(sink reports.Sink) *Server { s.reports = sink; return s }

// WithGameAddress configures the native game listener's advertised endpoint.
func (s *Server) WithGameAddress(host string, port int) *Server {
	s.gameAddress = net.JoinHostPort(host, strconv.Itoa(port))
	return s
}
