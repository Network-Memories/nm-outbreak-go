// Native KDDI browser entry pages and CRS-game-start CSV follow the existing
// BioServer web LoginAction / startsession.html.twig (AGPL-3.0-or-later).
package nativeweb

import (
	"context"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"networkmemories/nm-outbreak-go/internal/core"
	"networkmemories/nm-outbreak-go/internal/identity"
	"networkmemories/nm-outbreak-go/internal/world"
)

type RegisterAuthority interface {
	Register(string, string) (*identity.Account, error)
}
type Server struct {
	lobbyPorts map[world.Game]int
	engine     *core.Engine
	authority  identity.Authority
	register   func(string, string) (*identity.Account, error)
	server     *http.Server
}

func NewServer(addr string, engine *core.Engine, authority identity.Authority) *Server {
	s := &Server{engine: engine, authority: authority, lobbyPorts: map[world.Game]int{world.GameFile1: 8300, world.GameFile2: 8200}}
	if registration, ok := authority.(RegisterAuthority); ok {
		s.register = registration.Register
	}
	s.server = &http.Server{Addr: addr, Handler: s, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16384}
	return s
}

// WithNMRegistration enables native signup after NM exposes the scoped operation.
func (s *Server) WithNMRegistration(enabled bool) *Server {
	if client, ok := s.authority.(*identity.Client); enabled && ok {
		s.register = client.RegisterKDDI
	}
	return s
}
func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.server.Addr)
	if err != nil {
		return err
	}
	go s.server.Serve(ln)
	return nil
}
func (s *Server) Shutdown(ctx context.Context) error { return s.server.Shutdown(ctx) }
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/html; charset=us-ascii")
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}
	game := world.GameFile1
	if parts[0] == "00000010" {
		game = world.GameFile2
	} else if parts[0] != "00000002" {
		http.NotFound(w, r)
		return
	}
	id := parts[0]
	switch parts[1] {
	case "CRS-top.jsp":
		if r.Method != http.MethodGet && r.Method != http.MethodPost {
			w.WriteHeader(405)
			return
		}
		fmt.Fprintf(w, `<html><head><meta http-equiv="Refresh" content="1; url=/%s/login"></head></html>`, id)
	case "enterareas":
		if r.Method != http.MethodGet && r.Method != http.MethodPost {
			w.WriteHeader(405)
			return
		}
		// The native HTTP/1.0 browser can send the POST body separately
		// after the headers. Read it before responding so the TLS bridge
		// cannot close the backend socket while the body is still in flight.
		if r.Method == http.MethodPost {
			body := http.MaxBytesReader(w, r.Body, 4096)
			defer body.Close()
			if _, err := io.Copy(io.Discard, body); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
		}
		fmt.Fprint(w, `<html><head><!--CRS-lbs-info-get--><META HTTP-EQUIV=Content-Type CONTENT=text/html;CHARSET=us-ascii></head><!--<CSV>`+"\n"+`"OK",`+"\n")
		for _, entry := range s.engine.KDDIDirectory(game).BrowserEntries {
			label := "0ad601082008," + entry.Label
			if entry.Area != 0 {
				label += "," + strconv.Itoa(int(entry.Area))
			}
			rows := []string{"www01.kddi-mmbb.jp:" + strconv.Itoa(s.lobbyPorts[game]), strconv.Itoa(s.engine.KDDIConnected(game)), "4096", label, "<BODY>" + entry.Info + "<END>", "  "}
			for _, row := range rows {
				fmt.Fprintf(w, "\"%s\",\n", row)
			}
		}
		fmt.Fprint(w, `</CSV>--></html>`)
	case "login":
		if r.Method != http.MethodGet && r.Method != http.MethodPost {
			w.WriteHeader(405)
			return
		}
		fmt.Fprintf(w, `<html><head><!--CRS-top--><title>Outbreak</title></head><body text="#D7E0FF" bgcolor="black" link="#7799CC"><form method="post" action="/%s/login-form">ID: <input name="username"><br>Password: <input type="password" name="password"><input type="hidden" name="login" value="manual"><input type="submit" value="Login"></form>`, id)
		if s.register != nil {
			fmt.Fprintf(w, `<form method="post" action="/%s/login-form">New ID: <input name="username"><br>Password (5 to 10 characters): <input type="password" name="password" minlength="5" maxlength="10"><input type="hidden" name="login" value="newaccount"><input type="submit" value="Create account"></form>`, id)
		}
		fmt.Fprint(w, "</body></html>")
	case "login-form":
		if r.Method != http.MethodPost {
			w.WriteHeader(405)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		if err := r.ParseForm(); err != nil {
			w.WriteHeader(400)
			return
		}
		username, password := r.PostForm.Get("username"), r.PostForm.Get("password")
		if len(username) > 128 || len(password) > 128 {
			w.WriteHeader(400)
			return
		}
		if r.PostForm.Get("login") == "newaccount" {
			if s.register == nil {
				w.WriteHeader(403)
				return
			}
			if _, err := s.register(username, password); err != nil {
				s.failed(w, id)
				return
			}
		}
		ticket, err := s.engine.LoginKDDI(game, username, password)
		if err != nil {
			s.failed(w, id)
			return
		}
		fmt.Fprintf(w, `<html><head><!--CRS-top--></head><body text="#D7E0FF" bgcolor="black" link="#7799CC">Connected.<br><a href="/%s/startsession?sessid=%s">Enter lobby</a></body></html>`, id, ticket)
	case "startsession":
		if r.Method != http.MethodGet {
			w.WriteHeader(405)
			return
		}
		ticket := r.URL.Query().Get("sessid")
		if _, err := s.engine.KDDIHandoff(game, ticket); err != nil {
			s.failed(w, id)
			return
		}
		// All interpolated values are fixed game codes or validated decimal tickets.
		fmt.Fprintf(w, `<html><head><!--CRS-game-start--><META HTTP-EQUIV=Content-Type CONTENT=text/html;CHARSET=EUC-JP></head><!--result--><!--connection id--><!--start the game url--><!--exit game url--><!--<CSV>"OK","%s","https://www01.kddi-mmbb.jp/%s/enterareas","https://www01.kddi-mmbb.jp/%s/login",</CSV>--></html>`, ticket, id, id)
	default:
		http.NotFound(w, r)
	}
}
func (s *Server) failed(w http.ResponseWriter, id string) {
	fmt.Fprintf(w, `<html><body text="#D7E0FF" bgcolor="black" link="#7799CC">Login failed.<br><a href="/%s/login">Return</a></body></html>`, template.HTMLEscapeString(id))
}

// WithLobbyPorts is configured before listeners start. Browser and TCP transport
// use the same configured ports rather than a second hard-coded deployment map.
func (s *Server) WithLobbyPorts(file1, file2 int) *Server {
	s.lobbyPorts = map[world.Game]int{world.GameFile1: file1, world.GameFile2: file2}
	return s
}
