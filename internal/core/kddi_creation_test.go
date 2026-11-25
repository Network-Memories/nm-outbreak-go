package core

import (
	"networkmemories/nm-outbreak-go/internal/modules/file1"
	"networkmemories/nm-outbreak-go/internal/modules/file2"
	"networkmemories/nm-outbreak-go/internal/world"
	"testing"
)

func TestJapaneseCreationReservationCommitAndSecretBothGames(t *testing.T) {
	for _, game := range []world.Game{world.GameFile1, world.GameFile2} {
		t.Run(string(game), func(t *testing.T) {
			e := New(nil, testAuthority{}, file1.Definition(), file2.Definition())
			area := uint16(1)
			region := "pal"
			if game == world.GameFile1 {
				area = 2
				region = "ntsc-u"
			}
			host := actor(t, e, game, KDDI, "ntsc-j", "japanese")
			other := actor(t, e, game, KDDI, "ntsc-j", "other")
			e.SetKDDIBrowse(host.ID, area, 1)
			e.SetKDDIBrowse(other.ID, area, 1)
			if err := e.BeginKDDICreation(host.ID, 1); err != nil {
				t.Fatal(err)
			}
			if err := e.BeginKDDICreation(other.ID, 1); err == nil {
				t.Fatal("concurrent creator stole reserved slot")
			}
			if len(e.Worlds().ListRooms(game)) != 0 {
				t.Fatal("incomplete party published")
			}
			if _, err := e.CommitKDDICreation(host.ID); err == nil {
				t.Fatal("commit without native scene accepted")
			}
			if err := e.SetKDDIBrowse(host.ID, area, 2); err == nil {
				t.Fatal("creator moved reservation between directories")
			}
			west := actor(t, e, game, SNAP, region, "western")
			w, err := e.CreateRoom(west.ID, RoomOptions{Area: 1, Scenario: 1, MaxPlayers: 2})
			if err != nil {
				t.Fatal(err)
			}
			slots, _ := e.KDDISlots(other.ID)
			if slots[2].ID != w.ID {
				t.Fatal("western publication consumed native reservation", slots)
			}
			media := uint16(0x12)
			if game == world.GameFile1 {
				media = 0x01
			}
			if err = e.SetKDDICreationScene(host.ID, media, 1); err != nil {
				t.Fatal(err)
			}
			if err = e.SetKDDICreationTitle(host.ID, "native"); err != nil {
				t.Fatal(err)
			}
			if err = e.SetKDDICreationPassword(host.ID, []byte("roomkey")); err != nil {
				t.Fatal(err)
			}
			for i, v := range []byte{0, 4, 2} {
				if err = e.SetKDDICreationRule(host.ID, byte(i), v); err != nil {
					t.Fatal(err)
				}
			}
			if err = e.SetKDDICreationRule(host.ID, 0, 3); err == nil {
				t.Fatal("invalid native player count accepted")
			}
			room, err := e.CommitKDDICreation(host.ID)
			if err != nil {
				t.Fatal(err)
			}
			if room.MaxPlayers != 2 || room.Difficulty != 2 || room.NativeMedia != media || room.Title != "native" || !room.Protected {
				t.Fatal("native configuration lost", room)
			}
			again, err := e.CommitKDDICreation(host.ID)
			if err != nil || again.ID != room.ID {
				t.Fatal("finalization replay duplicated party")
			}
			slots, _ = e.KDDISlots(other.ID)
			if slots[1].ID != room.ID || slots[2].ID != w.ID {
				t.Fatal("commit renumbered native directory", slots)
			}
			if _, ok := e.KDDICreationForOwner(host.ID); ok {
				t.Fatal("committed reservation retained")
			}
			if _, err = e.JoinKDDISlot(other.ID, 1, []byte("wrong")); err == nil {
				t.Fatal("creation lost password protection")
			}
			if _, err = e.JoinKDDISlot(other.ID, 1, []byte("roomkey")); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestNativeCreationCancelAndDisconnectReleaseReservation(t *testing.T) {
	e := New(nil, testAuthority{}, file2.Definition())
	a := actor(t, e, world.GameFile2, KDDI, "ntsc-j", "japanese")
	b := actor(t, e, world.GameFile2, KDDI, "ntsc-j", "other")
	e.SetKDDIBrowse(a.ID, 1, 1)
	e.SetKDDIBrowse(b.ID, 1, 1)
	e.AttachKDDILobby(a.ID)
	if err := e.BeginKDDICreation(a.ID, 1); err != nil {
		t.Fatal(err)
	}
	e.DetachKDDILobby(a.ID)
	if err := e.BeginKDDICreation(b.ID, 1); err != nil {
		t.Fatal("disconnect retained reservation", err)
	}
	e.LeaveRoom(b.ID)
	e.SetKDDIBrowse(a.ID, 2, 1)
	if err := e.BeginKDDICreation(a.ID, 1); err != nil {
		t.Fatal(err)
	}
	if err := e.SetKDDICreationRule(a.ID, 3, 0); err == nil {
		t.Fatal("native locked nightmare flag overridden")
	}
	schema, values, err := e.KDDIRules(a.ID, 1)
	if err != nil || len(schema) != 4 || values[3] != 1 {
		t.Fatal("area-specific native rules lost", err)
	}
	schema[0].Choices[0] = "mutated"
	values[0] = 99
	againSchema, again, _ := e.KDDIRules(a.ID, 1)
	if again[0] != 2 || againSchema[0].Choices[0] == "mutated" {
		t.Fatal("creation rule snapshot leaked mutation")
	}
}

func TestConcurrentNativeCreatorsReserveOneSlot(t *testing.T) {
	e := New(nil, testAuthority{}, file2.Definition())
	a := actor(t, e, world.GameFile2, KDDI, "ntsc-j", "japanese")
	b := actor(t, e, world.GameFile2, KDDI, "ntsc-j", "western")
	e.SetKDDIBrowse(a.ID, 1, 1)
	e.SetKDDIBrowse(b.ID, 1, 1)
	start := make(chan struct{})
	results := make(chan struct {
		id  string
		err error
	}, 2)
	for _, id := range []string{a.ID, b.ID} {
		go func(id string) {
			<-start
			results <- struct {
				id  string
				err error
			}{id, e.BeginKDDICreation(id, 1)}
		}(id)
	}
	close(start)
	accepted := 0
	winner, other := "", ""
	for i := 0; i < 2; i++ {
		r := <-results
		if r.err == nil {
			accepted++
			winner = r.id
		} else {
			other = r.id
		}
	}
	if accepted != 1 || len(e.Worlds().ListRooms(world.GameFile2)) != 0 {
		t.Fatal("competing reservations published or shared a slot")
	}
	e.Logout(winner)
	if err := e.BeginKDDICreation(other, 1); err != nil {
		t.Fatal("logged-out creator retained reservation", err)
	}
}
