package core

import (
	"bytes"
	"networkmemories/nm-outbreak-go/internal/modules/file1"
	"networkmemories/nm-outbreak-go/internal/modules/file2"
	"networkmemories/nm-outbreak-go/internal/world"
	"testing"
	"time"
)

func TestWaitingGreetingPrecedesNative6504WithoutInventingReadiness(t *testing.T) {
	e, h, g, _, hs, gs := launchActors(t)
	if err := e.SignalWaitingGreeting(g.ID, h.WireID); err == nil {
		t.Fatal("guest forged host signal")
	}
	if err := e.SignalWaitingGreeting(h.ID, g.WireID); err != nil {
		t.Fatal(err)
	}
	first := <-gs.Events()
	if first.Kind != RoomWaitingGreeting || first.Actor.ID != h.ID {
		t.Fatal(first)
	}
	if e.waiting[g.ID].ready {
		t.Fatal("host signal invented native readiness")
	}
	if err := e.MarkKDDIWaitingReady(g.ID); err != nil {
		t.Fatal(err)
	}
	second := <-gs.Events()
	if second.Kind != RoomWaitingReady || second.Actor.ID != g.ID {
		t.Fatal(second)
	}
	if event := <-hs.Events(); event.Kind != RoomWaitingReady || event.Actor.ID != g.ID {
		t.Fatal(event)
	}
	e.MarkKDDIWaitingReady(g.ID)
	e.SignalWaitingGreeting(h.ID, g.WireID)
	select {
	case <-gs.Events():
		t.Fatal("duplicate native preparation callbacks")
	default:
	}
	if current, _ := e.Session(g.ID); current.InScenario {
		t.Fatal("waiting ready invented scenario entry")
	}
}
func TestWaitingReadyBeforeHostSignalAndExplicitDeparture(t *testing.T) {
	e, h, g, _, _, gs := launchActors(t)
	e.MarkKDDIWaitingReady(g.ID)
	if event := <-gs.Events(); event.Kind != RoomWaitingReady {
		t.Fatal(event)
	}
	select {
	case <-gs.Events():
		t.Fatal("host greeting invented from membership")
	default:
	}
	if err := e.SignalWaitingGreeting(h.ID, g.WireID); err != nil {
		t.Fatal(err)
	}
	if event := <-gs.Events(); event.Kind != RoomWaitingGreeting {
		t.Fatal(event)
	}
	e.LeaveRoom(g.ID)
	if _, exists := e.waiting[g.ID]; exists {
		t.Fatal("departed native state retained")
	}
	if e.SignalWaitingGreeting(h.ID, g.WireID) == nil {
		t.Fatal("departed recipient accepted")
	}
}
func TestWaitingProjectionAndDeadlineDoNotChangeOnRosterUpdates(t *testing.T) {
	e, h, g, room, _, _ := launchActors(t)
	// This native client is joined, with real stable area/room coordinates.
	e.mu.Lock()
	peer := e.sessions[g.ID]
	peer.NativeLobbyID = 1
	peer.NativeRoomID = 1
	peer.Area = 1
	e.mu.Unlock()
	slots, err := e.KDDISlots(g.ID)
	if err != nil {
		t.Fatal(err)
	}
	var slot uint16
	for key, value := range slots {
		if value.ID == room.ID {
			slot = key
		}
	}
	schema, values, err := e.KDDIRules(g.ID, slot)
	if err != nil || len(schema) != 3 || !bytes.Equal(values, []byte{0, 1, 2}) {
		t.Fatal(values, err)
	}
	now := room.CreatedAt.Add(45 * time.Second)
	before, err := e.KDDIRemaining(g.ID, slot, now)
	if err != nil || before != 255 {
		t.Fatal(before, err)
	}
	p := append([]byte(nil), h.Profile...)
	p[200] = 2
	e.SetProfile(h.ID, p)
	e.SetRoomStat(h.ID, room.StatWord|0x40000000)
	after, err := e.KDDIRemaining(g.ID, slot, now)
	if err != nil || after != before {
		t.Fatal("roster/stat reset deadline", after, err)
	}
	zero, err := e.KDDIRemaining(g.ID, slot, room.CreatedAt.Add(301*time.Second))
	if err != nil || zero != 0 {
		t.Fatal(zero, err)
	}
	if e.HasNativeLaunch(g.ID) || len(e.roomRounds) != 0 {
		t.Fatal("timer zero forged native launch")
	}
	if _, err = e.KDDIRemaining(g.ID, slot+1, now); err == nil {
		t.Fatal("empty slot supplied invented wait")
	}
}

