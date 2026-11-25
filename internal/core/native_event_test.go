package core

import (
	"bytes"
	"networkmemories/nm-outbreak-go/internal/modules/file1"
	"networkmemories/nm-outbreak-go/internal/world"
	"testing"
)

func TestKDDIEventRelaysVerbatimOnlyBetweenJapaneseMembersOfOneWaitingRoom(t *testing.T) {
	e := New(nil, testAuthority{}, file1.Definition())
	h := actor(t, e, world.GameFile1, KDDI, "ntsc-j", "japanese")
	g := actor(t, e, world.GameFile1, KDDI, "ntsc-j", "japanese2")
	w := actor(t, e, world.GameFile1, SNAP, "ntsc-u", "western")
	for id, handle := range map[string]string{h.ID: "HOST01", g.ID: "GUEST2", w.ID: "WEST03"} {
		e.sessions[id].Account.Handle = handle
	}
	h.Account.Handle, g.Account.Handle, w.Account.Handle = "HOST01", "GUEST2", "WEST03"
	r, err := e.CreateRoom(h.ID, RoomOptions{Area: 1, Scenario: 1, MaxPlayers: 3})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{g.ID, w.ID} {
		if _, err := e.JoinRoom(id, r.ID); err != nil {
			t.Fatal(err)
		}
	}
	hs, _ := e.SubscribeRoom(h.ID)
	e.SubscribeRoom(g.ID)
	event := []byte{0, 0, 0, 0, 0, 0, 0, 0}
	if err := e.RelayKDDIEvent(g.ID, h.Account.Handle, event); err != nil {
		t.Fatal(err)
	}
	ev := <-hs.Events()
	if ev.Kind != RoomNativeEvent || ev.Actor.ID != g.ID || !bytes.Equal(ev.Journal, event) {
		t.Fatalf("%+v", ev)
	}
	event[0] = 9
	if ev.Journal[0] != 0 {
		t.Fatal("event shares caller memory")
	}
	if e.RelayKDDIEvent(g.ID, w.Account.Handle, event) == nil {
		t.Fatal("western recipient must keep its measured path")
	}
	if e.RelayKDDIEvent(g.ID, g.Account.Handle, event) == nil {
		t.Fatal("self relay")
	}
	if e.RelayKDDIEvent(w.ID, h.Account.Handle, event) == nil {
		t.Fatal("western sender")
	}
	e.LeaveRoom(g.ID)
	if e.RelayKDDIEvent(g.ID, h.Account.Handle, event) == nil {
		t.Fatal("departed sender")
	}
}
