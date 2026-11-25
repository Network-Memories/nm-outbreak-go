package snap

import (
	"net"
	"networkmemories/nm-outbreak-go/internal/core"
	"networkmemories/nm-outbreak-go/internal/modules/file2"
	"networkmemories/nm-outbreak-go/internal/world"
	"testing"
	"time"
)

func TestAuthenticatedHeaderAliasIsBoundToExactNativeContext(t *testing.T) {
	e := core.New(nil, fixtureAuthority{}, file2.Definition())
	actor, _ := e.Login(world.GameFile2, core.SNAP, "pal", "aaaaa", "verified-fixture-password")
	s := NewTransport("file2", world.GameFile2, "127.0.0.1", 0, e)
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
	foreign, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer foreign.Close()
	if err = s.adoptSession(42, client.LocalAddr(), actor.ID); err != nil {
		t.Fatal(err)
	}
	s.bindings.ClientAlias(42, 99)
	p := make([]byte, 240)
	p[200] = 3
	s.handleMessage(server, foreign.LocalAddr(), Message{SessionID: 99, Command: CmdChangeUserProperty, Payload: p})
	got, _ := e.Session(actor.ID)
	if got.Profile != nil {
		t.Fatal("shared public IP was sufficient to take an identity")
	}
	s.handleMessage(server, client.LocalAddr(), Message{SessionID: 100, Command: CmdChangeUserProperty, Payload: p})
	got, _ = e.Session(actor.ID)
	if got.Profile != nil {
		t.Fatal("unknown alias acquired existing context")
	}
	s.handleMessage(server, client.LocalAddr(), Message{SessionID: 99, Command: CmdChangeUserProperty, Payload: p})
	client.SetReadDeadline(time.Now().Add(time.Second))
	buf := make([]byte, 1024)
	n, _, err := client.ReadFrom(buf)
	if err != nil {
		t.Fatal(err)
	}
	reply, err := DecodeDatagram(buf[:n])
	got, _ = e.Session(actor.ID)
	if err != nil || reply.SessionID != 42 || got.Profile[200] != 3 {
		t.Fatal("proved context did not normalize native header alias", err)
	}
}
func TestKICSRestartDoesNotConsumeFirstApplicationSequence(t *testing.T) {
	r := newReliableTransport()
	remote := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 31234}
	r.receive(remote, []Message{{SessionID: 42, TypeFlags: FlagReliable, PacketSeq: 2, Command: CmdLoginToKics}})
	r.restartIncoming(remote, 42)
	deliver, ack, _ := r.receive(remote, []Message{{SessionID: 42, TypeFlags: FlagReliable, PacketSeq: 0, Command: CmdQueryArea}})
	if len(deliver) != 1 || !ack {
		t.Fatal("bootstrap/KICS counter leaked into application receive window")
	}
}
