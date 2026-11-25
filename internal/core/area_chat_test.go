package core

import (
	"networkmemories/nm-outbreak-go/internal/modules/file1"
	"networkmemories/nm-outbreak-go/internal/modules/file2"
	"networkmemories/nm-outbreak-go/internal/world"
	"testing"
)

func TestAreaChatNativeScopeAndNavigationExpiry(t *testing.T) {
	def := file2.Definition()
	def.NativeChatAreas = []uint32{1, 2} // second test scope, not a production declaration
	e := New(nil, testAuthority{}, file1.Definition(), def)
	def.NativeChatAreas[0] = 99 // engine must own its module description
	h := actor(t, e, world.GameFile2, SNAP, "pal", "western")
	g := actor(t, e, world.GameFile2, KDDI, "ntsc-j", "japanese")
	other := actor(t, e, world.GameFile2, KDDI, "ntsc-j", "otherarea")
	browser := actor(t, e, world.GameFile2, KDDI, "ntsc-j", "slotbrowser")
	detached := actor(t, e, world.GameFile2, KDDI, "ntsc-j", "detached")
	e.SetArea(h.ID, 1)
	e.SetNativeLobby(h.ID, 1)
	for _, s := range []Session{g, other, browser} {
		if err := e.AttachKDDILobby(s.ID); err != nil {
			t.Fatal(err)
		}
	}
	e.SetKDDIBrowse(g.ID, 1, 0)
	e.SetKDDIBrowse(other.ID, 2, 0)
	e.SetKDDIBrowse(browser.ID, 1, 1)
	e.SetKDDIBrowse(detached.ID, 1, 0)
	hs, _ := e.SubscribeRoom(h.ID)
	gs, _ := e.SubscribeRoom(g.ID)
	os, _ := e.SubscribeRoom(other.ID)
	bs, _ := e.SubscribeRoom(browser.ID)
	ds, _ := e.SubscribeRoom(detached.ID)
	if !e.AreaChatSupported(h.ID) || !e.AreaChatSupported(g.ID) || e.AreaChatSupported(browser.ID) || e.AreaChatSupported(detached.ID) {
		t.Fatal("native area cohort wrong")
	}
	if err := e.PublishAreaChat(h.ID, "hello"); err != nil {
		t.Fatal(err)
	}
	if len(hs.events) != 0 || len(os.events) != 0 || len(bs.events) != 0 || len(ds.events) != 0 {
		t.Fatal("area text leaked into local SNAP echo, another area, directory or closed socket")
	}
	ev := <-gs.events
	if ev.ChatArea != 1 || ev.Actor.ID != h.ID || !e.RoomChatDeliverable(g.ID, ev) {
		t.Fatal("authenticated area event not delivered")
	}
	e.SetKDDIBrowse(g.ID, 1, 1)
	e.SetKDDIBrowse(g.ID, 1, 0)
	if e.RoomChatDeliverable(g.ID, ev) {
		t.Fatal("queued text survived leaving and revisiting area")
	}
	if err := e.PublishAreaChat(g.ID, "ｃｃｃｃｃ"); err != nil {
		t.Fatal(err)
	}
	own := <-gs.events
	incoming := <-hs.events
	if own.ChatText != "ｃｃｃｃｃ" || incoming.Actor.ID != g.ID || incoming.ChatAftergame {
		t.Fatal("native Japanese echo or canonical text wrong")
	}
	e.SetKDDIBrowse(g.ID, 2, 0)
	e.SetKDDIBrowse(g.ID, 1, 0)
	if e.RoomChatDeliverable(h.ID, incoming) {
		t.Fatal("queued source text survived navigation")
	}
	if err := e.PublishAreaChat(h.ID, "party transition"); err != nil {
		t.Fatal(err)
	}
	ev = <-gs.events
	r, err := e.CreateRoom(g.ID, RoomOptions{Area: 1, Scenario: 1, MaxPlayers: 2})
	if err != nil {
		t.Fatal(err)
	}
	if e.AreaChatSupported(g.ID) || e.RoomChatDeliverable(g.ID, ev) {
		t.Fatal("party member received area text")
	}
	if err = e.LeaveRoom(g.ID); err != nil {
		t.Fatal(err)
	}
	if !e.AreaChatSupported(g.ID) || e.RoomChatDeliverable(g.ID, ev) {
		t.Fatal("old area text replayed after party departure", r.ID)
	}
	if err = e.PublishAreaChat(h.ID, "detach"); err != nil {
		t.Fatal(err)
	}
	ev = <-gs.events
	e.DetachKDDILobby(g.ID)
	if e.AreaChatSupported(g.ID) || e.RoomChatDeliverable(g.ID, ev) {
		t.Fatal("disconnected lobby retained chat presence")
	}
}
func TestAreaChatBackpressureAndUnmeasuredGames(t *testing.T) {
	e := New(nil, testAuthority{}, file1.Definition(), file2.Definition())
	h := actor(t, e, world.GameFile2, SNAP, "pal", "western")
	g := actor(t, e, world.GameFile2, KDDI, "ntsc-j", "japanese")
	e.SetArea(h.ID, 1)
	e.SetNativeLobby(h.ID, 1)
	e.AttachKDDILobby(g.ID)
	e.SetKDDIBrowse(g.ID, 1, 0)
	hs, _ := e.SubscribeRoom(h.ID)
	gs, _ := e.SubscribeRoom(g.ID)
	for len(gs.events) < roomEventLimit {
		gs.events <- RoomEvent{}
	}
	if err := e.PublishAreaChat(g.ID, "bounded"); err != ErrRoomBackpressure {
		t.Fatal(err)
	}
	if len(hs.events) != 0 {
		t.Fatal("partial publication before congestion check")
	}
	if err := e.PublishAreaChat(h.ID, "bad\ncontrol"); err == nil {
		t.Fatal("control text accepted")
	}
	f := actor(t, e, world.GameFile1, SNAP, "ntsc-u", "fileone")
	e.SetArea(f.ID, 1)
	e.SetNativeLobby(f.ID, 1)
	if e.AreaChatSupported(f.ID) || e.PublishAreaChat(f.ID, "hello") == nil {
		t.Fatal("File2 behavior generalized to File1")
	}
	e.SetArea(h.ID, 2)
	e.SetNativeLobby(h.ID, 2)
	if e.AreaChatSupported(h.ID) || e.PublishAreaChat(h.ID, "hello") == nil {
		t.Fatal("unmeasured area enabled")
	}
}
