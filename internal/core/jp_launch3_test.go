package core

import (
	"bytes"
	"networkmemories/nm-outbreak-go/internal/modules/file1"
	"networkmemories/nm-outbreak-go/internal/protocol/enginewire"
	"networkmemories/nm-outbreak-go/internal/world"
	"testing"
)

// Japanese host, Japanese guest and US guest share one frozen launch; the
// Japanese seats unlock only after the actual US table reply.
func TestFile1JapaneseHostLaunchesThreeMixedSeats(t *testing.T) {
	e := New(nil, testAuthority{}, file1.Definition())
	h := actor(t, e, world.GameFile1, KDDI, "ntsc-j", "japanese")
	j := actor(t, e, world.GameFile1, KDDI, "ntsc-j", "japanese2")
	g := actor(t, e, world.GameFile1, SNAP, "ntsc-u", "western")
	for id, handle := range map[string]string{h.ID: "HOST01", j.ID: "GUEST2", g.ID: "WEST03"} {
		e.sessions[id].Account.Handle = handle
	}
	if err := e.BindSNAPID(g.ID, 42); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{h.ID, j.ID} {
		if err := e.AttachKDDILobby(id); err != nil {
			t.Fatal(err)
		}
		if err := e.SetKDDIBrowse(id, 2, 1); err != nil {
			t.Fatal(err)
		}
	}
	room, err := e.CreateRoom(h.ID, RoomOptions{NativeMedia: 0x0001, Area: 1, Scenario: 1, MaxPlayers: 3, Difficulty: 2, RuleValues: []byte{1, 1, 2, 1}, StatWord: file1.OpenRoomStatBase})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{j.ID, g.ID} {
		if _, err := e.JoinRoom(id, room.ID); err != nil {
			t.Fatal(err)
		}
	}
	hs, _ := e.SubscribeRoom(h.ID)
	js, _ := e.SubscribeRoom(j.ID)
	gs, _ := e.SubscribeRoom(g.ID)
	if err := e.JapaneseLaunchStart(h.ID); err == nil {
		t.Fatal("launch before the actual US greeting")
	}
	if err := e.SignalKDDIWaitingGreeting(h.ID, "WEST03"); err != nil {
		t.Fatal(err)
	}
	<-gs.Events()
	if err := e.JapaneseLaunchStart(h.ID); err != nil {
		t.Fatal(err)
	}
	for _, s := range []*RoomSubscription{hs, js, gs} {
		if (<-s.Events()).Kind != RoomLaunchAnnounced {
			t.Fatal("launch must notify every seat")
		}
	}
	readyHost, err := e.AcknowledgeNativeLaunch(h.ID)
	if err != nil {
		t.Fatal(err)
	}
	readyGuest, err := e.AcknowledgeNativeLaunch(j.ID)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-readyGuest:
		t.Fatal("Japanese guest unlocked before the US reply")
	default:
	}
	host, _ := e.Session(h.ID)
	table, handled, err := e.JapaneseLaunchGuestTable(g.ID, host.WireID, make([]byte, 768))
	if !handled || err != nil || table.Count != 3 || table.Number != 2 {
		t.Fatal(handled, table.Count, table.Number, err)
	}
	e.CompleteJapaneseLaunchGuest(g.ID, table.RoundID)
	for _, ready := range []<-chan struct{}{readyHost, readyGuest} {
		select {
		case <-ready:
		default:
			t.Fatal("launch barrier did not open")
		}
	}
	for want, id := range []string{h.ID, j.ID} {
		d, err := e.NativeLaunch(id)
		if err != nil || d.Number != want || d.Count != 3 || d.RoundID != table.RoundID {
			t.Fatal(id, d.Number, d.Count, err)
		}
	}
	if d, _ := e.NativeLaunch(h.ID); !bytes.Equal(d.Profiles[1][:208], make([]byte, 208)) || d.Profiles[2] != table.Profiles[2] {
		t.Fatal("seat profiles differ from the frozen roster")
	}
	for _, id := range []string{h.ID, j.ID, g.ID} {
		if err := e.ScenarioPresence(id, true); err != nil {
			t.Fatal(err)
		}
	}
	for index, id := range []string{h.ID, j.ID, g.ID} {
		ctx, err := e.MixedEngineContext(id)
		if err != nil || ctx.Index != index || ctx.Count != 3 {
			t.Fatalf("seat %d: %+v %v", index, ctx, err)
		}
	}
	ha, err := e.AttachEngine(h.ID)
	if err != nil {
		t.Fatal(err)
	}
	ja, _ := e.AttachEngine(j.ID)
	ga, _ := e.AttachEngine(g.ID)
	// Actual Japanese records reach only the other Japanese seat, verbatim.
	probe := []byte{7, 0x20, 0, 1, 2, 3, 4}
	if err := e.PublishKDDIRaw(h.ID, table.RoundID, probe); err != nil {
		t.Fatal(err)
	}
	if d := <-ja.Events(); !bytes.Equal(d.Raw, probe) || d.FromID != h.ID {
		t.Fatalf("%+v", d)
	}
	if e.PublishKDDIRaw(h.ID, table.RoundID, []byte{7, 0x21, 0, 1, 2, 3, 4}) == nil {
		t.Fatal("record with another sender place accepted")
	}
	// A Japanese data frame reaches the US seat only.
	if err := e.PublishEngine(j.ID, table.RoundID, []enginewire.Frame{{Peer: 1, Body: []byte{1, 2}}}); err != nil {
		t.Fatal(err)
	}
	if d := <-ga.Events(); d.Raw != nil || d.Frame.Peer != 1 {
		t.Fatalf("%+v", d)
	}
	select {
	case d := <-ha.Events():
		t.Fatalf("Japanese frame looped to a Japanese seat: %+v", d)
	default:
	}
}

