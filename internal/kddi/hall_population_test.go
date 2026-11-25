package kddi

import (
	"bytes"
	"net"
	"networkmemories/nm-outbreak-go/internal/core"
	"networkmemories/nm-outbreak-go/internal/world"
	"testing"
	"time"
)

func TestAreaPopulationWireIncludesOnlyAttachedSharedHall(t *testing.T) {
	e := nativeEngine(nil)
	ticket, err := e.LoginKDDI(world.GameFile2, "NativeOne", "nativeproof")
	if err != nil {
		t.Fatal(err)
	}
	j, err := e.KDDIHandoff(world.GameFile2, ticket)
	if err != nil {
		t.Fatal(err)
	}
	if err = e.AttachKDDILobby(j.ID); err != nil {
		t.Fatal(err)
	}
	if err = e.SetKDDIBrowse(j.ID, 1, 0); err != nil {
		t.Fatal(err)
	}
	p, err := e.Login(world.GameFile2, core.SNAP, "pal", "WesternHost", "nativeproof")
	if err != nil {
		t.Fatal(err)
	}
	e.SetArea(p.ID, 1)
	e.SetNativeLobby(p.ID, 1)
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	b.SetDeadline(time.Now().Add(time.Second))
	s := NewTransport("fixture", world.GameFile2, "127.0.0.1", 0, e)
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.handleDirectory(a, LobbyFrame{Kind: 1, Command: 0x6205, PacketID: 7, Payload: []byte{0, 1}}, j.ID)
	}()
	f, err := readFrame(b)
	if err != nil {
		t.Fatal(err)
	}
	<-done
	want := []byte{0, 1, 0, 2, 0, 0, 255, 255, 0, 0}
	if f.Kind != KindTell || f.Command != 0x6205 || f.PacketID != 7 || f.Error != 0 || !bytes.Equal(f.Payload, want) {
		t.Fatalf("native area count frame=%+v", f)
	}
}
