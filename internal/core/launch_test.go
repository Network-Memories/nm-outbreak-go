package core

import (
	"bytes"
	"encoding/binary"
	"networkmemories/nm-outbreak-go/internal/modules/file2"
	"networkmemories/nm-outbreak-go/internal/protocol/snaplaunch"
	"networkmemories/nm-outbreak-go/internal/results"
	"networkmemories/nm-outbreak-go/internal/world"
	"testing"
)

func launchActors(t *testing.T) (*Engine, Session, Session, *world.Room, *RoomSubscription, *RoomSubscription) {
	t.Helper()
	e := New(nil, testAuthority{}, file2.Definition())
	ledger, _ := results.Open("")
	e.ConfigureResults(ledger)
	host := actor(t, e, world.GameFile2, SNAP, "pal", "western")
	guest := actor(t, e, world.GameFile2, KDDI, "ntsc-j", "japanese")
	e.BindSNAPID(host.ID, 123)
	p := make([]byte, 240)
	p[224] = 4
	p[234] = 1
	p[235] = 2
	p[236] = 1
	e.SetProfile(host.ID, p)
	host, _ = e.Session(host.ID)
	stat, _ := file2.EncodeStatWord(1, 2, 1)
	room, err := e.CreateRoom(host.ID, RoomOptions{Area: 1, Scenario: 1, MaxPlayers: 2, StatWord: stat})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = e.JoinRoom(guest.ID, room.ID); err != nil {
		t.Fatal(err)
	}
	guest, _ = e.Session(guest.ID)
	hs, _ := e.SubscribeRoom(host.ID)
	gs, _ := e.SubscribeRoom(guest.ID)
	return e, host, guest, room, hs, gs
}
func feedLaunch(t *testing.T, e *Engine, host, guest Session) {
	t.Helper()
	send := func(tag uint32, b []byte) {
		t.Helper()
		if err := e.ReceiveWesternLaunch(host.ID, snaplaunch.Message{Target: guest.WireID, Tag: tag, Body: b}); err != nil {
			t.Fatal(tag, err)
		}
	}
	send(5, []byte{1, 2})
	for i, s := range []Session{host, guest} {
		b := make([]byte, 34)
		b[0] = byte(i)
		copy(b[18:], s.Account.NativeAlias)
		send(6, b)
	}
	for i, s := range []Session{host, guest} {
		p := append([]byte(nil), s.Profile...)
		if len(p) == 208 {
			p = append(p, make([]byte, 32)...)
		}
		for step := 0; step < 5; step++ {
			n := 50
			if step == 4 {
				n = 40
			}
			b := make([]byte, 54)
			b[0] = byte(i)
			b[1] = byte(n)
			binary.LittleEndian.PutUint16(b[2:], uint16(step*50))
			copy(b[4:], p[step*50:step*50+n])
			send(7, b)
		}
	}
	send(8, nil)
}
func TestLaunchNeedsActualGuestAndReusesCanonicalRound(t *testing.T) {
	e, h, g, r, hs, gs := launchActors(t)
	msg := snaplaunch.Message{Target: g.WireID, Tag: 3}
	if err := e.ReceiveWesternLaunch(g.ID, msg); err == nil {
		t.Fatal("guest announced host launch")
	}
	if err := e.ReceiveWesternLaunch(h.ID, msg); err != nil {
		t.Fatal(err)
	}
	if event := <-gs.Events(); event.Kind != RoomLaunchAnnounced {
		t.Fatal(event.Kind)
	}
	select {
	case <-hs.Events():
		t.Fatal("ack without guest proof")
	default:
	}
	if _, err := e.NativeLaunch(g.ID); err == nil {
		t.Fatal("incomplete table available")
	}
	if err := e.ReceiveWesternLaunch(h.ID, snaplaunch.Message{Target: g.WireID, Tag: 5, Body: []byte{1, 2}}); err == nil {
		t.Fatal("host bypassed guest acknowledgement")
	}
	if err := e.RecordResult(g.ID, results.Stats{}); err == nil {
		t.Fatal("unentered guest ranked")
	}
	ready, err := e.AcknowledgeNativeLaunch(g.ID)
	if err != nil {
		t.Fatal(err)
	}
	if event := <-hs.Events(); event.Kind != RoomLaunchAcknowledged || event.Actor.ID != g.ID {
		t.Fatal(event)
	}
	e.AcknowledgeNativeLaunch(g.ID)
	select {
	case <-hs.Events():
		t.Fatal("duplicate ack")
	default:
	}
	feedLaunch(t, e, h, g)
	select {
	case <-ready:
	default:
		t.Fatal("complete table not published")
	}
	got, err := e.NativeLaunch(g.ID)
	if err != nil || got.Number != 1 || got.Count != 2 || len(got.NativeNumber) != 15 || got.Rules[3] != 2 || got.Rules[4] != 1 {
		t.Fatal(got, err)
	}
	got.Profiles[0][200] = 99
	got.Room.Members[0].Profile[0] = 99
	again, _ := e.NativeLaunch(g.ID)
	if again.Profiles[0][200] == 99 || again.Room.Members[0].Profile[0] == 99 {
		t.Fatal("launch snapshot aliases shared state")
	}
	for _, s := range []Session{h, g} {
		if err = e.ScenarioPresence(s.ID, true); err != nil {
			t.Fatal(err)
		}
		p := uint32(10)
		if err = e.RecordResult(s.ID, results.Stats{Points: &p}); err != nil {
			t.Fatal(err)
		}
	}
	rows := e.Results().Snapshot()
	if len(rows) != 2 || rows[0].RoundID != got.RoundID || rows[1].RoundID != got.RoundID {
		t.Fatal("launch split stats round", rows)
	}
	if room, _ := e.Worlds().GetRoom(r.ID); !room.Playing {
		t.Fatal("entry missing")
	}
}
func TestLaunchRejectsSpoofAndReleasesWaiterOnDeparture(t *testing.T) {
	e, h, g, _, _, _ := launchActors(t)
	if err := e.ReceiveWesternLaunch(h.ID, snaplaunch.Message{Target: g.WireID, Tag: 3, Body: []byte{1}}); err == nil || len(e.roomRounds) != 0 {
		t.Fatal("malformed03 mutated round")
	}
	e.ReceiveWesternLaunch(h.ID, snaplaunch.Message{Target: g.WireID, Tag: 3})
	ready, _ := e.AcknowledgeNativeLaunch(g.ID)
	wrong := snaplaunch.Message{Target: g.WireID, Tag: 5, Body: []byte{1, 3}}
	if e.ReceiveWesternLaunch(h.ID, wrong) == nil {
		t.Fatal("invented participant count accepted")
	}
	e.ReceiveWesternLaunch(h.ID, snaplaunch.Message{Target: g.WireID, Tag: 5, Body: []byte{1, 2}})
	b := make([]byte, 34)
	copy(b[18:], "other-owner")
	if e.ReceiveWesternLaunch(h.ID, snaplaunch.Message{Target: g.WireID, Tag: 6, Body: b}) == nil {
		t.Fatal("spoofed owner name")
	}
	b = make([]byte, 34)
	copy(b[18:], h.Account.NativeAlias)
	if err := e.ReceiveWesternLaunch(h.ID, snaplaunch.Message{Target: g.WireID, Tag: 6, Body: b}); err != nil {
		t.Fatal("rejection poisoned parser", err)
	}
	e.LeaveRoom(g.ID)
	select {
	case <-ready:
	default:
		t.Fatal("departure leaked waiter")
	}
	if e.HasNativeLaunch(g.ID) || len(e.roomRounds) != 0 {
		t.Fatal("cancelled launch retained")
	}
	if _, err := e.NativeLaunch(g.ID); err == nil {
		t.Fatal("cancelled table remains usable")
	}
}
func TestLaunchRulesRequireMeasuredHostProperty(t *testing.T) {
	e, h, g, _, _, _ := launchActors(t)
	p := append([]byte(nil), h.Profile...)
	p[234] = 4
	e.SetProfile(h.ID, p)
	if e.ReceiveWesternLaunch(h.ID, snaplaunch.Message{Target: g.WireID, Tag: 3}) == nil {
		t.Fatal("contradictory wait rules accepted")
	}
	if len(e.roomRounds) != 0 {
		t.Fatal("bad rules froze round")
	}
	if bytes.Equal(p, h.Profile) {
		t.Fatal("fixture did not change wait")
	}
}

func TestPreviousLobbyStatusCannotEraseNewPreparedRound(t *testing.T) {
	e, h, g, room, _, _ := launchActors(t)
	for _, s := range []Session{h, g} {
		if err := e.ScenarioPresence(s.ID, true); err != nil {
			t.Fatal(err)
		}
	}
	for _, s := range []Session{h, g} {
		e.ScenarioPresence(s.ID, false)
	}
	if err := e.ReceiveWesternLaunch(h.ID, snaplaunch.Message{Target: g.WireID, Tag: 3}); err != nil {
		t.Fatal(err)
	}
	prepared := e.roomRounds[room.ID]
	if prepared == nil {
		t.Fatal("missing new round")
	}
	// A delayed duplicate return belongs to the old round retained for results.
	e.ScenarioPresence(g.ID, false)
	if e.roomRounds[room.ID] != prepared || !e.HasNativeLaunch(g.ID) {
		t.Fatal("old return cancelled new launch")
	}
}
