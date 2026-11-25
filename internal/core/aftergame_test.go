package core

import (
	"bytes"
	"networkmemories/nm-outbreak-go/internal/modules/file1"
	"networkmemories/nm-outbreak-go/internal/modules/file2"
	"networkmemories/nm-outbreak-go/internal/world"
	"testing"
)

func TestAftergameCountsOnlyActualReturnsAndKeepsOtherSeat(t *testing.T) {
	e := New(nil, testAuthority{}, file2.Definition())
	host := actor(t, e, world.GameFile2, SNAP, "pal", "western")
	guest := actor(t, e, world.GameFile2, KDDI, "ntsc-j", "japanese")
	room, err := e.CreateRoom(host.ID, RoomOptions{Area: 1, Scenario: 1, MaxPlayers: 2})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = e.JoinRoom(guest.ID, room.ID); err != nil {
		t.Fatal(err)
	}
	if e.HasScenarioRound(guest.ID) {
		t.Fatal("membership invented scenario entry")
	}
	if _, err = e.AftergameMembers(guest.ID); err == nil {
		t.Fatal("waiting counted as aftergame")
	}
	for _, id := range []string{host.ID, guest.ID} {
		if err = e.ScenarioPresence(id, true); err != nil {
			t.Fatal(err)
		}
	}
	e.ScenarioPresence(guest.ID, false)
	members, err := e.AftergameMembers(guest.ID)
	if err != nil || len(members) != 1 || members[0].ID != guest.ID {
		t.Fatal("first return", members, err)
	}
	frozen, _ := e.Worlds().GetRoom(room.ID)
	if len(frozen.Members) != 2 || !frozen.Playing {
		t.Fatal("other player's ongoing seat lost")
	}
	e.ScenarioPresence(host.ID, false)
	members, err = e.AftergameMembers(guest.ID)
	if err != nil || len(members) != 2 || members[0].ID != host.ID || members[1].ID != guest.ID {
		t.Fatal("mixed roster", members, err)
	}
	members[0].Profile[0] ^= 255
	fresh, _ := e.Session(host.ID)
	if members[0].Profile[0] == fresh.Profile[0] {
		t.Fatal("snapshot aliases profile")
	}
	if err = e.LeaveRoom(guest.ID); err != nil {
		t.Fatal(err)
	}
	members, err = e.AftergameMembers(host.ID)
	if err != nil || len(members) != 1 {
		t.Fatal("departure did not expire", members, err)
	}
	if _, err = e.AftergameMembers(guest.ID); err == nil {
		t.Fatal("departed peer still in aftergame")
	}
}

func TestAftergameReturnEventIsIdempotentAndDoesNotReachScenario(t *testing.T) {
	e := New(nil, testAuthority{}, file2.Definition())
	host := actor(t, e, world.GameFile2, SNAP, "pal", "western")
	guest := actor(t, e, world.GameFile2, KDDI, "ntsc-j", "japanese")
	room, err := e.CreateRoom(host.ID, RoomOptions{Area: 1, Scenario: 1, MaxPlayers: 2})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = e.JoinRoom(guest.ID, room.ID); err != nil {
		t.Fatal(err)
	}
	hs, _ := e.SubscribeRoom(host.ID)
	gs, _ := e.SubscribeRoom(guest.ID)
	e.ScenarioPresence(host.ID, true)
	e.ScenarioPresence(guest.ID, true)
	e.ScenarioPresence(guest.ID, false)
	select {
	case ev := <-hs.Events():
		t.Fatal("return entered scenario overlay", ev.Kind)
	default:
	}
	e.ScenarioPresence(host.ID, false)
	select {
	case ev := <-gs.Events():
		if ev.Kind != RoomAftergameReturned || ev.Actor.ID != host.ID {
			t.Fatal("wrong return event", ev.Kind)
		}
	default:
		t.Fatal("return not delivered")
	}
	e.ScenarioPresence(host.ID, false)
	select {
	case ev := <-gs.Events():
		t.Fatal("duplicate return event", ev.Kind)
	default:
	}
}

func TestKDDIReportRoundRequiresMatchingNativeRoundAndOwnEntry(t *testing.T) {
	e := New(nil, testAuthority{}, file2.Definition())
	a := actor(t, e, world.GameFile2, KDDI, "ntsc-j", "japanese")
	room, err := e.CreateRoom(a.ID, RoomOptions{Area: 1, Scenario: 1, MaxPlayers: 2})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = e.KDDIReportRound(a.ID, "000000000000001"); err == nil {
		t.Fatal("no actual entry")
	}
	e.ScenarioPresence(a.ID, true)
	round := e.sessionRounds[a.ID]
	if _, _, err = e.KDDIReportRound(a.ID, "wrong"); err == nil {
		t.Fatal("wrong native session accepted")
	}
	s, id, err := e.KDDIReportRound(a.ID, round.nativeNumber)
	if err != nil || id != round.id || s.Account.UserID != a.Account.UserID {
		t.Fatal("round identity", err)
	}
	e.ScenarioPresence(a.ID, false)
	if _, got, err := e.KDDIReportRound(a.ID, round.nativeNumber); err != nil || got != id {
		t.Fatal("return lost report context", err)
	}
	if room.ID == "" {
		t.Fatal("missing room")
	}
	e.Logout(a.ID)
	if _, _, err = e.KDDIReportRound(a.ID, round.nativeNumber); err == nil {
		t.Fatal("revoked session accepted")
	}
}

func TestAftergameMarkerRequiresActualJapaneseReturn(t *testing.T) {
	for _, game := range []world.Game{world.GameFile1, world.GameFile2} {
		e := New(nil, testAuthority{}, file1.Definition(), file2.Definition())
		region := "pal"
		if game == world.GameFile1 {
			region = "ntsc-u"
		}
		host := actor(t, e, game, SNAP, region, "western")
		guest := actor(t, e, game, KDDI, "ntsc-j", "japanese")
		room, err := e.CreateRoom(host.ID, RoomOptions{Area: 1, Scenario: 1, MaxPlayers: 2})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = e.JoinRoom(guest.ID, room.ID); err != nil {
			t.Fatal(err)
		}
		original := make([]byte, 240)
		original[17] = 99
		assertUnchanged := func(id string) {
			t.Helper()
			p, err := e.ProjectSNAPAftergameProfile(id, original)
			if err != nil || !bytes.Equal(p, original) {
				t.Fatal("unproved return changed profile", game, err)
			}
		}
		assertUnchanged(guest.ID)
		e.ScenarioPresence(host.ID, true)
		e.ScenarioPresence(guest.ID, true)
		assertUnchanged(guest.ID)
		e.ScenarioPresence(guest.ID, false)
		got, err := e.ProjectSNAPAftergameProfile(guest.ID, original)
		if err != nil {
			t.Fatal(err)
		}
		want := append([]byte(nil), original...)
		if game == world.GameFile2 {
			want[221] = 1
		} else {
			want[212] = 1 // File #1 marker measured 23/11/2026
		}
		if !bytes.Equal(got, want) || original[221] != 0 || original[212] != 0 {
			t.Fatal("marker modified unrelated bytes or caller", game)
		}
		e.ScenarioPresence(host.ID, false)
		assertUnchanged(host.ID)
		e.LeaveRoom(guest.ID)
		assertUnchanged(guest.ID)
	}
}
