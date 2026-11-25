package snap

import (
	"encoding/hex"
	"net"
	"networkmemories/nm-outbreak-go/internal/core"
	"networkmemories/nm-outbreak-go/internal/modules/file2"
	"networkmemories/nm-outbreak-go/internal/world"
	"testing"
	"time"
)

func TestAreaChatNeutralSourceAndAuthenticatedTextEnvelope(t *testing.T) {
	e := core.New(nil, fixtureAuthority{}, file2.Definition())
	h, _ := e.Login(world.GameFile2, core.SNAP, "pal", "aaaaa", "verified-fixture-password")
	g, _ := e.Login(world.GameFile2, core.KDDI, "ntsc-j", "ccccc", "verified-fixture-password")
	e.SetArea(h.ID, 1)
	e.SetNativeLobby(h.ID, 1)
	e.AttachKDDILobby(g.ID)
	e.SetKDDIBrowse(g.ID, 1, 0)
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
	if err = s.adoptSession(42, client.LocalAddr(), h.ID); err != nil {
		t.Fatal(err)
	}
	s.watchRoom(server, h.ID)
	if err = e.PublishAreaChat(g.ID, "ｃｃｃｃｃ"); err != nil {
		t.Fatal(err)
	}
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
	if m.SessionID != 0 || m.Command != CmdSend || m.TypeFlags != 0xd400 {
		t.Fatalf("area envelope source/flags: %+v", m)
	}
	text, err := file2.DecodeWesternAreaChat(m.Payload, "ccccc")
	if err != nil || text != "ccccc" {
		t.Fatal("authenticated Japanese area text", err, text)
	}
	fresh, _ := e.Session(g.ID)
	if fresh.WireID != 0 {
		t.Fatal("area chat fabricated a party routing ID")
	}
	gs, _ := e.SubscribeRoom(g.ID)
	body, _ := hex.DecodeString("020005010000616161616131")
	live, _ := e.Session(h.ID)
	s.relayNative(server, client.LocalAddr(), Message{Command: CmdSend, TypeFlags: 0xb400, SessionID: 42, Payload: body}, live)
	event := <-gs.Events()
	if event.ChatArea != 1 || event.ChatText != "1" || event.Actor.Account.UserID != h.Account.UserID {
		t.Fatal("native area request not routed by authenticated scope")
	}
	fake := append([]byte(nil), body...)
	copy(fake[6:11], "other")
	s.relayNative(server, client.LocalAddr(), Message{Command: CmdSend, TypeFlags: 0xb400, SessionID: 42, Payload: fake}, live)
	if len(gs.Events()) != 0 {
		t.Fatal("forged area sender admitted")
	}
}
