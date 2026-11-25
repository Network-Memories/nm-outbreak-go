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

func TestCrossHallDepartureBroadcastsAbsoluteJapanesePopulation(t *testing.T) {
	e := nativeEngine(nil)
	ticket, _ := e.LoginKDDI(world.GameFile2, "NativeOne", "nativeproof")
	j, err := e.KDDIHandoff(world.GameFile2, ticket)
	if err != nil {
		t.Fatal(err)
	}
	e.AttachKDDILobby(j.ID)
	e.SetKDDIBrowse(j.ID, 1, 0)
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := NewTransport("fixture", world.GameFile2, "127.0.0.1", 0, e)
	s.watchNativeRoom(ctx, a, j.ID, func() uint16 { return 31 })
	e.SetArea(p.ID, 0)
	e.SetNativeLobby(p.ID, 0)
	f, err := readFrame(b)
	if err != nil {
		t.Fatal(err)
	}
	if f.Command != 0x6205 || f.Kind != 0x10 || f.PacketID != 31 || !bytes.Equal(f.Payload, []byte{0, 1, 0, 1, 0, 0, 255, 255, 0, 0}) {
		t.Fatalf("native population broadcast: %+v", f)
	}
}
