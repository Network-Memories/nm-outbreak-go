package world

import (
	"testing"
)

func TestRegistryLifecycleAndFanOut(t *testing.T) {
	reg := NewRegistry()

	rm := reg.CreateRoom(Room{
		Game:       GameFile2,
		HostConn:   "kddi:host1",
		HostSeat:   0,
		Area:       1,
		Scenario:   1,
		Title:      "",
		MaxPlayers: 4,
		Members: []Member{
			{Connection: "kddi:host1", Adapter: "kddi-file2", Name: "bbbbb", Region: "ntsc-j", Seat: 0},
		},
	})

	// Multi-guest join (2 Western guests + 1 JP guest -> 4 players total)
	for i, conn := range []string{"snap:g1", "snap:g2", "kddi:g3"} {
		updated, err := reg.JoinRoom(rm.ID, Member{Connection: conn, Adapter: "snap-file2", Region: "pal"})
		if err != nil {
			t.Fatalf("join %d failed: %v", i, err)
		}
		if len(updated.Members) != i+2 {
			t.Fatalf("expected %d members, got %d", i+2, len(updated.Members))
		}
	}
	if _, err := reg.JoinRoom(rm.ID, Member{Connection: "snap:g4"}); err == nil {
		t.Fatalf("expected room full error")
	}

	// Direct in-memory game packet fan-out
	chHost := reg.AttachGameRelay(rm.ID, "kddi:host1")
	chG1 := reg.AttachGameRelay(rm.ID, "snap:g1")
	chG2 := reg.AttachGameRelay(rm.ID, "snap:g2")

	reg.FanOutGamePacket(GamePacket{RoomID: rm.ID, FromConn: "kddi:host1", Payload: []byte{1, 2, 3}})
	select {
	case pkt := <-chG1:
		if len(pkt.Payload) != 3 {
			t.Fatalf("unexpected payload len")
		}
	default:
		t.Fatalf("g1 did not receive packet")
	}
	select {
	case <-chG2:
	default:
		t.Fatalf("g2 did not receive packet")
	}
	select {
	case <-chHost:
		t.Fatalf("sender should not receive own packet")
	default:
	}

	// Chat pub/sub
	chatCh := reg.SubscribeChat(GameFile2)
	reg.PublishChat(ChatMessage{Game: GameFile2, Channel: "world:" + rm.ID, Sender: "aaaaa", Text: "hello"})
	msg := <-chatCh
	if msg.Text != "hello" {
		t.Fatalf("unexpected chat text: %q", msg.Text)
	}

	// Host leave disbands room
	removed, disbanded := reg.LeaveRoom(rm.ID, "kddi:host1")
	if !removed || !disbanded {
		t.Fatalf("expected host leave to disband room")
	}
	if _, ok := reg.GetRoom(rm.ID); ok {
		t.Fatalf("room should be deleted after disband")
	}
}

func TestRoomSnapshotsDoNotExposeSharedMemory(t *testing.T) {
	reg := NewRegistry()
	input := Room{Game: GameFile2, HostConn: "host", MaxPlayers: 4, RuleValues: []byte{1}, Settings: map[string]string{"area": "1"}, Members: []Member{{Connection: "host", Profile: []byte{7}}}}
	created := reg.CreateRoom(input)
	input.RuleValues[0] = 9
	input.Settings["area"] = "9"
	input.Members[0].Profile[0] = 9
	created.Members[0].Profile[0] = 8
	created.Settings["area"] = "8"
	got, _ := reg.GetRoom(created.ID)
	if got.RuleValues[0] != 1 || got.Settings["area"] != "1" || got.Members[0].Profile[0] != 7 {
		t.Fatal("create snapshot aliases live state")
	}
	got.RuleValues[0] = 8
	got.Members[0].Profile[0] = 8
	listing := reg.ListRooms(GameFile2)
	listing[0].Settings["area"] = "8"
	joined, err := reg.JoinRoom(created.ID, Member{Connection: "guest", Profile: []byte{4}})
	if err != nil {
		t.Fatal(err)
	}
	joined.Members[0].Profile[0] = 5
	again, _ := reg.GetRoom(created.ID)
	if again.RuleValues[0] != 1 || again.Settings["area"] != "1" || again.Members[0].Profile[0] != 7 {
		t.Fatal("read/join snapshot aliases live state")
	}
}

func TestRelayDepartureAndRoomClosure(t *testing.T) {
	reg := NewRegistry()
	rm := reg.CreateRoom(Room{Game: GameFile2, HostConn: "host", MaxPlayers: 4, Members: []Member{{Connection: "host"}, {Connection: "guest"}, {Connection: "other"}}})
	host := reg.AttachGameRelay(rm.ID, "host")
	guest := reg.AttachGameRelay(rm.ID, "guest")
	other := reg.AttachGameRelay(rm.ID, "other")
	unauthorized := reg.AttachGameRelay(rm.ID, "stranger")
	if _, ok := <-unauthorized; ok {
		t.Fatal("unregistered participant attached")
	}
	reg.FanOutGamePacket(GamePacket{RoomID: rm.ID, FromConn: "stranger", Payload: []byte{1}})
	select {
	case <-host:
		t.Fatal("unregistered sender relayed a frame")
	default:
	}
	payload := []byte{3}
	reg.FanOutGamePacket(GamePacket{RoomID: rm.ID, FromConn: "host", Payload: payload})
	payload[0] = 9
	first := <-guest
	first.Payload[0] = 8
	if second := <-other; second.Payload[0] != 3 {
		t.Fatal("peers share mutable frame storage")
	}
	reg.LeaveRoom(rm.ID, "guest")
	if _, ok := <-guest; ok {
		t.Fatal("departed relay not closed")
	}
	reg.LeaveRoom(rm.ID, "host")
	if _, ok := <-host; ok {
		t.Fatal("host relay not closed")
	}
	if _, ok := <-other; ok {
		t.Fatal("room closure left relay waiting")
	}
}

func TestConcurrentRelayPublicationAndDetach(t *testing.T) {
	reg := NewRegistry()
	rm := reg.CreateRoom(Room{Game: GameFile2, HostConn: "host", MaxPlayers: 2, Members: []Member{{Connection: "host"}, {Connection: "guest"}}})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 2000; i++ {
			reg.FanOutGamePacket(GamePacket{RoomID: rm.ID, FromConn: "host", Payload: []byte{1}})
		}
	}()
	for i := 0; i < 2000; i++ {
		reg.AttachGameRelay(rm.ID, "guest")
		reg.DetachGameRelay(rm.ID, "guest")
	}
	<-done
}

func TestChatUnsubscribeIsSafeDuringPublication(t *testing.T) {
	reg := NewRegistry()
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 2000; i++ {
			reg.PublishChat(ChatMessage{Game: GameFile2, Text: "hello"})
		}
	}()
	for i := 0; i < 2000; i++ {
		ch := reg.SubscribeChat(GameFile2)
		reg.UnsubscribeChat(GameFile2, ch)
		reg.UnsubscribeChat(GameFile2, ch)
	}
	<-done
	if len(reg.chatSubs) != 0 {
		t.Fatal("chat subscriptions retained after disconnect")
	}
}
