package core

import (
	"networkmemories/nm-outbreak-go/internal/modules/file1"
	"networkmemories/nm-outbreak-go/internal/modules/file2"
	"networkmemories/nm-outbreak-go/internal/world"
	"testing"
)

func TestSharedHallPopulationTracksNativeNavigationAndTransport(t *testing.T) {
	def := file2.Definition()
	e := New(nil, testAuthority{}, file1.Definition(), def)
	def.NativeSharedHallAreas[0] = 99 // registry must own the slice
	p := actor(t, e, world.GameFile2, SNAP, "pal", "western")
	j := actor(t, e, world.GameFile2, KDDI, "ntsc-j", "japanese")
	ticket := actor(t, e, world.GameFile2, KDDI, "ntsc-j", "ticket")
	e.SetArea(p.ID, 1)
	e.SetNativeLobby(p.ID, 1)
	e.AttachKDDILobby(j.ID)
	e.SetKDDIBrowse(j.ID, 1, 0)
	e.SetKDDIBrowse(ticket.ID, 1, 0) // no actual lobby socket
	check := func(want uint32, other uint16) {
		t.Helper()
		if got := e.NativePopulation(world.GameFile2, SNAP, 1); got != want {
			t.Fatalf("SNAP count=%d want=%d", got, want)
		}
		if got := e.OtherProtocolHallPopulation(world.GameFile2, KDDI, 1); got != other {
			t.Fatalf("JAP supplement=%d want=%d", got, other)
		}
	}
	check(2, 1)
	if lobby, rooms := e.KDDIBrowsePopulation(world.GameFile2, 1); lobby != 2 || rooms != 0 {
		t.Fatalf("native directory includes detached ticket: %d/%d", lobby, rooms)
	}
	e.SetKDDIBrowse(j.ID, 1, 1)
	check(1, 1) // directory is not hall
	e.SetKDDIBrowse(j.ID, 1, 0)
	check(2, 1)
	e.DetachKDDILobby(j.ID)
	check(1, 1)
	e.SetArea(p.ID, 0)
	e.SetNativeLobby(p.ID, 0)
	check(0, 0)
}

func TestSharedHallPopulationKeepsUnmeasuredScopesAndPartiesSeparate(t *testing.T) {
	e := New(nil, testAuthority{}, file1.Definition(), file2.Definition())
	p := actor(t, e, world.GameFile2, SNAP, "pal", "western")
	j := actor(t, e, world.GameFile2, KDDI, "ntsc-j", "japanese")
	e.AttachKDDILobby(j.ID)
	e.SetArea(p.ID, 2)
	e.SetNativeLobby(p.ID, 2)
	e.SetKDDIBrowse(j.ID, 2, 0)
	if got := e.NativePopulation(world.GameFile2, SNAP, 2); got != 1 {
		t.Fatal("unmeasured area merged", got)
	}
	e.SetArea(p.ID, 1)
	e.SetNativeLobby(p.ID, 1)
	e.SetKDDIBrowse(j.ID, 1, 0)
	if _, err := e.CreateRoom(j.ID, RoomOptions{Area: 1, Scenario: 1, MaxPlayers: 2}); err != nil {
		t.Fatal(err)
	}
	if got := e.NativePopulation(world.GameFile2, SNAP, 1); got != 1 {
		t.Fatal("party counted as cross-dialect hall", got)
	}
	if _, err := e.CreateRoom(p.ID, RoomOptions{Area: 1, Scenario: 1, MaxPlayers: 2}); err != nil {
		t.Fatal(err)
	}
	if got := e.OtherProtocolHallPopulation(world.GameFile2, KDDI, 1); got != 0 {
		t.Fatal("western party counted as hall", got)
	}
	f := actor(t, e, world.GameFile1, KDDI, "ntsc-j", "fileone")
	e.AttachKDDILobby(f.ID)
	e.SetKDDIBrowse(f.ID, 1, 0)
	if got := e.OtherProtocolHallPopulation(world.GameFile1, SNAP, 1); got != 0 {
		t.Fatal("File1 implicitly enabled", got)
	}
}
