package httpapi

import (
	"encoding/json"
	"net"
	"net/http/httptest"
	"networkmemories/nm-outbreak-go/internal/stun"
	"networkmemories/nm-outbreak-go/internal/world"
	"testing"
	"time"
)

func TestHealthDoesNotClaimNativeReadiness(t *testing.T) {
	s := NewServer("127.0.0.1:0", world.NewRegistry(), stun.NewLink("127.0.0.1:1", time.Second, time.Millisecond))
	r := httptest.NewRecorder()
	s.srv.Handler.ServeHTTP(r, httptest.NewRequest("GET", "/health", nil))
	var data struct {
		Status string `json:"status"`
		Ready  bool   `json:"ready"`
		STUN   struct {
			RelayEnabled bool `json:"relay_enabled"`
		} `json:"stun"`
	}
	if err := json.Unmarshal(r.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	if r.Code != 200 || data.Status != "degraded" || data.Ready || data.STUN.RelayEnabled {
		t.Fatal("prototype claims a working native relay")
	}
}
func TestStateSeparatesGamesAndDoesNotExposeProfiles(t *testing.T) {
	reg := world.NewRegistry()
	reg.CreateRoom(world.Room{Game: world.GameFile1, Members: []world.Member{{Name: "RealReceivedName", Profile: []byte{1, 2}}}})
	s := NewServer("127.0.0.1:0", reg, stun.NewLink("127.0.0.1:1", time.Second, time.Millisecond))
	for _, entry := range []struct {
		Path  string
		Count int
	}{{"/state/file1", 1}, {"/state/file2", 0}} {
		r := httptest.NewRecorder()
		s.srv.Handler.ServeHTTP(r, httptest.NewRequest("GET", entry.Path, nil))
		var data struct {
			Rooms []struct{ Members []map[string]any }
		}
		if err := json.Unmarshal(r.Body.Bytes(), &data); err != nil {
			t.Fatal(err)
		}
		if len(data.Rooms) != entry.Count {
			t.Fatal("game room leaked across modules")
		}
		if len(data.Rooms) > 0 {
			if _, ok := data.Rooms[0].Members[0]["Profile"]; ok {
				t.Fatal("opaque profile exposed")
			}
			if _, ok := data.Rooms[0].Members[0]["profile"]; ok {
				t.Fatal("opaque profile exposed")
			}
		}
	}
}
func TestOccupiedHTTPPortFailsStartup(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	s := NewServer(ln.Addr().String(), world.NewRegistry(), stun.NewLink("127.0.0.1:1", time.Second, time.Millisecond))
	if err := s.Start(); err == nil {
		t.Fatal("HTTP bind failure hidden in background goroutine")
	}
}