func TestKDDIAftergameEventRelaysOnlyBetweenReturnedJapaneseSeats(t *testing.T) {
	e := New(nil, testAuthority{}, file1.Definition())
	h := actor(t, e, world.GameFile1, KDDI, "ntsc-j", "japanese")
	j := actor(t, e, world.GameFile1, KDDI, "ntsc-j", "japanese2")
	g := actor(t, e, world.GameFile1, SNAP, "ntsc-u", "western")
	for id, handle := range map[string]string{h.ID: "HOST01", j.ID: "GUEST2", g.ID: "WEST03"} {
		e.sessions[id].Account.Handle = handle
	}
	room, err := e.CreateRoom(h.ID, RoomOptions{Area: 1, Scenario: 1, MaxPlayers: 3})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{j.ID, g.ID} {
		if _, err := e.JoinRoom(id, room.ID); err != nil {
			t.Fatal(err)
		}
	}
	hs, _ := e.SubscribeRoom(h.ID)
	e.SubscribeRoom(j.ID)
	e.SubscribeRoom(g.ID)
	r, _ := e.worlds.GetRoom(room.ID)
	if _, err := e.scenarioRoundLocked(r); err != nil {
		t.Fatal(err)
	}
	event := make([]byte, 80)
	if e.RelayKDDIAftergameEvent(j.ID, "HOST01", event) == nil {
		t.Fatal("relay before any actual return")
	}
	for _, id := range []string{h.ID, j.ID, g.ID} {
		e.ScenarioPresence(id, true)
	}
	for _, id := range []string{h.ID, j.ID, g.ID} {
		e.ScenarioPresence(id, false)
	}
	for len(hs.events) > 0 {
		<-hs.events
	}
	if err := e.RelayKDDIAftergameEvent(j.ID, "HOST01", event); err != nil {
		t.Fatal(err)
	}
	if ev := <-hs.Events(); ev.Kind != RoomNativeEvent || ev.Actor.ID != j.ID || len(ev.Journal) != 80 {
		t.Fatalf("%+v", ev)
	}
	if e.RelayKDDIAftergameEvent(j.ID, "WEST03", event) == nil {
		t.Fatal("western recipient must keep its measured path")
	}
}
