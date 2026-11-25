package snap

import (
	"context"
	"encoding/binary"
	"errors"
	"net"
	"networkmemories/nm-outbreak-go/internal/core"
	"networkmemories/nm-outbreak-go/internal/identity"
	"networkmemories/nm-outbreak-go/internal/modules/file1"
	"networkmemories/nm-outbreak-go/internal/modules/file2"
	"testing"
	"time"

	"networkmemories/nm-outbreak-go/internal/world"
)

type fixtureAuthority struct{}

func (fixtureAuthority) Authenticate(name, password string) (*identity.Account, error) {
	if password != "verified-fixture-password" {
		return nil, errors.New("fixture authentication rejected")
	}
	id := int64(1)
	handle := "JP1234"
	if name == "aaaaa" {
		id = 2
		handle = "US1234"
	}
	return &identity.Account{UserID: id, Username: name, NativeAlias: name, NativeNickname: name, Handle: handle}, nil
}

func TestSharedCoreJPHostAndAuthenticatedSNAPCodecFixture(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reg := world.NewRegistry()
	// Simulate a Japanese File #2 host creating a room in Area 1 directly in memory (zero HTTP bridge).
	jpProf := make([]byte, 208)
	jpProf[200] = 2 // Jim
	engine := core.New(reg, fixtureAuthority{}, file1.Definition(), file2.Definition())
	host, err := engine.Login(world.GameFile2, core.KDDI, "ntsc-j", "bbbbb", "verified-fixture-password")
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.SetProfile(host.ID, jpProf); err != nil {
		t.Fatal(err)
	}
	rm, err := engine.CreateRoom(host.ID, core.RoomOptions{Area: 1, Scenario: 1, Title: "JP-ROOM", MaxPlayers: 4, RuleValues: []byte{2, 0, 1}, Difficulty: 1})
	if err != nil {
		t.Fatal(err)
	}

	// Bind SNAP File #2 UDP server on an ephemeral port.
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := NewTransport("file2", world.GameFile2, "127.0.0.1", 0, engine)
	go func() {
		buf := make([]byte, 2048)
		for {
			n, remote, err := pc.ReadFrom(buf)
			if err != nil {
				return
			}
			if msg, err := DecodeDatagram(buf[:n]); err == nil {
				srv.handleMessage(pc, remote, msg)
			}
		}
	}()
	defer pc.Close()
	_ = ctx

	client, err := net.Dial("udp", pc.LocalAddr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()
	_ = client.SetDeadline(time.Now().Add(2 * time.Second))

	sendRecv := func(m Message) Message {
		if _, err := client.Write(EncodeDatagram(m)); err != nil {
			t.Fatalf("write: %v", err)
		}
		buf := make([]byte, 2048)
		for {
			n, err := client.Read(buf)
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			out, err := DecodeDatagram(buf[:n])
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			if out.Command == CmdAck {
				continue
			}
			if out.TypeFlags&FlagReliable != 0 {
				_, _ = client.Write(EncodeDatagram(Message{TypeFlags: FlagResponse, SessionID: out.SessionID, AckNumber: out.PacketSeq}))
			}
			return out
		}
	}

	// The missing native bootstrap is an explicit authenticated fixture, not a packet username.
	guest, err := engine.Login(world.GameFile2, core.SNAP, "pal", "aaaaa", "verified-fixture-password")
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.adoptSession(0x11223344, client.LocalAddr(), guest.ID); err != nil {
		t.Fatal(err)
	}
	// 1. Login
	loginPayload := make([]byte, 0x128)
	copy(loginPayload, "aaaaa")
	resp := sendRecv(Message{TypeFlags: FlagRoom | FlagReliable, Command: CmdLoginToKics, SessionID: 0x11223344, PacketSeq: 1, Payload: loginPayload})
	if resp.Command != 0x29 || len(resp.Payload) != 12 || binary.BigEndian.Uint32(resp.Payload[8:12]) != 0x11223344 {
		t.Fatalf("login failed: %+v", resp)
	}

	profileReply := sendRecv(Message{TypeFlags: FlagRoom | FlagReliable, Command: CmdChangeUserProperty, SessionID: 0x11223344, PacketSeq: 2, Payload: make([]byte, 240)})
	if profileReply.Command != 0x28 {
		t.Fatal("native profile not accepted")
	}
	// 2. Query Game Rooms in Area 1 -> must see JP-ROOM!
	qArea := make([]byte, 4)
	binary.BigEndian.PutUint32(qArea, 1)
	listResp := sendRecv(Message{TypeFlags: FlagChannelBits, Command: CmdQueryGameRooms, SessionID: 0x11223344, PacketSeq: 2, Payload: qArea})
	if binary.BigEndian.Uint32(listResp.Payload[8:12]) != 1 {
		t.Fatalf("expected 1 room in Area 1, got %d", binary.BigEndian.Uint32(listResp.Payload[8:12]))
	}
	roomNumID := binary.BigEndian.Uint32(listResp.Payload[12+32 : 12+36])

	// 3. Join JP-ROOM
	joinPayload := make([]byte, 4)
	binary.BigEndian.PutUint32(joinPayload, roomNumID)
	joinResp := sendRecv(Message{TypeFlags: FlagRoom | FlagReliable, Command: CmdJoin, SessionID: 0x11223344, PacketSeq: 3, Payload: joinPayload})
	if binary.BigEndian.Uint32(joinResp.Payload[4:8]) != ResultOK {
		t.Fatalf("join failed: %+v", joinResp)
	}

	updated, _ := reg.GetRoom(rm.ID)
	if len(updated.Members) != 2 {
		t.Fatalf("expected 2 members (JP host + PAL guest), got %d", len(updated.Members))
	}
}

func TestUnprovedKICSClaimCannotCreateSession(t *testing.T) {
	engine := core.New(nil, fixtureAuthority{}, file1.Definition(), file2.Definition())
	srv := NewTransport("file2", world.GameFile2, "127.0.0.1", 0, engine)
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer pc.Close()
	remote := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 32000}
	payload := make([]byte, 0x128)
	copy(payload, "claimed_name")
	srv.handleMessage(pc, remote, Message{Command: 0x01, SessionID: 1234, Payload: payload})
	if _, ok := srv.bindings.Get(1234); ok {
		t.Fatal("username-only KICS established identity")
	}
	actor, err := engine.Login(world.GameFile2, core.SNAP, "pal", "aaaaa", "verified-fixture-password")
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.adoptSession(1234, remote, actor.ID); err != nil {
		t.Fatal(err)
	}
	engine.Logout(actor.ID)
	srv.handleMessage(pc, remote, Message{Command: 0x04, SessionID: 1234, Payload: make([]byte, 44)})
	if len(engine.Worlds().ListRooms(world.GameFile2)) != 0 {
		t.Fatal("logged-out session mutated the shared world")
	}
}
