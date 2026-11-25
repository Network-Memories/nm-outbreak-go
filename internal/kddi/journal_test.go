package kddi

import (
	"bytes"
	"net"
	"networkmemories/nm-outbreak-go/internal/core"
	"networkmemories/nm-outbreak-go/internal/modules/file2"
	"networkmemories/nm-outbreak-go/internal/world"
	"testing"
	"time"
)

func TestEventDatAcknowledgesOnlyRetainedAuthenticatedJournal(t *testing.T) {
	e := nativeEngine(nil)
	host, _ := e.Login(world.GameFile2, core.SNAP, "pal", "WesternHost", "nativeproof")
	ticket, _ := e.LoginKDDI(world.GameFile2, "NativeOne", "nativeproof")
	guest, _ := e.KDDIHandoff(world.GameFile2, ticket)
	e.SetProfile(host.ID, make([]byte, 240))
	e.SetProfile(guest.ID, make([]byte, 208))
	room, err := e.CreateRoom(host.ID, core.RoomOptions{Area: 1, Scenario: 1, MaxPlayers: 2})
	if err != nil {
		t.Fatal(err)
	}
	e.JoinRoom(guest.ID, room.ID)
	e.SubscribeRoom(host.ID)
	e.SubscribeRoom(guest.ID)
	srv := NewTransport("file2-lobby", world.GameFile2, "127.0.0.1", 0, e)
	blocks, _ := file2.JournalBlocks(make([]byte, 2048), true)
	const pid = 42
	payload := append(nativeField([]byte(host.Account.Handle), pid), nativeField(blocks[0], pid)...)
	query := func(p []byte) LobbyFrame {
		t.Helper()
		a, b := net.Pipe()
		defer a.Close()
		defer b.Close()
		b.SetDeadline(time.Now().Add(time.Second))
		done := make(chan bool, 1)
		go func() {
			done <- srv.handleJournal(a, LobbyFrame{Sender: 0x81, Kind: 1, Command: 0x670d, PacketID: pid, Payload: p}, guest.ID)
		}()
		r, err := readFrame(b)
		if err != nil {
			t.Fatal(err)
		}
		if !<-done || r.Command != 0x670d || r.Kind != 2 || r.PacketID != pid {
			t.Fatal("reply envelope", r)
		}
		return r
	}
	if query(payload).Error != 0xff {
		t.Fatal("unproven aftergame acknowledged")
	}
	e.ScenarioPresence(host.ID, true)
	e.ScenarioPresence(guest.ID, true)
	e.ScenarioPresence(host.ID, false)
	e.ScenarioPresence(guest.ID, false)
	for _, p := range [][]byte{nil, payload[:len(payload)-1], append(append([]byte(nil), payload...), 0), append(nativeField([]byte("ABSENT"), pid), nativeField(blocks[0], pid)...)} {
		if query(p).Error != 0xff {
			t.Fatal("malformed or foreign destination acknowledged")
		}
	}
	r := query(payload)
	if r.Error != 0 || !bytes.Equal(r.Payload, append(word(6), []byte(host.Account.Handle)...)) {
		t.Fatal("retained chunk missing native ACK", r)
	}
	e.LeaveRoom(guest.ID)
	if query(payload).Error != 0xff {
		t.Fatal("departed session acknowledged")
	}
}

func TestFile1WaitingGreetingAllowedButJournalOutsideRoundRefused(t *testing.T) {
	e := nativeEngine(nil)
	ticket, _ := e.LoginKDDI(world.GameFile1, "NativeOne", "nativeproof")
	host, _ := e.KDDIHandoff(world.GameFile1, ticket)
	guest, _ := e.Login(world.GameFile1, core.SNAP, "ntsc-u", "WesternGuest", "nativeproof")
	e.SetProfile(host.ID, make([]byte, 208))
	e.SetProfile(guest.ID, make([]byte, 240))
	room, err := e.CreateRoom(host.ID, core.RoomOptions{Area: 1, Scenario: 1, MaxPlayers: 2})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.JoinRoom(guest.ID, room.ID); err != nil {
		t.Fatal(err)
	}
	sub, err := e.SubscribeRoom(guest.ID)
	if err != nil {
		t.Fatal(err)
	}
	srv := NewTransport("file1-lobby", world.GameFile1, "127.0.0.1", 0, e)
	const pid = 43
	query := func(event []byte) LobbyFrame {
		t.Helper()
		p := append(nativeField([]byte(guest.Account.Handle), pid), nativeField(event, pid)...)
		a, b := net.Pipe()
		defer a.Close()
		defer b.Close()
		b.SetDeadline(time.Now().Add(time.Second))
		done := make(chan bool, 1)
		go func() {
			done <- srv.handleJournal(a, LobbyFrame{Sender: 0x81, Kind: 1, Command: 0x670d, PacketID: pid, Payload: p}, host.ID)
		}()
		r, err := readFrame(b)
		if err != nil {
			t.Fatal(err)
		}
		if !<-done || r.Command != 0x670d || r.Kind != 2 || r.PacketID != pid {
			t.Fatal("unexpected native reply", r)
		}
		return r
	}
	if r := query(make([]byte, 8)); r.Error != 0 || !bytes.Equal(r.Payload, append(word(6), []byte(guest.Account.Handle)...)) {
		t.Fatalf("File1 0x670d waiting greeting rejected: %+v", r)
	}
	select {
	case ev := <-sub.Events():
		if ev.Kind != core.RoomWaitingGreeting {
			t.Fatalf("expected RoomWaitingGreeting, got %v", ev.Kind)
		}
	case <-time.After(time.Second):
		t.Fatal("missing RoomWaitingGreeting event for File1 SNAP guest")
	}
	blocks, _ := file2.JournalBlocks(make([]byte, 2048), true)
	if r := query(blocks[0]); r.Error != 0xff {
		t.Fatalf("File1 journal outside an actual returned round should be refused, got err=%d", r.Error)
	}
}
