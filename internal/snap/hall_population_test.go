package snap

import (
	"encoding/binary"
	"net"
	"networkmemories/nm-outbreak-go/internal/core"
	"networkmemories/nm-outbreak-go/internal/modules/file2"
	"networkmemories/nm-outbreak-go/internal/world"
	"testing"
	"time"
)

func TestCrossHallArrivalUsesNativeAreaCallbackWithoutFakePeer(t *testing.T) {
	e := core.New(nil, fixtureAuthority{}, file2.Definition())
	p, _ := e.Login(world.GameFile2, core.SNAP, "pal", "aaaaa", "verified-fixture-password")
	j, _ := e.Login(world.GameFile2, core.KDDI, "ntsc-j", "ccccc", "verified-fixture-password")
	e.SetArea(p.ID, 1)
	e.SetNativeLobby(p.ID, 1)
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
	done := make(chan struct{})
	defer close(done)
	s := NewTransport("fixture", world.GameFile2, "127.0.0.1", 0, e)
	s.notifyDone = done
	if err = s.adoptSession(42, client.LocalAddr(), p.ID); err != nil {
		t.Fatal(err)
	}
	s.watchRoom(server, p.ID)
	e.AttachKDDILobby(j.ID)
	e.SetKDDIBrowse(j.ID, 1, 0)
	client.SetReadDeadline(time.Now().Add(time.Second))
	buf := make([]byte, 1024)
	n, _, err := client.ReadFrom(buf)
	if err != nil {
		t.Fatal(err)
	}
	m, err := DecodeDatagram(buf[:n])
	if err != nil {
		t.Fatal(err)
	}
	if m.Command != CmdJoin || m.TypeFlags != 0xf000 || m.SessionID != 42 || len(m.Payload) != 4 || binary.BigEndian.Uint32(m.Payload) != 0 {
		t.Fatalf("native area arrival envelope: %+v", m)
	}
	fresh, _ := e.Session(j.ID)
	if fresh.WireID != 0 {
		t.Fatal("population callback allocated fictitious party peer")
	}
}
