package snap

import (
	"encoding/binary"
	"net"
	"networkmemories/nm-outbreak-go/internal/core"
	"networkmemories/nm-outbreak-go/internal/modules/file1"
	"networkmemories/nm-outbreak-go/internal/modules/file2"
	"networkmemories/nm-outbreak-go/internal/world"
	"testing"
	"time"
)

// Original File1 sends a reliable lobby leave before joining the selected
// free-mode area. An ACK alone, or a room-channel result, leaves that callback
// waiting forever. File2 uses the same channel distinction.
func TestNativeLeaveCompletesCorrectChannelAndPreservesLobbyOnRoomLeave(t *testing.T) {
	for _, game := range []world.Game{world.GameFile1, world.GameFile2} {
		for _, channel := range []uint16{FlagChannelBits, FlagRoom} {
			t.Run(string(game)+"/"+map[uint16]string{FlagChannelBits: "area", FlagRoom: "room"}[channel], func(t *testing.T) {
				e := core.New(nil, fixtureAuthority{}, file1.Definition(), file2.Definition())
				actor, err := e.Login(game, core.SNAP, "ntsc-u", "aaaaa", "verified-fixture-password")
				if err != nil {
					t.Fatal(err)
				}
				native := uint32(SharedAreaToLobby(game, 1))
				e.SetArea(actor.ID, 1)
				e.SetNativeLobby(actor.ID, native)
				if err = e.SetProfile(actor.ID, make([]byte, 240)); err != nil {
					t.Fatal(err)
				}
				room, err := e.CreateRoom(actor.ID, core.RoomOptions{Area: 1, Scenario: 1, MaxPlayers: 2})
				if err != nil {
					t.Fatal(err)
				}
				server, err := net.ListenPacket("udp", "127.0.0.1:0")
				if err != nil {
					t.Fatal(err)
				}
				defer server.Close()
				client, err := net.ListenPacket("udp", "127.0.0.1:0")
				if err != nil {
					t.Fatal(err)
				}
				defer client.Close()
				s := NewTransport("fixture", game, "127.0.0.1", 0, e)
				if err = s.adoptSession(42, client.LocalAddr(), actor.ID); err != nil {
					t.Fatal(err)
				}
				s.handleMessage(server, client.LocalAddr(), Message{TypeFlags: FlagReliable | channel, Command: CmdLeave, SessionID: 42, PacketSeq: 6})
				client.SetReadDeadline(time.Now().Add(time.Second))
				buf := make([]byte, 1024)
				n, _, err := client.ReadFrom(buf)
				if err != nil {
					t.Fatal(err)
				}
				m, err := DecodeDatagram(buf[:n])
				if err != nil || m.Command != CmdResultWrapper || m.TypeFlags != FlagReliable|FlagResponse|channel || len(m.Payload) != 8 || binary.BigEndian.Uint32(m.Payload) != uint32(CmdLeave) || binary.BigEndian.Uint32(m.Payload[4:]) != ResultOK {
					t.Fatalf("native leave callback rejected: %+v, %v", m, err)
				}
				got, _ := e.Session(actor.ID)
				if got.RoomID != "" {
					t.Fatal("room membership retained")
				}
				if _, exists := e.Worlds().GetRoom(room.ID); exists {
					t.Fatal("empty room retained")
				}
				if channel == FlagChannelBits {
					if got.Area != 0 || got.NativeLobbyID != 0 {
						t.Fatal("departed area still counted")
					}
				} else if got.Area != 1 || got.NativeLobbyID != native {
					t.Fatal("room leave removed area presence")
				}
			})
		}
	}
}
