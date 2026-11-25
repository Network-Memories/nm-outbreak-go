package core

import (
	"networkmemories/nm-outbreak-go/internal/modules/file1"
	"networkmemories/nm-outbreak-go/internal/world"
	"testing"
	"time"
)

func TestSilentSessionIsReleasedButLiveOnesStay(t *testing.T) {
	e := New(nil, testAuthority{}, file1.Definition())
	quiet := actor(t, e, world.GameFile1, SNAP, "ntsc-u", "western")
	live := actor(t, e, world.GameFile1, KDDI, "ntsc-j", "japanese")
	lobby := actor(t, e, world.GameFile1, KDDI, "ntsc-j", "japanese3")
	if err := e.AttachKDDILobby(lobby.ID); err != nil {
		t.Fatal(err)
	}
	room, _ := e.CreateRoom(quiet.ID, RoomOptions{Area: 1, Scenario: 1, MaxPlayers: 3})
	later := time.Now().Add(SessionSilenceLimit + time.Second)
	e.mu.Lock()
	e.lastAlive[live.ID] = later
	e.mu.Unlock()
	reaped := e.ReapSilent(later)
	if len(reaped) != 1 || reaped[0] != quiet.ID {
		t.Fatalf("reaped %v", reaped)
	}
	if _, ok := e.worlds.GetRoom(room.ID); ok {
		t.Fatal("ghost room left behind")
	}
	for _, id := range []string{live.ID, lobby.ID} {
		if _, ok := e.Session(id); !ok {
			t.Fatal("live session released")
		}
	}
}

func TestRoundMemberKeepsItsSeatWhileReadingResults(t *testing.T) {
	e := New(nil, testAuthority{}, file1.Definition())
	h := actor(t, e, world.GameFile1, SNAP, "ntsc-u", "western")
	g := actor(t, e, world.GameFile1, KDDI, "ntsc-j", "japanese")
	room, _ := e.CreateRoom(h.ID, RoomOptions{Area: 1, Scenario: 1, MaxPlayers: 2})
	if _, err := e.JoinRoom(g.ID, room.ID); err != nil {
		t.Fatal(err)
	}
	r, _ := e.worlds.GetRoom(room.ID)
	if _, err := e.scenarioRoundLocked(r); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{h.ID, g.ID} {
		if err := e.ScenarioPresence(id, true); err != nil {
			t.Fatal(err)
		}
	}
	if reaped := e.ReapSilent(time.Now().Add(10 * time.Minute)); len(reaped) != 0 {
		t.Fatalf("round members released after the game: %v", reaped)
	}
	if reaped := e.ReapSilent(time.Now().Add(RoundSilenceLimit + time.Minute)); len(reaped) != 2 {
		t.Fatalf("vanished round members kept: %v", reaped)
	}
}

func TestUsedJapaneseTicketOutlivesLongScenario(t *testing.T) {
	e := New(nil, testAuthority{}, file1.Definition())
	ticket, err := e.LoginKDDI(world.GameFile1, "japanese", "fixture-proof")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.KDDIHandoff(world.GameFile1, ticket); err != nil {
		t.Fatal(err)
	}
	e.mu.Lock()
	tk := e.kddiTickets[ticket]
	tk.expires = time.Now().Add(-time.Hour)
	e.kddiTickets[ticket] = tk
	e.mu.Unlock()
	s, err := e.KDDIHandoff(world.GameFile1, ticket)
	if err != nil {
		t.Fatal("after-game reconnection refused once the login is old", err)
	}
	e.Logout(s.ID)
	if _, err := e.KDDIHandoff(world.GameFile1, ticket); err == nil {
		t.Fatal("ticket outlived its session")
	}
	unused, _ := e.LoginKDDI(world.GameFile1, "japanese2", "fixture-proof")
	e.mu.Lock()
	tk = e.kddiTickets[unused]
	tk.expires = time.Now().Add(-time.Second)
	e.kddiTickets[unused] = tk
	e.mu.Unlock()
	if _, err := e.KDDIHandoff(world.GameFile1, unused); err == nil {
		t.Fatal("unused expired ticket accepted")
	}
}
