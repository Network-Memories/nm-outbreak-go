package nativeweb

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"networkmemories/nm-outbreak-go/internal/core"
	"networkmemories/nm-outbreak-go/internal/identity"
	"networkmemories/nm-outbreak-go/internal/modules/file1"
	"networkmemories/nm-outbreak-go/internal/modules/file2"
	"networkmemories/nm-outbreak-go/internal/world"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestEnterAreasWaitsForNativeHTTP10PostBody(t *testing.T) {
	e := core.New(nil, nil, file1.Definition(), file2.Definition())
	web := httptest.NewServer(NewServer("", e, nil).server.Handler)
	defer web.Close()
	conn, err := net.Dial("tcp", web.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	const body = "native=1"
	if _, err := fmt.Fprintf(conn, "POST /00000002/enterareas HTTP/1.0\r\nContent-Length: %d\r\n\r\n", len(body)); err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(conn)
	conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	if _, err := reader.Peek(1); err == nil {
		t.Fatal("sent directory response before reading the POST body")
	} else if timeout, ok := err.(net.Error); !ok || !timeout.Timeout() {
		t.Fatal(err)
	}
	conn.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := io.WriteString(conn, body); err != nil {
		t.Fatal(err)
	}
	response, err := http.ReadResponse(reader, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil || response.StatusCode != http.StatusOK || !strings.Contains(string(data), "WEST TOWN") {
		t.Fatalf("unexpected native directory response: status %d, error %v", response.StatusCode, err)
	}
}

func TestNativeWebProofAndCRSTicketForBothGames(t *testing.T) {
	store, err := identity.OpenLocal(filepath.Join(t.TempDir(), "accounts.json"))
	if err != nil {
		t.Fatal(err)
	}
	account, err := store.Register("NativeOne", "nativepwd")
	if err != nil {
		t.Fatal(err)
	}
	e := core.New(nil, store, file1.Definition(), file2.Definition())
	web := httptest.NewServer(NewServer("", e, store))
	defer web.Close()
	for _, code := range []string{"00000002", "00000010"} {
		response, err := http.PostForm(web.URL+"/"+code+"/login-form", url.Values{"username": {"NativeOne"}, "password": {"nativepwd"}})
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(response.Body)
		response.Body.Close()
		match := regexp.MustCompile(`sessid=([0-9]{8})`).FindSubmatch(body)
		if len(match) != 2 {
			t.Fatal("proved native ticket missing", string(body))
		}
		ticket := string(match[1])
		game := world.GameFile1
		if code == "00000010" {
			game = world.GameFile2
		}
		proved, err := e.KDDIHandoff(game, ticket)
		if err != nil || proved.Account.UserID != account.UserID || proved.Account.Handle != account.Handle {
			t.Fatal("ticket lost proved owner/HN", err)
		}
		response, err = http.Get(web.URL + "/" + code + "/startsession?sessid=" + ticket)
		if err != nil {
			t.Fatal(err)
		}
		body, _ = io.ReadAll(response.Body)
		response.Body.Close()
		if !strings.Contains(string(body), `<CSV>"OK","`+ticket+`"`) || !strings.Contains(string(body), "CRS-game-start") {
			t.Fatal("native CSV handoff absent")
		}
		other := world.GameFile1
		if game == other {
			other = world.GameFile2
		}
		if _, err = e.KDDIHandoff(other, ticket); err == nil {
			t.Fatal("ticket crossed games")
		}
	}
	response, err := http.PostForm(web.URL+"/00000002/login-form", url.Values{"username": {"NativeOne"}, "password": {"wrongproof"}})
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if strings.Contains(string(body), "sessid=") {
		t.Fatal("failed proof issued ticket")
	}
	response, _ = http.Get(web.URL + "/00000002/startsession?sessid=12345678")
	body, _ = io.ReadAll(response.Body)
	response.Body.Close()
	if strings.Contains(string(body), "<CSV>") {
		t.Fatal("unproved ticket reached native client")
	}
}

func TestOriginalCRSEntryAndLobbyCatalogUseConfiguredTransport(t *testing.T) {
	store, err := identity.OpenLocal(filepath.Join(t.TempDir(), "accounts.json"))
	if err != nil {
		t.Fatal(err)
	}
	store.Register("NativeOne", "nativepwd")
	e := core.New(nil, store, file1.Definition(), file2.Definition())
	web := httptest.NewServer(NewServer("", e, store).WithLobbyPorts(31300, 31200))
	defer web.Close()
	for _, tc := range []struct {
		code, port string
		game       world.Game
		entries    int
	}{{"00000002", "31300", world.GameFile1, 2}, {"00000010", "31200", world.GameFile2, 1}} {
		response, err := http.Get(web.URL + "/" + tc.code + "/CRS-top.jsp")
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := io.ReadAll(response.Body)
		response.Body.Close()
		if response.StatusCode != 200 || !strings.Contains(string(raw), "url=/"+tc.code+"/login") {
			t.Fatal("original entry returned blank/404", response.StatusCode, string(raw))
		}
		ticket, err := e.LoginKDDI(tc.game, "NativeOne", "nativepwd")
		if err != nil {
			t.Fatal(err)
		}
		if e.KDDIConnected(tc.game) != 0 {
			t.Fatal("web ticket invented a connected native lobby")
		}
		proved, _ := e.KDDIHandoff(tc.game, ticket)
		if err = e.AttachKDDILobby(proved.ID); err != nil {
			t.Fatal(err)
		}
		response, err = http.Get(web.URL + "/" + tc.code + "/enterareas")
		if err != nil {
			t.Fatal(err)
		}
		raw, _ = io.ReadAll(response.Body)
		response.Body.Close()
		body := string(raw)
		if !strings.Contains(body, "CRS-lbs-info-get") || strings.Count(body, "www01.kddi-mmbb.jp:"+tc.port) != tc.entries || strings.Count(body, "\n\"1\",\n\"4096\",") != tc.entries {
			t.Fatal("native catalog differs from configured transport/presence", body)
		}
		e.DetachKDDILobby(proved.ID)
		if e.KDDIConnected(tc.game) != 0 {
			t.Fatal("closed native transport counted")
		}
	}
}

func TestOriginalPostLoginReturnOnlyDisplaysFormWithoutNewIdentity(t *testing.T) {
	store, _ := identity.OpenLocal(filepath.Join(t.TempDir(), "accounts.json"))
	e := core.New(nil, store, file1.Definition(), file2.Definition())
	web := httptest.NewServer(NewServer("", e, store))
	defer web.Close()
	r, err := http.PostForm(web.URL+"/00000010/login", url.Values{"username": {"UnprovedName"}, "result": {"disconnected"}})
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(r.Body)
	r.Body.Close()
	if r.StatusCode != 200 || !strings.Contains(string(body), "/00000010/login-form") || strings.Contains(string(body), "Connected.") || len(e.Sessions()) != 0 {
		t.Fatal("return path promoted posted metadata to identity", r.StatusCode)
	}
}