func TestNativeNewProfileRequiresFreshReadyWithoutDuplicateGreeting(t *testing.T) {
	e, h, g, _, _, gs := launchActors(t)
	e.SignalWaitingGreeting(h.ID, g.WireID)
	e.MarkKDDIWaitingReady(g.ID)
	<-gs.Events()
	<-gs.Events()
	p := append([]byte(nil), g.Profile...)
	p[200] = 2
	if err := e.SetProfile(g.ID, p); err != nil {
		t.Fatal(err)
	}
	if err := e.MarkKDDIWaitingReady(g.ID); err != nil {
		t.Fatal(err)
	}
	if event := <-gs.Events(); event.Kind != RoomWaitingReady || event.Actor.Profile[200] != 2 {
		t.Fatal(event)
	}
	select {
	case <-gs.Events():
		t.Fatal("new character repeated already delivered host greeting")
	default:
	}
}

func TestWaitingGreetingBackpressureCanRetryWithoutLosingHostProof(t *testing.T) {
	e, h, g, _, _, gs := launchActors(t)
	for i := 0; i < roomEventLimit; i++ {
		gs.events <- RoomEvent{}
	}
	if err := e.SignalWaitingGreeting(h.ID, g.WireID); err != ErrRoomBackpressure {
		t.Fatal(err)
	}
	if e.waiting[g.ID].hostID != "" {
		t.Fatal("failed enqueue consumed host signal")
	}
	for i := 0; i < roomEventLimit; i++ {
		<-gs.Events()
	}
	if err := e.SignalWaitingGreeting(h.ID, g.WireID); err != nil {
		t.Fatal(err)
	}
	if event := <-gs.Events(); event.Kind != RoomWaitingGreeting {
		t.Fatal(event)
	}
	if e.waiting[g.ID].ready {
		t.Fatal("greeting invented native readiness")
	}
}

