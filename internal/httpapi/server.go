package httpapi

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"

	"networkmemories/nm-outbreak-go/internal/stun"
	"networkmemories/nm-outbreak-go/internal/world"
)

type Server struct {
	nativeTransports map[string]func() stun.NativeStatus
	addr             string
	registry         *world.Registry
	stunLink         *stun.Link
	srv              *http.Server
}

func NewServer(addr string, reg *world.Registry, stunLink *stun.Link) *Server {
	s := &Server{
		addr:     addr,
		registry: reg,
		stunLink: stunLink,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/state/file1", s.handleState(world.GameFile1))
	mux.HandleFunc("/state/file2", s.handleState(world.GameFile2))
	s.srv = &http.Server{Addr: addr, Handler: mux}
	return s
}

func (s *Server) Start() error {
	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	go func() {
		log.Printf("[nm.httpapi] listening on %s", s.addr)
		if err := s.srv.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Printf("[nm.httpapi] stopped: %v", err)
		}
	}()
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}

// SetNativeTransports is called before Start, separately from the RFC probe.
func (s *Server) SetNativeTransports(sources map[string]func() stun.NativeStatus) {
	s.nativeTransports = make(map[string]func() stun.NativeStatus)
	for name, source := range sources {
		s.nativeTransports[name] = source
	}
}
func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	native := map[string]stun.NativeStatus{}
	for name, source := range s.nativeTransports {
		native[name] = source()
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"native_udp":                  native,
		"status":                      "degraded",
		"ready":                       false,
		"native_authentication_ready": false,
		"game_relay_ready":            false,
		"stun":                        s.stunLink.Status(),
	})
}

func (s *Server) handleState(game world.Game) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		rooms := s.registry.ListRooms(game)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"game":  game,
			"rooms": rooms,
		})
	}
}
