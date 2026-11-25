package core

import (
	"networkmemories/nm-outbreak-go/internal/modules/file1"
	"networkmemories/nm-outbreak-go/internal/modules/file2"
	"networkmemories/nm-outbreak-go/internal/results"
	"networkmemories/nm-outbreak-go/internal/world"
	"testing"
)

func TestMixedRoundResultsCommonLedgerAndNoReplayAward(t *testing.T) {
	e := New(nil, testAuthority{}, file1.Definition(), file2.Definition())
	l, _ := results.Open("")
	if e.ConfigureResults(l) != nil {
		t.Fatal("configure")
	}
	host := actor(t, e, world.GameFile2, KDDI, "ntsc-j", "japanese")
	guest := actor(t, e, world.GameFile2, SNAP, "pal", "western")
	room, err := e.CreateRoom(host.ID, RoomOptions{Area: 1, Scenario: 1, MaxPlayers: 4})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = e.JoinRoom(guest.ID, room.ID); err != nil {
		t.Fatal(err)
	}
	points := uint32(123)
	stat := results.Stats{Points: &points}
	if err = e.RecordResult(host.ID, stat); err == nil {
		t.Fatal("report before native start accepted")
	}
	for _, s := range []Session{host, guest} {
		if err = e.ScenarioPresence(s.ID, true); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 3; i++ {
			if err = e.RecordResult(s.ID, stat); err != nil {
				t.Fatal(err)
			}
		}
	}
	rows := l.Snapshot()
	playing, _ := e.Worlds().GetRoom(room.ID)
	if !playing.Playing {
		t.Fatal("native scenario entry still advertised as a waiting room")
	}
	if len(rows) != 2 || rows[0].RoundID != rows[1].RoundID || l.Points(world.GameFile2, 1) != 123 || l.Points(world.GameFile2, 2) != 123 {
		t.Fatal("mixed game duplicated or separated")
	}
	// Return of one client must not split the other client's run.
	if err = e.ScenarioPresence(host.ID, false); err != nil {
		t.Fatal(err)
	}
	playing, _ = e.Worlds().GetRoom(room.ID)
	if !playing.Playing {
		t.Fatal("first return reopened an ongoing mixed round")
	}
	if err = e.RecordResult(guest.ID, stat); err != nil || len(l.Snapshot()) != 2 {
		t.Fatal("return split mixed run", err)
	}
	e.ScenarioPresence(guest.ID, false)
	playing, _ = e.Worlds().GetRoom(room.ID)
	if playing.Playing {
		t.Fatal("all native returns left room advertised in scenario")
	}
	e.ScenarioPresence(host.ID, true)
	e.ScenarioPresence(guest.ID, true)
	e.RecordResult(host.ID, stat)
	e.RecordResult(guest.ID, stat)
	if len(l.Snapshot()) != 4 || l.Points(world.GameFile2, 1) != 246 {
		t.Fatal("second real round lost")
	}
	e.Logout(host.ID)
	if err = e.RecordResult(host.ID, stat); err == nil {
		t.Fatal("revoked session accepted")
	}
}
