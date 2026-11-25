package kddi

import (
	"bytes"
	"context"
	"encoding/hex"
	"net"
	"networkmemories/nm-outbreak-go/internal/core"
	"networkmemories/nm-outbreak-go/internal/modules/file2"
	"networkmemories/nm-outbreak-go/internal/world"
	"testing"
	"time"
)

func TestCapturedChatInProducesAuthenticatedNativeEchoAndSharedEvent(t *testing.T) {
	for _, aftergame := range []bool{false, true} {
		name := "waiting04"
		if aftergame {
			name = "aftergame0c"
		}
		t.Run(name, func(t *testing.T) {
			e := nativeEngine(nil)
			host, _ := e.Login(world.GameFile2, core.SNAP, "pal", "WesternHost", "nativeproof")
			ticket, _ := e.LoginKDDI(world.GameFile2, "NativeOne", "nativeproof")
			guest, _ := e.KDDIHandoff(world.GameFile2, ticket)
			e.SetProfile(host.ID, make([]byte, 240))
			e.SetProfile(guest.ID, make([]byte, 208))
			room, _ := e.CreateRoom(host.ID, core.RoomOptions{Area: 1, Scenario: 1, MaxPlayers: 2})
			e.JoinRoom(guest.ID, room.ID)
			if aftergame {
				e.ScenarioPresence(host.ID, true)
				e.ScenarioPresence(guest.ID, true)
				e.ScenarioPresence(host.ID, false)
				e.ScenarioPresence(guest.ID, false)
			}
			hs, _ := e.SubscribeRoom(host.ID)
			a, b := net.Pipe()
			defer a.Close()
			defer b.Close()
			b.SetDeadline(time.Now().Add(time.Second))
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			srv := NewTransport("file2-lobby", world.GameFile2, "127.0.0.1", 0, e)
			srv.watchNativeRoom(ctx, a, guest.ID, func() uint16 { return 17 })
			plain, _ := hex.DecodeString("82838283828382838283")
			trailer := byte(4)
			if aftergame {
				trailer = 12
			}
			p := append(nativeField(plain, 42), trailer)
			// An extra trailer byte and query kind must never publish or echo a message.
			bad := append([]byte(nil), p...)
			bad = append(bad, 255)
			srv.handleChat(a, LobbyFrame{Command: 0x6701, Kind: 16, PacketID: 42, Payload: bad}, guest.ID)
			srv.handleChat(a, LobbyFrame{Command: 0x6701, Kind: 1, PacketID: 42, Payload: p}, guest.ID)
			if len(hs.Events()) != 0 {
				t.Fatal("unsupported native frame published")
			}
			if !srv.handleChat(a, LobbyFrame{Command: 0x6701, Kind: 16, PacketID: 42, Payload: p}, guest.ID) {
				t.Fatal("CHATIN unhandled")
			}
			echo, err := readFrame(b)
			if err != nil {
				t.Fatal(err)
			}
			want, err := file2.EncodeJapaneseChat(guest.Account.Handle, guest.Account.NativeNickname, "ｃｃｃｃｃ")
			if err != nil {
				t.Fatal(err)
			}
			if echo.Command != 0x6702 || echo.Kind != 16 || echo.Sender != SenderServer || !bytes.Equal(echo.Payload, want) {
				t.Fatal("native echo identity or text changed")
			}
			ev := <-hs.Events()
			if ev.Kind != core.RoomChat || ev.Actor.ID != guest.ID || ev.ChatText != "ｃｃｃｃｃ" || ev.ChatAftergame != aftergame {
				t.Fatal("shared event does not match original native input")
			}
		})
	}
}
