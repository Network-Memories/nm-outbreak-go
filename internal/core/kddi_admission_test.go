package core

import (
	"networkmemories/nm-outbreak-go/internal/modules/file1"
	"networkmemories/nm-outbreak-go/internal/modules/file2"
	"networkmemories/nm-outbreak-go/internal/world"
	"testing"
)

func TestJapanesePreviewCannotJoinReplacementParty(t *testing.T) {
	for _, game := range []world.Game{world.GameFile1, world.GameFile2} {
		t.Run(string(game), func(t *testing.T) {
			e := New(nil, testAuthority{}, file1.Definition(), file2.Definition())
			region := "pal"
			area := uint16(1)
			if game == world.GameFile1 {
				region = "ntsc-u"
				area = 2
			}
			host := actor(t, e, game, SNAP, region, "western")
			guest := actor(t, e, game, KDDI, "ntsc-j", "japanese")
			if err := e.SetKDDIBrowse(guest.ID, area, 1); err != nil {
				t.Fatal(err)
			}
			options := RoomOptions{Area: 1, Scenario: 1, MaxPlayers: 2}
			first, err := e.CreateRoom(host.ID, options)
			if err != nil {
				t.Fatal(err)
			}
			preview, err := e.PreviewKDDISlot(guest.ID, 1)
			if err != nil || preview.ID != first.ID {
				t.Fatal("preview", err)
			}
			if err = e.LeaveRoom(host.ID); err != nil {
				t.Fatal(err)
			}
			replacement, err := e.CreateRoom(host.ID, options)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = e.JoinKDDISlot(guest.ID, 1, nil); err == nil {
				t.Fatal("old character choice joined a replacement party")
			}
			unchanged, _ := e.Worlds().GetRoom(replacement.ID)
			s, _ := e.Session(guest.ID)
			if len(unchanged.Members) != 1 || s.RoomID != "" {
				t.Fatal("stale admission mutated membership")
			}
			if _, err = e.PreviewKDDISlot(guest.ID, 1); err != nil {
				t.Fatal(err)
			}
			if _, err = e.JoinKDDISlot(guest.ID, 1, nil); err != nil {
				t.Fatal("fresh preview was rejected", err)
			}
			extra := actor(t, e, game, KDDI, "ntsc-j", "extra")
			if err = e.SetKDDIBrowse(extra.ID, area, 1); err != nil {
				t.Fatal(err)
			}
			if _, err = e.JoinKDDISlot(extra.ID, 1, nil); err == nil {
				t.Fatal("native path bypassed common capacity")
			}
		})
	}
}
