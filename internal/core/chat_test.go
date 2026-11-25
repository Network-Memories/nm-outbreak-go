package core

import (
	"networkmemories/nm-outbreak-go/internal/modules/file1"
	"networkmemories/nm-outbreak-go/internal/modules/file2"
	"networkmemories/nm-outbreak-go/internal/world"
	"testing"
)

func TestNativeChatRoomScopeEchoAndActualAftergameCohort(t *testing.T) {
	e := New(nil, testAuthority{}, file1.Definition(), file2.Definition())
	h := actor(t, e, world.GameFile2, SNAP, "pal", "western")
	g := actor(t, e, world.GameFile2, KDDI, "ntsc-j", "japanese")
	other := actor(t, e, world.GameFile2, KDDI, "ntsc-j", "outsider")
	r, _ := e.CreateRoom(h.ID, RoomOptions{Area: 1, Scenario: 1, MaxPlayers: 2})
	e.JoinRoom(g.ID, r.ID)
	e.CreateRoom(other.ID, RoomOptions{Area: 1, Scenario: 1, MaxPlayers: 2})
	hs, _ := e.SubscribeRoom(h.ID)
	gs, _ := e.SubscribeRoom(g.ID)
	os, _ := e.SubscribeRoom(other.ID)
	if err := e.PublishRoomChat(h.ID, false, "hello"); err != nil {
		t.Fatal(err)
	}
	if len(hs.events) != 0 || len(os.events) != 0 {
		t.Fatal("local SNAP echo or another room received text")
	}
	ev := <-gs.events
	if ev.Kind != RoomChat || ev.ChatText != "hello" || ev.Actor.ID != h.ID || !e.RoomChatDeliverable(g.ID, ev) {
		t.Fatal("wrong authenticated sender")
	}
	forged := ev
	forged.Actor.Account.UserID++
	if e.RoomChatDeliverable(g.ID, forged) {
		t.Fatal("forged owner accepted")
	}
	if err := e.PublishRoomChat(g.ID, false, "ｃｃｃｃｃ"); err != nil {
		t.Fatal(err)
	}
	for _, sub := range []*RoomSubscription{hs, gs} {
		got := <-sub.events
		if got.Actor.ID != g.ID || got.ChatText != "ｃｃｃｃｃ" {
			t.Fatal("Unicode text or native local echo lost")
		}
	}
	e.ScenarioPresence(h.ID, true)
	e.ScenarioPresence(g.ID, true)
	if err := e.PublishRoomChat(h.ID, false, "overlay"); err == nil {
		t.Fatal("chat entered scenario")
	}
	e.ScenarioPresence(g.ID, false)
	if err := e.PublishRoomChat(g.ID, true, "alone"); err != nil {
		t.Fatal(err)
	}
	if len(hs.events) != 0 {
		t.Fatal("aftergame chat entered ongoing peer scenario")
	}
	<-gs.events
	e.ScenarioPresence(h.ID, false)
	for len(gs.events) > 0 {
		<-gs.events
	}
	if err := e.PublishRoomChat(g.ID, false, "wrong context"); err == nil {
		t.Fatal("waiting chat accepted in actual aftergame")
	}
	if err := e.PublishRoomChat(g.ID, true, "returned"); err != nil {
		t.Fatal(err)
	}
	ev = <-hs.events
	if !e.RoomChatDeliverable(h.ID, ev) {
		t.Fatal("same frozen round not admitted")
	}
	e.LeaveRoom(g.ID)
	if e.RoomChatDeliverable(h.ID, ev) {
		t.Fatal("queued text survived sender departure")
	}
	if err := e.PublishRoomChat(g.ID, true, "departed"); err == nil {
		t.Fatal("departed sender admitted")
	}
}

func TestNativeRoomChatBackpressureIsAtomicAndFile1RemainsUnqualified(t *testing.T) {
	e := New(nil, testAuthority{}, file1.Definition(), file2.Definition())
	h := actor(t, e, world.GameFile2, SNAP, "pal", "western")
	g := actor(t, e, world.GameFile2, KDDI, "ntsc-j", "japanese")
	r, _ := e.CreateRoom(h.ID, RoomOptions{Area: 1, Scenario: 1, MaxPlayers: 2})
	e.JoinRoom(g.ID, r.ID)
	hs, _ := e.SubscribeRoom(h.ID)
	gs, _ := e.SubscribeRoom(g.ID)
	for len(gs.events) < roomEventLimit {
		gs.events <- RoomEvent{}
	}
	if err := e.PublishRoomChat(g.ID, false, "bounded"); err != ErrRoomBackpressure {
		t.Fatal(err)
	}
	if len(hs.events) != 0 {
		t.Fatal("partial publication before congestion check")
	}
	if err := e.PublishRoomChat(h.ID, false, "bad\ncontrol"); err == nil {
		t.Fatal("control injection accepted")
	}
	if err := e.PublishRoomChat("missing", false, "hello"); err == nil {
		t.Fatal("missing identity accepted")
	}
	f := actor(t, e, world.GameFile1, SNAP, "ntsc-u", "fileone")
	if err := e.PublishRoomChat(f.ID, false, "unmeasured"); err == nil {
		t.Fatal("File2 chat generalized to File1")
	}
}
