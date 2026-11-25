package kddi

import (
	"bytes"
	"context"
	"net"
	"networkmemories/nm-outbreak-go/internal/world"
	"testing"
	"time"
)

func TestAuthenticatedNativeAreaRoomAndSlotNavigationBothGames(t *testing.T) {
	for _, game := range []world.Game{world.GameFile1, world.GameFile2} {
		t.Run(string(game), func(t *testing.T) {
			e := nativeEngine(nil)
			ticket, err := e.LoginKDDI(game, "NativeOne", "nativeproof")
			if err != nil {
				t.Fatal(err)
			}
			a, b := net.Pipe()
			defer b.Close()
			b.SetDeadline(time.Now().Add(3 * time.Second))
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan struct{})
			go func() { defer close(done); NewTransport("fixture-lobby", game, "127.0.0.1", 0, e).serveConn(ctx, a) }()
			completeHandshake(t, b, ticket)
			query := func(cmd uint16, payload []byte) LobbyFrame {
				t.Helper()
				if err := writeFrame(b, LobbyFrame{Sender: 0x81, Kind: 1, Command: cmd, PacketID: 19, Payload: payload}); err != nil {
					t.Fatal(err)
				}
				r, err := readFrame(b)
				if err != nil || r.Sender != 0x18 || r.Kind != 2 || r.Command != cmd || r.PacketID != 19 {
					t.Fatalf("native reply envelope: %+v %v", r, err)
				}
				return r
			}

			if r := query(0x6145, []byte{0, 0}); r.Error != 0xff || len(r.Payload) != 0 {
				t.Fatal("unimplemented ranking fabricated data")
			}
			catalog := query(0x6881, nil)
			if game == world.GameFile2 {
				if catalog.Error != 0 || !bytes.Equal(catalog.Payload, []byte{0}) {
					t.Fatal("empty File2 download catalog missing")
				}
			} else if catalog.Error != 0xff {
				t.Fatal("File2 download variant leaked into File1")
			}
			if query(0x6881, []byte{0}).Error != 0xff || query(0x6882, nil).Error != 0xff {
				t.Fatal("unadvertised download accepted")
			}
			area, areaCount, roomCount := byte(1), byte(10), byte(6)
			if game == world.GameFile1 {
				area = 2
				areaCount = 2
				roomCount = 10
			}
			checks := []struct {
				cmd      uint16
				in, want []byte
			}{
				{0x614c, nil, []byte{0, 0, 0}},
				{0x6203, nil, []byte{0, areaCount}},
				{0x6206, []byte{0, area}, []byte{0, area, 3}},
				{0x6207, []byte{0, area}, []byte{0, area}},
				{0x6301, nil, []byte{0, roomCount}},
				{0x6305, []byte{0, 1}, []byte{0, 1}},
				{0x6401, nil, []byte{0, 20}},
				{0x6404, []byte{0, 1}, []byte{0, 1, 1}},
				{0x6408, nil, nil},
				{0x6209, nil, nil},
			}
			for _, c := range checks {
				r := query(c.cmd, c.in)
				if r.Error != 0 || !bytes.Equal(r.Payload, c.want) {
					t.Fatalf("opcode %04x: got %x error %x want %x", c.cmd, r.Payload, r.Error, c.want)
				}
			}
			for _, invalid := range [][]byte{{0}, {0, 1, 0}, {0, 99}} {
				if r := query(0x614c, invalid); r.Error != 0xff {
					t.Fatal("malformed message-of-day query accepted")
				}
				if r := query(0x6207, invalid); r.Error != 0xff {
					t.Fatal("malformed/nonexistent area accepted")
				}
			}
			actor, _ := e.KDDIHandoff(game, ticket)
			if actor.NativeLobbyID != 0 || actor.Area != 0 || actor.NativeRoomID != 0 {
				t.Fatal("bad navigation request changed state")
			}
			query(0x6207, []byte{0, area})
			variant := query(0x6209, make([]byte, 6))
			if variant.Error != 0 {
				t.Fatalf("original %s exit shape rejected", game)
			}
			if query(0x6209, []byte{0, 0, 0, 0, 0, 1}).Error != 0xff {
				t.Fatal("unmeasured exit body accepted")
			}
			b.Close()
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("closed native lobby still running")
			}
			actor, _ = e.KDDIHandoff(game, ticket)
			if actor.NativeLobbyID != 0 || actor.NativeRoomID != 0 || actor.Area != 0 {
				t.Fatal("closed native socket kept browse presence")
			}
		})
	}
}