func TestWesternGuestWaitRequiresJapaneseHostGreeting(t *testing.T) {
	e := New(nil, testAuthority{}, file2.Definition())
	h := actor(t, e, world.GameFile2, KDDI, "ntsc-j", "japanese")
	g := actor(t, e, world.GameFile2, SNAP, "pal", "western")
	if err := e.AttachKDDILobby(h.ID); err != nil {
		t.Fatal(err)
	}
	if err := e.SetKDDIBrowse(h.ID, 1, 1); err != nil {
		t.Fatal(err)
	}
	room, err := e.CreateRoom(h.ID, RoomOptions{Area: 1, Scenario: 1, MaxPlayers: 2, RuleValues: []byte{0, 1, 1}, Difficulty: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.JoinRoom(g.ID, room.ID); err != nil {
		t.Fatal(err)
	}
	sub, err := e.SubscribeRoom(g.ID)
	if err != nil {
		t.Fatal(err)
	}
	host, _ := e.Session(h.ID)
	now := room.CreatedAt.Add(45 * time.Second)
	if _, err := e.WesternGuestWait(g.ID, host.WireID, now); err == nil {
		t.Fatal("reply before greeting")
	}
	if err := e.SignalKDDIWaitingGreeting(h.ID, g.Account.Handle); err != nil {
		t.Fatal(err)
	}
	if event := <-sub.Events(); event.Kind != RoomWaitingGreeting || event.Actor.ID != h.ID {
		t.Fatal("wrong greeting")
	}
	if seconds, err := e.WesternGuestWait(g.ID, host.WireID, now); err != nil || seconds != 255 {
		t.Fatal(seconds, err)
	}
	if _, err := e.WesternGuestWait(g.ID, host.WireID+1, now); err == nil {
		t.Fatal("reply to wrong target")
	}
}

func TestJapaneseLaunchWaitsForActualWesternAcknowledgement(t *testing.T) {
	e := New(nil, testAuthority{}, file2.Definition())
	h := actor(t, e, world.GameFile2, KDDI, "ntsc-j", "japanese")
	g := actor(t, e, world.GameFile2, SNAP, "pal", "western")
	if err := e.BindSNAPID(g.ID, 42); err != nil {
		t.Fatal(err)
	}
	if err := e.AttachKDDILobby(h.ID); err != nil {
		t.Fatal(err)
	}
	if err := e.SetKDDIBrowse(h.ID, 1, 1); err != nil {
		t.Fatal(err)
	}
	stat, err := file2.EncodeStatWord(1, 4, 1)
	if err != nil {
		t.Fatal(err)
	}
	room, err := e.CreateRoom(h.ID, RoomOptions{Area: 1, Scenario: 1, MaxPlayers: 4, Difficulty: 1, RuleValues: []byte{2, 1, 1}, StatWord: stat})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.JoinRoom(g.ID, room.ID); err != nil {
		t.Fatal(err)
	}
	hs, _ := e.SubscribeRoom(h.ID)
	gs, _ := e.SubscribeRoom(g.ID)
	if err := e.SignalKDDIWaitingGreeting(h.ID, g.Account.Handle); err != nil {
		t.Fatal(err)
	}
	<-gs.Events()
	if err := e.JapaneseLaunchStart(h.ID); err != nil {
		t.Fatal(err)
	}
	if (<-hs.Events()).Kind != RoomLaunchAnnounced || (<-gs.Events()).Kind != RoomLaunchAnnounced {
		t.Fatal("launch must notify both clients")
	}
	ready, err := e.AcknowledgeNativeLaunch(h.ID)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-ready:
		t.Fatal("JAP unlocked before SNAP response")
	default:
	}
	host, _ := e.Session(h.ID)
	table, handled, err := e.JapaneseLaunchGuestTable(g.ID, host.WireID, make([]byte, 768))
	if !handled || err != nil || table.Count != 2 || table.Number != 1 {
		t.Fatal(handled, table.Count, table.Number, err)
	}
	e.CompleteJapaneseLaunchGuest(g.ID, table.RoundID)
	select {
	case <-ready:
	default:
		t.Fatal("launch barrier did not open")
	}
	details, err := e.NativeLaunch(h.ID)
	if err != nil || details.RoundID != table.RoundID || details.Number != 0 || details.Count != 2 {
		t.Fatal(details.RoundID, details.Number, details.Count, err)
	}
	for index, id := range []string{h.ID, g.ID} {
		if err := e.ScenarioPresence(id, true); err != nil {
			t.Fatal(err)
		}
		ctx, err := e.MixedEngineContext(id)
		if err != nil || ctx.Index != index || ctx.RoundID != table.RoundID {
			t.Fatalf("seat %d mixed relay: %+v %v", index, ctx, err)
		}
	}
}

func TestFile1JapaneseLaunchWaitsForActualUSAcknowledgement(t *testing.T) {
	e := New(nil, testAuthority{}, file1.Definition())
	h := actor(t, e, world.GameFile1, KDDI, "ntsc-j", "japanese")
	g := actor(t, e, world.GameFile1, SNAP, "ntsc-u", "western")
	if err := e.BindSNAPID(g.ID, 42); err != nil {
		t.Fatal(err)
	}
	if err := e.AttachKDDILobby(h.ID); err != nil {
		t.Fatal(err)
	}
	if err := e.SetKDDIBrowse(h.ID, 2, 1); err != nil {
		t.Fatal(err)
	}
	room, err := e.CreateRoom(h.ID, RoomOptions{NativeMedia: 0x0001, Area: 1, Scenario: 1, MaxPlayers: 4, Difficulty: 2, RuleValues: []byte{2, 1, 2, 1}, StatWord: file1.OpenRoomStatBase})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.JoinRoom(g.ID, room.ID); err != nil {
		t.Fatal(err)
	}
	hs, _ := e.SubscribeRoom(h.ID)
	gs, _ := e.SubscribeRoom(g.ID)
	if err := e.SignalKDDIWaitingGreeting(h.ID, g.Account.Handle); err != nil {
		t.Fatal(err)
	}
	<-gs.Events()
	if err := e.JapaneseLaunchStart(h.ID); err != nil {
		t.Fatal(err)
	}
	if (<-hs.Events()).Kind != RoomLaunchAnnounced || (<-gs.Events()).Kind != RoomLaunchAnnounced {
		t.Fatal("launch must notify both clients")
	}
	ready, err := e.AcknowledgeNativeLaunch(h.ID)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-ready:
		t.Fatal("JAP unlocked before SNAP response")
	default:
	}
	host, _ := e.Session(h.ID)
	table, handled, err := e.JapaneseLaunchGuestTable(g.ID, host.WireID, make([]byte, 768))
	if !handled || err != nil || table.Count != 2 || table.Number != 1 {
		t.Fatal(handled, table.Count, table.Number, err)
	}
	e.CompleteJapaneseLaunchGuest(g.ID, table.RoundID)
	select {
	case <-ready:
	default:
		t.Fatal("launch barrier did not open")
	}
	details, err := e.NativeLaunch(h.ID)
	if err != nil || details.RoundID != table.RoundID || details.Number != 0 || details.Count != 2 {
		t.Fatal(details.RoundID, details.Number, details.Count, err)
	}
	if len(details.Rules) != 18 || details.Rules[1] != 16 || details.Rules[2] != 1 || details.Rules[3] != 2 || details.Rules[4] != 1 {
		t.Fatalf("unexpected File1 GAMEDIFF rules: %x", details.Rules)
	}
	if score := e.KDDIEmptyPlayerScore(world.GameFile1, 1); len(score) != 3 || score[0] != 1 {
		t.Fatalf("unexpected File1 empty player score: %x", score)
	}
}
