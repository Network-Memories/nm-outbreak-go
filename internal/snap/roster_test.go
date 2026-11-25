package snap

import (
	"encoding/binary"
	"fmt"
	"net"
	"networkmemories/nm-outbreak-go/internal/core"
	"networkmemories/nm-outbreak-go/internal/modules/file2"
	"networkmemories/nm-outbreak-go/internal/world"
	"testing"
	"time"
)

func TestNativeRosterFourMembersPaginationAndAftergameZeroID(t *testing.T) {
	e := core.New(nil, fixtureAuthority{}, file2.Definition())
	s := NewTransport("file2", world.GameFile2, "127.0.0.1", 0, e)
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
	client.SetReadDeadline(time.Now().Add(time.Second))
	var roomID string
	for i := 0; i < 4; i++ {
		actor, err := e.Login(world.GameFile2, core.SNAP, "pal", fmt.Sprintf("user%d", i), "verified-fixture-password")
		if err != nil {
			t.Fatal(err)
		}
		p := make([]byte, 240)
		p[200] = byte(i)
		if err = e.SetProfile(actor.ID, p); err != nil {
			t.Fatal(err)
		}
		if err = s.adoptSession(uint32(100+i), client.LocalAddr(), actor.ID); err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			room, err := e.CreateRoom(actor.ID, core.RoomOptions{Area: 1, Scenario: 1, MaxPlayers: 4})
			if err != nil {
				t.Fatal(err)
			}
			roomID = room.ID
		} else {
			if _, err = e.JoinRoom(actor.ID, roomID); err != nil {
				t.Fatal(err)
			}
		}
	}
	req := Message{TypeFlags: FlagReliable | FlagRoom, Command: CmdQueryUser, Sequence: 9, SessionID: 100, PacketSeq: 7, Payload: make([]byte, 4)}
	s.queryRoster(server, client.LocalAddr(), req)
	count := 0
	seen := map[uint32]bool{}
	for page := 0; page < 2; page++ {
		buf := make([]byte, 2048)
		n, _, err := client.ReadFrom(buf)
		if err != nil {
			t.Fatal(err)
		}
		if n > 896 {
			t.Fatal("reply exceeds actual client receive buffer")
		}
		m, err := DecodeDatagram(buf[:n])
		if err != nil {
			t.Fatal(err)
		}
		if binary.BigEndian.Uint32(m.Payload) != 0 || binary.BigEndian.Uint32(m.Payload[4:]) != 4 {
			t.Fatal("aftergame zero-ID roster missing")
		}
		rows := int(binary.BigEndian.Uint32(m.Payload[8:]))
		count += rows
		for i := 0; i < rows; i++ {
			sid := binary.BigEndian.Uint32(m.Payload[12+i*264+16:])
			if sid < 100 || sid > 103 || seen[sid] {
				t.Fatal("invented, duplicated or seat-based player ID")
			}
			seen[sid] = true
		}
		if page == 0 && (m.Sequence != 1 || m.TypeFlags&FlagRoom != 0) {
			t.Fatal("continuation page flags")
		}
		if page == 1 && (m.Sequence != 9 || m.TypeFlags&FlagRoom == 0) {
			t.Fatal("final page flags")
		}
	}
	if count != 4 {
		t.Fatal("roster truncated")
	}
}
func TestAreaSearchBinaryLayoutAndTruncation(t *testing.T) {
	p := make([]byte, 8)
	binary.BigEndian.PutUint32(p, 99)
	p[4] = 2
	for _, c := range []struct {
		op   byte
		name string
	}{{4, "obmf01"}, {6, "obmf99"}} {
		p = append(p, []byte("NAME")...)
		p = append(p, 0x40|c.op)
		v := make([]byte, 16)
		copy(v, c.name)
		p = append(p, v...)
	}
	limit, rows, err := parseSearch(p)
	if err != nil || limit != 99 || len(rows) != 2 || rows[1].Operator != 6 {
		t.Fatal("native search layout", err)
	}
	for n := 0; n < len(p); n++ {
		if _, _, err := parseSearch(p[:n]); err == nil {
			t.Fatal("truncated search accepted")
		}
	}
}
