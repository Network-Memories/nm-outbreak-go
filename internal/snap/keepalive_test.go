package snap

import (
	"net"
	"networkmemories/nm-outbreak-go/internal/core"
	"networkmemories/nm-outbreak-go/internal/modules/file2"
	"networkmemories/nm-outbreak-go/internal/world"
	"testing"
	"time"
)

func TestIdleNativeKeepaliveAndACKDoNotRunApplicationCallback(t *testing.T) {
	e := core.New(nil, fixtureAuthority{}, file2.Definition())
	actor, err := e.Login(world.GameFile2, core.SNAP, "pal", "aaaaa", "verified-fixture-password")
	if err != nil {
		t.Fatal(err)
	}
	s := NewTransport("file2", world.GameFile2, "127.0.0.1", 0, e)
	server, _ := net.ListenPacket("udp", "127.0.0.1:0")
	defer server.Close()
	client, _ := net.ListenPacket("udp", "127.0.0.1:0")
	defer client.Close()
	if err = s.adoptSession(42, client.LocalAddr(), actor.ID); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	s.reliable.clock = func() time.Time { return now }
	s.reliable.receive(client.LocalAddr(), []Message{{SessionID: 42, Command: CmdAck}})
	now = now.Add(21 * time.Second)
	s.keepAlive(server, now)
	client.SetReadDeadline(time.Now().Add(time.Second))
	buf := make([]byte, 1024)
	n, _, err := client.ReadFrom(buf)
	if err != nil {
		t.Fatal(err)
	}
	m, err := DecodeDatagram(buf[:n])
	if err != nil || m.TypeFlags != FlagReliable || m.Command != CmdAck || len(m.Payload) != 0 || m.PacketSeq != 0 {
		t.Fatal("unsafe or malformed native keepalive", err)
	}
	now = now.Add(21 * time.Second)
	s.keepAlive(server, now)
	client.SetReadDeadline(time.Now().Add(10 * time.Millisecond))
	if _, _, err = client.ReadFrom(buf); err == nil {
		t.Fatal("new keepalive queued before ACK")
	}
	s.reliable.receive(client.LocalAddr(), []Message{{SessionID: 42, Command: CmdAck, TypeFlags: FlagResponse, AckNumber: 0}})
	now = now.Add(21 * time.Second)
	s.keepAlive(server, now)
	client.SetReadDeadline(time.Now().Add(time.Second))
	n, _, err = client.ReadFrom(buf)
	if err != nil {
		t.Fatal(err)
	}
	m, err = DecodeDatagram(buf[:n])
	if err != nil || m.PacketSeq != 1 {
		t.Fatal("keepalive sequence did not continue")
	}
	if len(e.Worlds().ListRooms(world.GameFile2)) != 0 {
		t.Fatal("keepalive created fake presence/room")
	}
}
