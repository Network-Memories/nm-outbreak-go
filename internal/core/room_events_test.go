package core

import (
	"errors"
	"networkmemories/nm-outbreak-go/internal/modules/file2"
	"networkmemories/nm-outbreak-go/internal/world"
	"testing"
)

func eventFixture(t *testing.T) (*Engine, Session, Session, *world.Room, *RoomSubscription) {
	t.Helper()
	e := New(nil, testAuthority{}, file2.Definition())
	host := actor(t, e, world.GameFile2, SNAP, "pal", "western")
	guest := actor(t, e, world.GameFile2, KDDI, "ntsc-j", "japanese")
	e.BindSNAPID(host.ID, 42)
	room, err := e.CreateRoom(host.ID, RoomOptions{Area: 1, Scenario: 1, MaxPlayers: 4})
	if err != nil {
		t.Fatal(err)
	}
	sub, err := e.SubscribeRoom(host.ID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { e.UnsubscribeRoom(host.ID, sub) })
	if _, err = e.JoinRoom(guest.ID, room.ID); err != nil {
		t.Fatal(err)
	}
	return e, host, guest, room, sub
}
func TestSharedRoomEventsSurviveLogoutAndDoNotAliasState(t *testing.T) {
	e, _, guest, room, sub := eventFixture(t)
	joined := <-sub.Events()
	if joined.Kind != RoomJoined || joined.Count != 2 || joined.Actor.ID != guest.ID || joined.Actor.WireID == 0 {
		t.Fatal("missing proved native join", joined)
	}
	wire := joined.Actor.WireID
	joined.Actor.Profile[200] = 99
	joined.Room.Members[1].Profile[200] = 98
	state, _ := e.Session(guest.ID)
	r, _ := e.Worlds().GetRoom(room.ID)
	if state.Profile[200] == 99 || r.Members[1].Profile[200] == 98 {
		t.Fatal("event recipient modified authoritative state")
	}
	e.Logout(guest.ID)
	left := <-sub.Events()
	if left.Kind != RoomLeft || left.Count != 1 || left.Actor.WireID != wire || left.Actor.Account.UserID != guest.Account.UserID {
		t.Fatal("logout lost actor identity", left)
	}
	if _, ok := e.Session(guest.ID); ok {
		t.Fatal("logout not complete")
	}
	select {
	case ev := <-sub.Events():
		t.Fatal("duplicate membership event", ev)
	default:
	}
}
func TestSlowRoomObserverRefusesMutationsButAllowsDeparture(t *testing.T) {
	e, _, guest, room, sub := eventFixture(t)
	<-sub.Events()
	for i := 0; i < roomEventLimit; i++ {
		p := make([]byte, 208)
		p[0] = byte(i)
		if err := e.SetProfile(guest.ID, p); err != nil {
			t.Fatal(err)
		}
	}
	p := make([]byte, 208)
	p[0] = 250
	if err := e.SetProfile(guest.ID, p); !errors.Is(err, ErrRoomBackpressure) {
		t.Fatal("slow observer did not reject profile update", err)
	}
	snapshot, _ := e.Session(guest.ID)
	if snapshot.Profile[0] == 250 {
		t.Fatal("refused profile was committed")
	}
	third := actor(t, e, world.GameFile2, KDDI, "ntsc-j", "third")
	if _, err := e.JoinRoom(third.ID, room.ID); !errors.Is(err, ErrRoomBackpressure) {
		t.Fatal("congested admission committed", err)
	}
	if err := e.LeaveRoom(guest.ID); err != nil {
		t.Fatal("congested peer prevented explicit departure", err)
	}
	for i := 0; i < roomEventLimit; i++ {
		if ev := <-sub.Events(); ev.Kind != RoomProfileChanged {
			t.Fatal("event order lost")
		}
	}
	if ev := <-sub.Events(); ev.Kind != RoomLeft || ev.Count != 1 || sub.Overflowed() {
		t.Fatal("reserved departure event lost", ev)
	}
}
func TestScenarioReceiversDoNotGetLobbyCallbacks(t *testing.T) {
	e, host, guest, _, sub := eventFixture(t)
	<-sub.Events()
	if err := e.ScenarioPresence(host.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := e.SetProfile(guest.ID, make([]byte, 208)); err != nil {
		t.Fatal(err)
	}
	select {
	case event := <-sub.Events():
		t.Fatal("lobby callback entered game overlay", event)
	default:
	}
	e.LeaveRoom(guest.ID)
	select {
	case event := <-sub.Events():
		t.Fatal("departure callback entered game overlay", event)
	default:
	}
}
