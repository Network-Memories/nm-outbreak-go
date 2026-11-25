package kddi

import (
	"bytes"
	"context"
	"net"
	"networkmemories/nm-outbreak-go/internal/core"
	"networkmemories/nm-outbreak-go/internal/world"
	"testing"
	"time"
)

// Literal native opcodes and fields exercise an authenticated KDDI connection
// joining a room owned by the western adapter in the same core.
func TestNativeKDDIAdmissionUsesWesternRoomPasswordAndRosterBothGames(t *testing.T) {
	for _, game := range []world.Game{world.GameFile1, world.GameFile2} {
		t.Run(string(game), func(t *testing.T) {
			e := nativeEngine(nil)
			region := "pal"
			if game == world.GameFile1 {
				region = "ntsc-u"
			}
			host, err := e.Login(game, core.SNAP, region, "WesternHost", "nativeproof")
			if err != nil {
				t.Fatal(err)
			}
			profile := make([]byte, 240)
			profile[200] = 4
			if err = e.SetProfile(host.ID, profile); err != nil {
				t.Fatal(err)
			}
			room, err := e.CreateRoom(host.ID, core.RoomOptions{Area: 1, Scenario: 1, MaxPlayers: 2, Password: []byte("roomkey")})
			if err != nil {
				t.Fatal(err)
			}
			ticket, err := e.LoginKDDI(game, "NativeOne", "nativeproof")
			if err != nil {
				t.Fatal(err)
			}
			server, client := net.Pipe()
			defer client.Close()
			client.SetDeadline(time.Now().Add(3 * time.Second))
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			go NewTransport("fixture-lobby", game, "127.0.0.1", 0, e).serveConn(ctx, server)
			completeHandshake(t, client, ticket)
			query := func(cmd uint16, payload []byte) LobbyFrame {
				t.Helper()
				if err := writeFrame(client, LobbyFrame{Sender: 0x81, Kind: 1, Command: cmd, PacketID: 23, Payload: payload}); err != nil {
					t.Fatal(err)
				}
				r, err := readFrame(client)
				for err == nil && r.Kind == 0x10 {
					r, err = readFrame(client)
				}
				if err != nil || r.Command != cmd || r.PacketID != 23 {
					t.Fatal("native reply", r, err)
				}
				return r
			}
			area := byte(1)
			if game == world.GameFile1 {
				area = 2
			}
			if query(0x6207, []byte{0, area}).Error != 0 || query(0x6305, []byte{0, 1}).Error != 0 {
				t.Fatal("native browse")
			}
			jp := make([]byte, 208)
			jp[200] = 1
			if query(0x6190, nativeField(jp, 23)).Error != 0 {
				t.Fatal("native profile")
			}
			preview := query(0x640a, []byte{0, 1})
			if preview.Error != 0 || len(preview.Payload) < 14 || !bytes.Equal(preview.Payload[:12], []byte{0, 1, 3, 1, 0, 6, 'H', 'S', 'T', '0', '0', '1'}) {
				t.Fatal("western owner preview not projected", preview)
			}
			join := func(password string) LobbyFrame {
				return query(0x6406, append([]byte{0, 1}, nativeField([]byte(password), 23)...))
			}
			if join("wrong").Error != 0xff {
				t.Fatal("wrong room password accepted")
			}
			snapshot, _ := e.Worlds().GetRoom(room.ID)
			if len(snapshot.Members) != 1 {
				t.Fatal("rejected join mutated membership")
			}
			e.SetRoomStat(host.ID, 0x40000000)
			if join("roomkey").Error != 0xff {
				t.Fatal("closed native admission accepted")
			}
			e.SetRoomStat(host.ID, 0)
			for i := 0; i < 2; i++ {
				r := join("roomkey")
				if r.Error != 0 || !bytes.Equal(r.Payload, []byte{0, 1}) {
					t.Fatal("proved native join failed", r)
				}
			}
			snapshot, _ = e.Worlds().GetRoom(room.ID)
			if len(snapshot.Members) != 2 {
				t.Fatal("native join not shared or replay duplicated it")
			}
			if r := query(0x6504, []byte{1}); r.Error != 0 || !bytes.Equal(r.Payload, []byte{1}) {
				t.Fatal("guest completion was mistaken for host creation", r)
			}
			if r := query(0x6911, nil); r.Error != 0 || !bytes.Equal(r.Payload, []byte{2}) {
				t.Fatal("native count not common membership", r)
			}
			if r := query(0x6912, nil); r.Error != 0 || !bytes.Equal(r.Payload, []byte{2}) {
				t.Fatal("native seat not common seat", r)
			}
			stat := query(0x6913, []byte{1})
			if stat.Error != 0 || !bytes.Equal(stat.Payload[:10], []byte{1, 1, 0, 6, 'H', 'S', 'T', '0', '0', '1'}) || !bytes.Equal(stat.Payload[len(stat.Payload)-3:], []byte{0, 0, 6}) {
				t.Fatal("native western player record", stat)
			}
			if r := query(0x6501, nil); r.Error != 0 || len(r.Payload) != 0 {
				t.Fatal("native leave", r)
			}
			snapshot, _ = e.Worlds().GetRoom(room.ID)
			if len(snapshot.Members) != 1 || snapshot.HostConn != host.ID {
				t.Fatal("native guest leave removed western host")
			}
		})
	}
}
