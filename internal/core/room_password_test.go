package core

import (
	"bytes"
	"encoding/json"
	"networkmemories/nm-outbreak-go/internal/modules/file2"
	"networkmemories/nm-outbreak-go/internal/world"
	"testing"
)

func TestNativeRoomPasswordPreservedWithoutPublicSecret(t *testing.T) {
	e := New(nil, testAuthority{}, file2.Definition())
	host := actor(t, e, world.GameFile2, KDDI, "ntsc-j", "japanese")
	guest := actor(t, e, world.GameFile2, SNAP, "pal", "western")
	room, err := e.CreateRoom(host.ID, RoomOptions{Area: 1, Scenario: 1, MaxPlayers: 4, Password: []byte("private-room-pass")})
	if err != nil {
		t.Fatal(err)
	}
	if !room.Protected {
		t.Fatal("native protection ignored")
	}
	raw, _ := json.Marshal(room)
	if bytes.Contains(raw, []byte("private-room-pass")) {
		t.Fatal("room secret exposed in state")
	}
	if _, err = e.JoinRoom(guest.ID, room.ID); err == nil {
		t.Fatal("password bypass")
	}
	if _, err = e.JoinRoomWithPassword(guest.ID, room.ID, []byte("wrong")); err == nil {
		t.Fatal("wrong password accepted")
	}
	if _, err = e.JoinRoomWithPassword(guest.ID, room.ID, []byte("private-room-pass")); err != nil {
		t.Fatal(err)
	}
	if _, err = e.JoinRoomWithPassword(guest.ID, room.ID, nil); err != nil {
		t.Fatal("proved member's repeated join failed")
	}
	e.Logout(host.ID)
	if len(e.roomSecrets) != 0 {
		t.Fatal("closed room secret retained")
	}
}
