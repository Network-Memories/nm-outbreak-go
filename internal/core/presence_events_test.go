package core

import (
	"networkmemories/nm-outbreak-go/internal/modules/file2"
	"networkmemories/nm-outbreak-go/internal/world"
	"testing"
)

func TestCrossHallNotificationsFollowActualNavigationAndDetach(t *testing.T) {
	e := New(nil, testAuthority{}, file2.Definition())
	p := actor(t, e, world.GameFile2, SNAP, "pal", "western")
	j := actor(t, e, world.GameFile2, KDDI, "ntsc-j", "japanese")
	e.SetArea(p.ID, 1)
	e.SetNativeLobby(p.ID, 1)
	ps, _ := e.SubscribeRoom(p.ID)
	e.AttachKDDILobby(j.ID)
	e.SetKDDIBrowse(j.ID, 1, 0)
	next := func(delta int) RoomEvent {
		t.Helper()
		select {
		case ev := <-ps.Events():
			if ev.Kind != HallPopulationChanged || ev.PopulationArea != 1 || ev.Count != delta || !e.HallPopulationDeliverable(p.ID, ev) {
				t.Fatalf("bad presence event: %+v", ev)
			}
			return ev
		default:
			t.Fatal("missing native presence change")
			return RoomEvent{}
		}
	}
	next(1)
	e.SetKDDIBrowse(j.ID, 1, 0)
	if len(ps.Events()) != 0 {
		t.Fatal("repeated navigation duplicated count")
	}
	e.SetKDDIBrowse(j.ID, 1, 1)
	next(-1)
	e.SetKDDIBrowse(j.ID, 1, 0)
	next(1)
	e.DetachKDDILobby(j.ID)
	next(-1)
	e.Logout(j.ID)
	if len(ps.Events()) != 0 {
		t.Fatal("logout counted detached actor twice")
	}
}

func TestHallNotificationsExpireForRecipientButKeepSourceDeparture(t *testing.T) {
	e := New(nil, testAuthority{}, file2.Definition())
	p := actor(t, e, world.GameFile2, SNAP, "pal", "western")
	j := actor(t, e, world.GameFile2, KDDI, "ntsc-j", "japanese")
	e.SetArea(p.ID, 1)
	e.SetNativeLobby(p.ID, 1)
	e.AttachKDDILobby(j.ID)
	e.SetKDDIBrowse(j.ID, 1, 0)
	ps, _ := e.SubscribeRoom(p.ID)
	e.Logout(j.ID)
	ev := <-ps.Events()
	if !e.HallPopulationDeliverable(p.ID, ev) || ev.Count != -1 {
		t.Fatal("departed source invalidated necessary decrement")
	}
	e.SetArea(p.ID, 0)
	e.SetNativeLobby(p.ID, 0)
	e.SetArea(p.ID, 1)
	e.SetNativeLobby(p.ID, 1)
	if e.HallPopulationDeliverable(p.ID, ev) {
		t.Fatal("old delta replayed after recipient refreshed its hall")
	}
}

func TestSharedHallPopulationAcrossMixedScenario(t *testing.T) {
	e := New(nil, testAuthority{}, file2.Definition())
	host := actor(t, e, world.GameFile2, SNAP, "pal", "western")
	guest := actor(t, e, world.GameFile2, KDDI, "ntsc-j", "guest")
	observer := actor(t, e, world.GameFile2, KDDI, "ntsc-j", "observer")
	if err := e.SetArea(host.ID, 1); err != nil {
		t.Fatal(err)
	}
	if err := e.SetNativeLobby(host.ID, 1); err != nil {
		t.Fatal(err)
	}
	for _, s := range []Session{guest, observer} {
		if err := e.AttachKDDILobby(s.ID); err != nil {
			t.Fatal(err)
		}
		if err := e.SetKDDIBrowse(s.ID, 1, 0); err != nil {
			t.Fatal(err)
		}
	}
	if err := e.SetKDDIBrowse(guest.ID, 1, 1); err != nil {
		t.Fatal(err)
	}
	sub, err := e.SubscribeRoom(observer.ID)
	if err != nil {
		t.Fatal(err)
	}
	check := func(want uint16, delta int) {
		t.Helper()
		hall, _ := e.KDDIBrowsePopulation(world.GameFile2, 1)
		if hall != want {
			t.Fatalf("hall population = %d, want %d", hall, want)
		}
		select {
		case event := <-sub.Events():
			if delta == 0 || event.Count != delta || event.Actor.ID != host.ID || !e.HallPopulationDeliverable(observer.ID, event) {
				t.Fatalf("unexpected hall event kind=%d delta=%d, want delta=%d", event.Kind, event.Count, delta)
			}
		default:
			if delta != 0 {
				t.Fatalf("missing hall event delta=%d", delta)
			}
		}
	}
	check(2, 0)
	room, err := e.CreateRoom(host.ID, RoomOptions{Area: 1, Scenario: 1, MaxPlayers: 2})
	if err != nil {
		t.Fatal(err)
	}
	check(1, -1)
	if _, err := e.JoinRoom(guest.ID, room.ID); err != nil {
		t.Fatal(err)
	}
	for _, s := range []Session{host, guest} {
		if err := e.ScenarioPresence(s.ID, true); err != nil {
			t.Fatal(err)
		}
		check(1, 0)
	}
	for i, s := range []Session{guest, host} {
		if err := e.ScenarioPresence(s.ID, false); err != nil {
			t.Fatal(err)
		}
		members, err := e.AftergameMembers(s.ID)
		if err != nil || len(members) != i+1 {
			t.Fatalf("aftergame return %d: members=%d err=%v", i+1, len(members), err)
		}
		check(1, 0)
	}
	if err := e.LeaveRoom(guest.ID); err != nil {
		t.Fatal(err)
	}
	check(1, 0)
	if err := e.LeaveRoom(host.ID); err != nil {
		t.Fatal(err)
	}
	check(2, 1)
}
