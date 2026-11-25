package kddi

import (
	"context"
	"net"
	"networkmemories/nm-outbreak-go/internal/world"
	"testing"
	"time"
)

func TestNativeExplicitLogoutRevokesTicketButPreservesOwner(t *testing.T) {
	for _, game := range []world.Game{world.GameFile1, world.GameFile2} {
		t.Run(string(game), func(t *testing.T) {
			e := nativeEngine(nil)
			ticket, err := e.LoginKDDI(game, "NativeOne", "nativeproof")
			if err != nil {
				t.Fatal(err)
			}
			before, _ := e.KDDIHandoff(game, ticket)
			a, b := net.Pipe()
			defer b.Close()
			b.SetDeadline(time.Now().Add(3 * time.Second))
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan struct{})
			go func() { defer close(done); NewTransport("fixture-lobby", game, "127.0.0.1", 0, e).serveConn(ctx, a) }()
			completeHandshake(t, b, ticket)
			if err := writeFrame(b, LobbyFrame{Sender: 0x81, Kind: 1, Command: 0x6006, PacketID: 27}); err != nil {
				t.Fatal(err)
			}
			r, err := readFrame(b)
			if err != nil || r.Command != 0x6006 || r.Kind != 2 || r.PacketID != 27 || r.Error != 0 || len(r.Payload) != 0 {
				t.Fatal(r, err)
			}
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("explicit logout did not close transport")
			}
			if _, err := e.KDDIHandoff(game, ticket); err == nil {
				t.Fatal("logout retained usable ticket")
			}
			next, err := e.LoginKDDI(game, "NativeOne", "nativeproof")
			if err != nil {
				t.Fatal(err)
			}
			after, err := e.KDDIHandoff(game, next)
			if err != nil || after.Account.UserID != before.Account.UserID {
				t.Fatal("logout changed account owner", err)
			}
		})
	}
}
