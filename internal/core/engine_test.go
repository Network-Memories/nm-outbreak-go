package core

import (
	"errors"
	"networkmemories/nm-outbreak-go/internal/identity"
	"networkmemories/nm-outbreak-go/internal/modules/file1"
	"networkmemories/nm-outbreak-go/internal/modules/file2"
	"networkmemories/nm-outbreak-go/internal/world"
	"testing"
	"time"
)

type testAuthority struct{}

func (testAuthority) Authenticate(name, password string) (*identity.Account, error) {
	if password != "fixture-proof" {
		return nil, identity.ErrAuthentication
	}
	id := int64(1)
	if name == "western" {
		id = 2
	}
	return &identity.Account{UserID: id, Username: name, NativeAlias: name, NativeNickname: name, Handle: "ABC123"}, nil
}
func actor(t *testing.T, e *Engine, game world.Game, protocol Protocol, region, name string) Session {
	t.Helper()
	s, err := e.Login(game, protocol, region, name, "fixture-proof")
	if err != nil {
		t.Fatal(err)
	}
	size := 240
	if protocol == KDDI {
		size = 208
	}
	if err := e.SetProfile(s.ID, make([]byte, size)); err != nil {
		t.Fatal(err)
	}
	return s
}
func TestOneLifecycleAcrossBothNativeDialects(t *testing.T) {
	e := New(nil, testAuthority{}, file1.Definition(), file2.Definition())
	jp := actor(t, e, world.GameFile2, KDDI, "ntsc-j", "japanese")
	west := actor(t, e, world.GameFile2, SNAP, "pal", "western")
	room, err := e.CreateRoom(jp.ID, RoomOptions{Area: 1, Scenario: 1, MaxPlayers: 4, Title: ""})
	if err != nil {
		t.Fatal(err)
	}
	joined, err := e.JoinRoom(west.ID, room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(joined.Members) != 2 || joined.Members[1].Connection != west.ID || joined.Members[1].OwnerID != "2" {
		t.Fatal("native transports do not share authenticated membership")
	}
	updatedProfile := make([]byte, 240)
	updatedProfile[200] = 4
	if err := e.SetProfile(west.ID, updatedProfile); err != nil {
		t.Fatal(err)
	}
	updatedProfile[200] = 9
	observed, _ := e.Worlds().GetRoom(room.ID)
	if observed.Members[1].Profile[200] != 4 {
		t.Fatal("native profile update was not shared or was aliased")
	}
	recv, err := e.AttachRelay(west.ID)
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte{2, 9}
	if err := e.Relay(jp.ID, payload); err != nil {
		t.Fatal(err)
	}
	payload[1] = 8
	if packet := <-recv; packet.FromConn != jp.ID || packet.Payload[1] != 9 {
		t.Fatal("direct core relay changed ownership or frame")
	}
	e.DetachRelay(west.ID)
	current, ok := e.Session(west.ID)
	if !ok || current.RoomID != room.ID {
		t.Fatal("game socket closure removed the logical player")
	}
	snapshot, _ := e.Worlds().GetRoom(room.ID)
	if len(snapshot.Members) != 2 {
		t.Fatal("relay closure removed the place")
	}
	if err := e.LeaveRoom(jp.ID); err != nil {
		t.Fatal(err)
	}
	current, _ = e.Session(west.ID)
	if current.RoomID != "" {
		t.Fatal("host departure left stale guest membership")
	}
	if _, ok := e.Worlds().GetRoom(room.ID); ok {
		t.Fatal("host room remains visible")
	}
	e.Logout(west.ID)
	if _, ok := e.Session(west.ID); ok {
		t.Fatal("logout retained an authenticated session")
	}
}
func TestCoreRejectsUnprovedAndWrongGameActions(t *testing.T) {
	e := New(nil, testAuthority{}, file1.Definition(), file2.Definition())
	if _, err := e.Login(world.GameFile2, SNAP, "pal", "western", "wrong"); !errors.Is(err, identity.ErrAuthentication) {
		t.Fatal("wrong proof accepted")
	}
	if _, err := e.Login(world.GameFile1, SNAP, "pal", "western", "fixture-proof"); err == nil {
		t.Fatal("PAL File1 online invented")
	}
	if _, err := e.CreateRoom("invented", RoomOptions{}); !errors.Is(err, ErrSession) {
		t.Fatal("packet username can create a room")
	}
	f1 := actor(t, e, world.GameFile1, KDDI, "ntsc-j", "japanese")
	f2 := actor(t, e, world.GameFile2, SNAP, "ntsc-u", "western")
	room, err := e.CreateRoom(f1.ID, RoomOptions{Area: 1, Scenario: 1, MaxPlayers: 2})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.JoinRoom(f2.ID, room.ID); err == nil {
		t.Fatal("File2 entered File1")
	}
	if err := e.SetProfile(f2.ID, make([]byte, 208)); err == nil {
		t.Fatal("western profile accepted Japanese shape")
	}
	if _, err := e.AttachRelay("invented"); !errors.Is(err, ErrSession) {
		t.Fatal("unproved relay attached")
	}
	if len(e.Worlds().ListRooms(world.GameFile2)) != 0 {
		t.Fatal("failed action fabricated a room")
	}
}

func TestModuleDescriptionsAreOwnedByCore(t *testing.T) {
	definition := file2.Definition()
	e := New(nil, testAuthority{}, definition)
	definition.Regions["snap"][0] = "unsupported"
	definition.ProfileBytes["snap"] = 1
	definition.Areas[0] = 99
	s := actor(t, e, world.GameFile2, SNAP, "pal", "western")
	if _, err := e.CreateRoom(s.ID, RoomOptions{Area: 1, Scenario: 1, MaxPlayers: 2}); err != nil {
		t.Fatal("external descriptor mutation changed the running module", err)
	}
}

type slowAuthority struct{ started, release chan struct{} }

func (a slowAuthority) Authenticate(name, password string) (*identity.Account, error) {
	if name == "slow" {
		close(a.started)
		<-a.release
	}
	return (testAuthority{}).Authenticate(name, password)
}
func TestSlowAuthenticationDoesNotBlockEstablishedTraffic(t *testing.T) {
	a := slowAuthority{started: make(chan struct{}), release: make(chan struct{})}
	defer close(a.release)
	e := New(nil, a, file2.Definition())
	host := actor(t, e, world.GameFile2, KDDI, "ntsc-j", "japanese")
	guest := actor(t, e, world.GameFile2, SNAP, "pal", "western")
	room, err := e.CreateRoom(host.ID, RoomOptions{Area: 1, Scenario: 1, MaxPlayers: 2})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.JoinRoom(guest.ID, room.ID); err != nil {
		t.Fatal(err)
	}
	rx, err := e.AttachRelay(guest.ID)
	if err != nil {
		t.Fatal(err)
	}
	go e.Login(world.GameFile2, SNAP, "pal", "slow", "fixture-proof")
	<-a.started
	sent := make(chan error, 1)
	go func() { sent <- e.Relay(host.ID, []byte{2, 9}) }()
	select {
	case err := <-sent:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("authentication blocked active game traffic")
	}
	select {
	case <-rx:
	case <-time.After(time.Second):
		t.Fatal("established relay stopped behind authentication")
	}
}
