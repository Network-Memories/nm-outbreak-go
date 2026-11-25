package core

import (
	"networkmemories/nm-outbreak-go/internal/modules/file1"
	"networkmemories/nm-outbreak-go/internal/modules/file2"
	"networkmemories/nm-outbreak-go/internal/world"
	"testing"
)

func TestNativeDirectorySlotsRemainStableAcrossCloseAndAreas(t *testing.T) {
	for _, game := range []world.Game{world.GameFile1, world.GameFile2} {
		t.Run(string(game), func(t *testing.T) {
			reg := world.NewRegistry()
			e := New(reg, testAuthority{}, file1.Definition(), file2.Definition())
			jp := actor(t, e, game, KDDI, "ntsc-j", "japanese")
			area := uint16(1)
			if game == world.GameFile1 {
				area = 2
			}
			if err := e.SetKDDIBrowse(jp.ID, area, 1); err != nil {
				t.Fatal(err)
			}
			for _, r := range []world.Room{{ID: "a", Game: game, Area: 1}, {ID: "b", Game: game, Area: 1}, {ID: "other-area", Game: game, Area: 2}} {
				r.HostConn = "fixture"
				r.Members = []world.Member{{Connection: "fixture"}}
				reg.CreateRoom(r)
			}
			slots, err := e.KDDISlots(jp.ID)
			if err != nil || len(slots) != 2 || slots[1].ID != "a" || slots[2].ID != "b" {
				t.Fatal("directory leaked another area", slots, err)
			}
			reg.LeaveRoom("a", "fixture")
			slots, _ = e.KDDISlots(jp.ID)
			if len(slots) != 1 || slots[2].ID != "b" {
				t.Fatal("closing one party renumbered another", slots)
			}
			reg.CreateRoom(world.Room{ID: "aa", Game: game, Area: 1})
			slots, _ = e.KDDISlots(jp.ID)
			if slots[1].ID != "aa" || slots[2].ID != "b" {
				t.Fatal("new party stole occupied coordinates", slots)
			}
			if game == world.GameFile1 {
				if err = e.SetKDDIBrowse(jp.ID, area, 2); err != nil {
					t.Fatal(err)
				}
			} else {
				if err = e.SetKDDIBrowse(jp.ID, 2, 1); err != nil {
					t.Fatal(err)
				}
			}
			slots, _ = e.KDDISlots(jp.ID)
			if len(slots) != 1 || slots[1].ID != "other-area" {
				t.Fatal("native area/room mapping wrong", slots)
			}
			if err = e.SetKDDIBrowse(jp.ID, 0, 0); err != nil {
				t.Fatal(err)
			}
			if _, err = e.KDDISlots(jp.ID); err == nil {
				t.Fatal("slot list allowed outside native directory")
			}
			if err = e.SetKDDIBrowse(jp.ID, 99, 1); err == nil {
				t.Fatal("nonexistent area selected")
			}
			if game == world.GameFile2 {
				if err = e.SetKDDIBrowse(jp.ID, 6, 1); err == nil {
					t.Fatal("disabled area selected")
				}
				if err = e.SetKDDIBrowse(jp.ID, 1, 2); err == nil {
					t.Fatal("disabled directory room selected")
				}
			}
			d := e.KDDIDirectory(game)
			d.Areas[0].Name = "mutated"
			if e.KDDIDirectory(game).Areas[0].Name == "mutated" {
				t.Fatal("directory snapshot mutated module")
			}
		})
	}
}

func TestLobbyDetachExpiresBrowsePresenceButKeepsGamePlace(t *testing.T) {
	e := New(nil, testAuthority{}, file1.Definition(), file2.Definition())
	jp := actor(t, e, world.GameFile2, KDDI, "ntsc-j", "japanese")
	if err := e.AttachKDDILobby(jp.ID); err != nil {
		t.Fatal(err)
	}
	if err := e.AttachKDDILobby(jp.ID); err == nil {
		t.Fatal("two native lobby sockets own one browse context")
	}
	if err := e.SetKDDIBrowse(jp.ID, 1, 1); err != nil {
		t.Fatal(err)
	}
	r, err := e.CreateRoom(jp.ID, RoomOptions{Area: 1, Scenario: 1, MaxPlayers: 2})
	if err != nil {
		t.Fatal(err)
	}
	if err = e.ScenarioPresence(jp.ID, true); err != nil {
		t.Fatal(err)
	}
	e.DetachKDDILobby(jp.ID)
	s, ok := e.Session(jp.ID)
	if !ok || s.NativeLobbyID != 0 || s.NativeRoomID != 0 || s.Area != 0 || s.RoomID != r.ID || !s.InScenario {
		t.Fatal("transport close corrupted logical game place", s)
	}
	room, ok := e.Worlds().GetRoom(r.ID)
	if !ok || len(room.Members) != 1 || room.Members[0].Connection != jp.ID || !room.Playing {
		t.Fatal("native lobby close removed game membership")
	}
	if err = e.AttachKDDILobby(jp.ID); err != nil {
		t.Fatal("closed lobby prevented proved reconnect", err)
	}
}
