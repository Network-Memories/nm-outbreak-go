package kddi

import (
	"bytes"
	"context"
	"net"
	"networkmemories/nm-outbreak-go/internal/core"
	"testing"
	"time"

	"networkmemories/nm-outbreak-go/internal/world"
)

func TestKDDICodecReadsSharedRoomFixture(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reg := world.NewRegistry()
	// Simulate a PAL File #2 host creating a room in memory (zero HTTP bridge).
	reg.CreateRoom(world.Room{
		Game:       world.GameFile2,
		HostConn:   "snap:aaaaa",
		HostSeat:   0,
		Area:       1,
		Scenario:   1,
		Title:      "PAL-ROOM",
		MaxPlayers: 4,
		Members: []world.Member{{
			Connection: "snap:aaaaa",
			Adapter:    "snap-file2",
			Name:       "aaaaa",
			Region:     "pal",
			Seat:       0,
		}},
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	engine := nativeEngine(reg)
	ticket, err := engine.LoginKDDI(world.GameFile2, "NativeOne", "nativeproof")
	if err != nil {
		t.Fatal(err)
	}
	srv := NewTransport("file2-lobby", world.GameFile2, "127.0.0.1", 0, engine)
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go srv.serveConn(ctx, c)
		}
	}()

	client, err := net.DialTimeout("tcp", ln.Addr().String(), 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()
	_ = client.SetDeadline(time.Now().Add(2 * time.Second))

	completeHandshake(t, client, ticket)
	var ls LobbyStream
	query := func(cmd uint16, payload []byte) LobbyFrame {
		raw, _ := LobbyFrame{Sender: 0x81, Kind: 0x01, Command: cmd, PacketID: 1, Payload: payload}.Encode()
		if _, err := client.Write(raw); err != nil {
			t.Fatalf("write: %v", err)
		}
		buf := make([]byte, 1024)
		for {
			n, err := client.Read(buf)
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			if frames := ls.Feed(buf[:n]); len(frames) > 0 {
				return frames[0]
			}
		}
	}

	query(0x6207, []byte{0, 1})
	query(0x6305, []byte{0, 1})
	// 1. Query SLOTSTATUS for slot 1 -> must be recruiting (3)
	st := query(CmdSlotStatus, []byte{0x00, 0x01})
	if len(st.Payload) != 3 || st.Payload[2] != 3 {
		t.Fatalf("expected slot 1 status=3, got %x", st.Payload)
	}

	// 2. Query SLOTTITLE for slot 1 -> must be "PAL-ROOM"
	tt := query(CmdSlotTitle, []byte{0x00, 0x01})
	if len(tt.Payload) < 4 || string(tt.Payload[4:]) != "PAL-ROOM" {
		t.Fatalf("expected title PAL-ROOM, got %q", string(tt.Payload[4:]))
	}
}

// Literal opcode and sender from the Java BioServer dialect, not the Go constants:
// this catches accidental self-consistency between a wrong client and wrong server.
func TestReferenceSlotTitleOpcodeAndServerEnvelope(t *testing.T) {
	reg := world.NewRegistry()
	reg.CreateRoom(world.Room{Game: world.GameFile2, Area: 1, HostConn: "host", MaxPlayers: 4, Title: "", Members: []world.Member{{Connection: "host"}}})
	server, client := net.Pipe()
	defer client.Close()
	_ = client.SetDeadline(time.Now().Add(time.Second))
	engine := nativeEngine(reg)
	actor, err := engine.Login(world.GameFile2, core.KDDI, "ntsc-j", "NativeOne", "nativeproof")
	if err != nil {
		t.Fatal(err)
	}
	if err = engine.SetKDDIBrowse(actor.ID, 1, 1); err != nil {
		t.Fatal(err)
	}
	srv := NewTransport("file2-lobby", world.GameFile2, "127.0.0.1", 0, engine)
	go func() {
		defer server.Close()
		srv.handleLobbyFrame(server, LobbyFrame{Sender: 0x81, Kind: 0x01, Command: 0x6402, PacketID: 0x1234, Payload: []byte{0, 1}}, actor.ID)
	}()
	buf := make([]byte, 32)
	n, err := client.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	expected := []byte{0x18, 0x02, 0x64, 0x02, 0, 4, 0x12, 0x34, 0, 0xff, 0xff, 0xff, 0, 1, 0, 0}
	if !bytes.Equal(buf[:n], expected) {
		t.Fatalf("reference title reply: got %x want %x", buf[:n], expected)
	}
}

func TestShutdownReleasesIdleNativeConnection(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server, client := net.Pipe()
	defer client.Close()
	transport := NewTransport("file2-lobby", world.GameFile2, "127.0.0.1", 0, core.New(nil, nil))
	done := make(chan struct{})
	go func() { defer close(done); transport.serveConn(ctx, server) }()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("shutdown left an idle native reader blocked")
	}
}
