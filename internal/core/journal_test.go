package core

import (
	"bytes"
	"networkmemories/nm-outbreak-go/internal/modules/file2"
	"networkmemories/nm-outbreak-go/internal/world"
	"testing"
)

func TestMixedJournalAuthenticatesRoundAndWaitsForNativeReadiness(t *testing.T) {
	e := New(nil, testAuthority{}, file2.Definition())
	h := actor(t, e, world.GameFile2, SNAP, "pal", "western")
	g := actor(t, e, world.GameFile2, KDDI, "ntsc-j", "japanese")
	outsider := actor(t, e, world.GameFile2, KDDI, "ntsc-j", "outsider")
	room, err := e.CreateRoom(h.ID, RoomOptions{Area: 1, Scenario: 1, MaxPlayers: 2})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = e.JoinRoom(g.ID, room.ID); err != nil {
		t.Fatal(err)
	}
	hs, _ := e.SubscribeRoom(h.ID)
	gs, _ := e.SubscribeRoom(g.ID)
	if e.ReceiveAftergameJournal(g.ID, h.ID, true, make([]byte, 64)) == nil {
		t.Fatal("waiting invented journal")
	}
	e.ScenarioPresence(h.ID, true)
	e.ScenarioPresence(g.ID, true)
	e.ScenarioPresence(g.ID, false)
	if e.ReceiveAftergameJournal(g.ID, h.ID, true, make([]byte, 64)) == nil {
		t.Fatal("journal entered scenario overlay")
	}
	e.ScenarioPresence(h.ID, false)
	for len(hs.events) > 0 {
		<-hs.events
	}
	for len(gs.events) > 0 {
		<-gs.events
	}
	if e.ReceiveAftergameJournal(outsider.ID, h.ID, true, make([]byte, 64)) == nil {
		t.Fatal("outsider accepted")
	}
	if e.ReceiveAftergameJournal(g.ID, h.ID, false, make([]byte, 64)) == nil {
		t.Fatal("premature terminator accepted")
	}
	jp := make([]byte, 2048)
	pal := make([]byte, 2048)
	for i := range jp {
		jp[i] = byte(i * 7)
		pal[i] = byte(i * 11)
	}
	for i := 0; i < 2048; i += 64 {
		if err = e.ReceiveAftergameJournal(g.ID, h.ID, i+64 < 2048, jp[i:i+64]); err != nil {
			t.Fatal(err)
		}
	}
	if len(hs.events) != 0 {
		t.Fatal("journal delivered before receiver table ready")
	}
	if err = e.ReceiveAftergameJournal(h.ID, g.ID, true, pal[:32]); err != nil {
		t.Fatal(err)
	}
	ev := <-hs.events
	if ev.Kind != RoomAftergameJournal || ev.Actor.ID != g.ID || !bytes.Equal(ev.Journal, jp) {
		t.Fatal("actual Japanese journal changed")
	}
	ev.Journal[0] ^= 255
	if e.sessionRounds[g.ID].journals[journalRoute{g.ID, h.ID}].data[0] != jp[0] {
		t.Fatal("journal aliases snapshot")
	}
	for i := 32; i < 2048; i += 32 {
		if err = e.ReceiveAftergameJournal(h.ID, g.ID, i+32 < 2048, pal[i:i+32]); err != nil {
			t.Fatal(err)
		}
	}
	ev = <-gs.events
	if ev.Actor.ID != h.ID || !bytes.Equal(ev.Journal, pal) {
		t.Fatal("actual western journal changed")
	}
	if len(hs.events) != 0 || len(gs.events) != 0 {
		t.Fatal("duplicate delivery")
	}
	if e.ReceiveAftergameJournal(h.ID, g.ID, false, pal[2016:]) == nil {
		t.Fatal("completed stream appended again")
	}
	e.LeaveRoom(g.ID)
	if e.ReceiveAftergameJournal(g.ID, h.ID, true, jp[:64]) == nil {
		t.Fatal("departed owner retained journal access")
	}
}

func TestJournalBackpressureRetainsDataUntilRecipientDrains(t *testing.T) {
	e := New(nil, testAuthority{}, file2.Definition())
	h := actor(t, e, world.GameFile2, SNAP, "pal", "western")
	g := actor(t, e, world.GameFile2, KDDI, "ntsc-j", "japanese")
	r, _ := e.CreateRoom(h.ID, RoomOptions{Area: 1, Scenario: 1, MaxPlayers: 2})
	e.JoinRoom(g.ID, r.ID)
	hs, _ := e.SubscribeRoom(h.ID)
	e.SubscribeRoom(g.ID)
	e.ScenarioPresence(h.ID, true)
	e.ScenarioPresence(g.ID, true)
	e.ScenarioPresence(h.ID, false)
	e.ScenarioPresence(g.ID, false)
	for len(hs.events) < roomEventLimit {
		hs.events <- RoomEvent{}
	}
	for i := 0; i < 32; i++ {
		if err := e.ReceiveAftergameJournal(g.ID, h.ID, i < 31, bytes.Repeat([]byte{7}, 64)); err != nil {
			t.Fatal(err)
		}
	}
	if err := e.ReceiveAftergameJournal(h.ID, g.ID, true, make([]byte, 32)); err != nil {
		t.Fatal(err)
	}
	stream := e.sessionRounds[g.ID].journals[journalRoute{g.ID, h.ID}]
	if !stream.complete || stream.delivered || len(stream.data) != 2048 {
		t.Fatal("complete journal lost under pressure")
	}
	for len(hs.events) > 0 {
		<-hs.events
	}
	e.FlushAftergameJournals(h.ID)
	ev := <-hs.events
	if ev.Kind != RoomAftergameJournal || !bytes.Equal(ev.Journal, bytes.Repeat([]byte{7}, 2048)) {
		t.Fatal("deferred data changed")
	}
	e.FlushAftergameJournals(h.ID)
	if len(hs.events) != 0 {
		t.Fatal("deferred journal repeated")
	}

}
