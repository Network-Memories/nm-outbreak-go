package kddi

import (
	"bytes"
	"context"
	"net"
	"networkmemories/nm-outbreak-go/internal/world"
	"testing"
	"time"
)

func TestNativeCreationWireStagesBothGames(t *testing.T) {
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
			go NewTransport("fixture-lobby", game, "127.0.0.1", 0, e).serveConn(ctx, a)
			completeHandshake(t, b, ticket)
			query := func(cmd uint16, p []byte) LobbyFrame {
				t.Helper()
				if err := writeFrame(b, LobbyFrame{Sender: 0x81, Kind: 1, Command: cmd, PacketID: 29, Payload: p}); err != nil {
					t.Fatal(err)
				}
				r, err := readFrame(b)
				for err == nil && r.Kind == 0x10 {
					r, err = readFrame(b)
				}
				if err != nil || r.Command != cmd || r.PacketID != 29 {
					t.Fatal("native creation reply", r, err)
				}
				return r
			}
			area, rules := byte(1), byte(3)
			if game == world.GameFile1 {
				area = 2
				rules = 4
			}
			query(0x6207, []byte{0, area})
			query(0x6305, []byte{0, 1})
			if r := query(0x6603, []byte{0, 1}); r.Error != 0 || !bytes.Equal(r.Payload, []byte{rules}) {
				t.Fatal("native rule count", r)
			}
			if r := query(0x6407, []byte{0, 1}); r.Error != 0 || !bytes.Equal(r.Payload, []byte{0, 1}) {
				t.Fatal("native reservation", r)
			}
			if r := query(0x6404, []byte{0, 1}); r.Error != 0 || !bytes.Equal(r.Payload, []byte{0, 1, 2}) {
				t.Fatal("creating slot not reserved", r)
			}
			if len(e.Worlds().ListRooms(game)) != 0 {
				t.Fatal("native draft appeared as public party")
			}
			if r := query(0x6504, []byte{1}); r.Error != 0xff {
				t.Fatal("incomplete creation finalized")
			}
			mediaByte := byte(0x11)
			if game == world.GameFile1 {
				mediaByte = 0x01
			}
			if r := query(0x6509, []byte{0, mediaByte, 0, 1}); r.Error != 0 || !bytes.Equal(r.Payload, []byte{0, 1, 0, mediaByte, 0, 1}) {
				t.Fatal("native scene selection", r)
			}
			if query(0x6609, nativeField([]byte("native"), 29)).Error != 0 || query(0x660a, nativeField([]byte("roomkey"), 29)).Error != 0 {
				t.Fatal("native title/password")
			}
			for i, v := range []byte{0, 4, 2} {
				if query(0x660b, []byte{byte(i), v}).Error != 0 {
					t.Fatal("native rule setter")
				}
			}
			if r := query(0x6606, []byte{0, 1, 1}); r.Error != 0 || !bytes.Equal(r.Payload, []byte{1, 4}) {
				t.Fatal("native chosen rule not retained", r)
			}
			p := make([]byte, 208)
			p[200] = 1
			if query(0x6190, nativeField(p, 29)).Error != 0 {
				t.Fatal("native profile")
			}
			for i := 0; i < 2; i++ {
				if r := query(0x6504, []byte{1}); r.Error != 0 || !bytes.Equal(r.Payload, []byte{1}) {
					t.Fatal("native finalization", r)
				}
			}
			rooms := e.Worlds().ListRooms(game)
			if len(rooms) != 1 || rooms[0].Title != "native" || rooms[0].MaxPlayers != 2 || rooms[0].Difficulty != 2 || !rooms[0].Protected {
				t.Fatal("native room not committed to shared registry", rooms)
			}
			if r := query(0x6404, []byte{0, 1}); r.Error != 0 || !bytes.Equal(r.Payload, []byte{0, 1, 3}) {
				t.Fatal("committed slot not recruiting", r)
			}
			if r := query(0x6501, nil); r.Error != 0 {
				t.Fatal("cancel published native room", r)
			}
			if len(e.Worlds().ListRooms(game)) != 0 {
				t.Fatal("native cancel retained party")
			}
		})
	}
}
