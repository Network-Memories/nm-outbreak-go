package kddi

import (
	"bytes"
	"net"
	"networkmemories/nm-outbreak-go/internal/core"
	"networkmemories/nm-outbreak-go/internal/world"
	"testing"
	"time"
)

func TestNativeAftergameRequiresOwnEntryAndCountsMixedReturns(t *testing.T) {
	e := nativeEngine(nil)
	host, _ := e.Login(world.GameFile2, core.SNAP, "pal", "WesternHost", "nativeproof")
	ticket, _ := e.LoginKDDI(world.GameFile2, "NativeOne", "nativeproof")
	guest, _ := e.KDDIHandoff(world.GameFile2, ticket)
	e.SetProfile(host.ID, make([]byte, 240))
	e.SetProfile(guest.ID, make([]byte, 208))
	room, err := e.CreateRoom(host.ID, core.RoomOptions{Area: 1, Scenario: 1, MaxPlayers: 2})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = e.JoinRoom(guest.ID, room.ID); err != nil {
		t.Fatal(err)
	}
	srv := NewTransport("file2-lobby", world.GameFile2, "127.0.0.1", 0, e)
	query := func(cmd uint16, payload []byte) LobbyFrame {
		t.Helper()
		a, b := net.Pipe()
		defer a.Close()
		defer b.Close()
		b.SetDeadline(time.Now().Add(time.Second))
		done := make(chan bool, 1)
		go func() {
			done <- srv.handleAftergame(a, LobbyFrame{Sender: 0x81, Kind: 1, Command: cmd, PacketID: 42, Payload: payload}, guest.ID)
		}()
		r, err := readFrame(b)
		if err != nil {
			t.Fatal(err)
		}
		if !<-done || r.Command != cmd || r.PacketID != 42 || r.Kind != 2 {
			t.Fatal("reply envelope", r)
		}
		return r
	}
	if query(0x6210, nil).Error != 0xff {
		t.Fatal("waiting membership invented aftergame")
	}
	e.ScenarioPresence(host.ID, true)
	e.ScenarioPresence(guest.ID, true)
	if query(0x6210, []byte{0}).Error != 0xff {
		t.Fatal("malformed enter accepted")
	}
	if r := query(0x6210, nil); r.Error != 0 || len(r.Payload) != 0 {
		t.Fatal("enter", r)
	}
	if r := query(0x6212, nil); r.Error != 0 || !bytes.Equal(r.Payload, []byte{0, 1}) {
		t.Fatal("ongoing peer counted", r)
	}
	e.ScenarioPresence(host.ID, false)
	if r := query(0x6212, nil); r.Error != 0 || !bytes.Equal(r.Payload, []byte{0, 2}) {
		t.Fatal("mixed count", r)
	}
	r := query(0x6213, nil)
	hp, _ := nativeMember(mustSession(t, e, host.ID))
	gp, _ := nativeMember(mustSession(t, e, guest.ID))
	want := append([]byte{0, 0, 3, 2}, hp...)
	want = append(want, gp...)
	if r.Error != 0 || !bytes.Equal(r.Payload, want) {
		t.Fatal("mixed native roster", r.Error, len(r.Payload))
	}
	if r = query(0x6211, nil); r.Error != 0 {
		t.Fatal("leave", r)
	}
	if query(0x6212, nil).Error != 0xff {
		t.Fatal("departed player counted")
	}
}
func mustSession(t *testing.T, e *core.Engine, id string) core.Session {
	t.Helper()
	s, ok := e.Session(id)
	if !ok {
		t.Fatal("session missing")
	}
	return s
}
