package kddi

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"networkmemories/nm-outbreak-go/internal/core"
	"networkmemories/nm-outbreak-go/internal/identity"
	"networkmemories/nm-outbreak-go/internal/modules/file1"
	"networkmemories/nm-outbreak-go/internal/modules/file2"
	"networkmemories/nm-outbreak-go/internal/world"
	"testing"
	"time"
)

type nativeAuthority struct{}

func (nativeAuthority) Authenticate(name, password string) (*identity.Account, error) {
	if password != "nativeproof" {
		return nil, identity.ErrAuthentication
	}
	id, handle := int64(1), "ABC123"
	if name == "WesternHost" {
		id = 2
		handle = "HST001"
	}
	account := &identity.Account{UserID: id, Username: name, NativeAlias: name, NativeNickname: name, Handle: handle}
	if name == "MultiHandle" {
		account.HandlePairs = [3]identity.HandlePair{{Handle: handle, Nickname: name}, {Handle: "DEF456", Nickname: "あ"}}
	}
	return account, nil
}

func TestNativeHandleListAndSelectionKeepAccountOwner(t *testing.T) {
	e := nativeEngine(nil)
	ticket, err := e.LoginKDDI(world.GameFile2, "MultiHandle", "nativeproof")
	if err != nil {
		t.Fatal(err)
	}
	actor, err := e.KDDIHandoff(world.GameFile2, ticket)
	if err != nil {
		t.Fatal(err)
	}
	list, err := hnList(actor)
	second := []byte{0, 6, 'D', 'E', 'F', '4', '5', '6', 0, 2, 0x82, 0xa0, 0, 0}
	if err != nil || len(list) == 0 || list[0] != 2 || !bytes.HasSuffix(list, second) {
		t.Fatalf("bad two-handle wire list: %x (%v)", list, err)
	}
	if _, err := e.SelectKDDIHandle(actor.ID, "DEF456", "wrong"); err == nil {
		t.Fatal("accepted an unproved handle and nickname pairing")
	}
	selected, err := e.SelectKDDIHandle(actor.ID, "DEF456", "あ")
	if err != nil || selected.Account.UserID != actor.Account.UserID || selected.Account.Handle != "DEF456" || selected.Account.NativeNickname != "あ" {
		t.Fatalf("selection changed the owner or lost the pair: %v", err)
	}
	selected.Profile = make([]byte, 208)
	card, err := nativeMember(selected)
	if err != nil || !bytes.HasPrefix(card, second[:12]) {
		t.Fatalf("selected Japanese nickname did not reach the roster: %v", err)
	}
	reconnected, err := e.KDDIHandoff(world.GameFile2, ticket)
	if err != nil || reconnected.Account.Handle != "DEF456" {
		t.Fatalf("selection did not survive reconnection: %v", err)
	}
}
func nativeEngine(reg *world.Registry) *core.Engine {
	return core.New(reg, nativeAuthority{}, file1.Definition(), file2.Definition())
}
func ticketPayload(ticket string, pid uint16, lobby bool) []byte {
	var a, b int
	fmt.Sscanf(ticket, "%4d%4d", &a, &b)
	p := []byte(fmt.Sprintf("%05d%05d", a+int(pid), b+int(pid)))
	if lobby {
		return append([]byte{0, 10}, p...)
	}
	return p
}
func nativeField(plain []byte, pid uint16) []byte {
	p := make([]byte, 4+len(plain))
	binary.BigEndian.PutUint16(p, uint16(len(plain)+2))
	copy(p[4:], plain)
	out, _, _ := decodeNativeField(p, pid)
	copy(p[4:], out)
	return p
}
func completeHandshake(t *testing.T, c net.Conn, ticket string) {
	t.Helper()
	completeHandshakeWithHandle(t, c, ticket, "ABC123", nil)
}

func completeHandshakeWithHandle(t *testing.T, c net.Conn, ticket, handle string, nickname []byte) {
	t.Helper()
	f, err := readFrame(c)
	if err != nil || f.Command != 0x6101 || f.Sender != 0x18 || f.Kind != 1 {
		t.Fatal("LOGIN query missing", err)
	}
	if err = writeFrame(c, LobbyFrame{Sender: 0x81, Kind: 2, Command: f.Command, PacketID: f.PacketID, Payload: ticketPayload(ticket, f.PacketID, true)}); err != nil {
		t.Fatal(err)
	}
	version, err := readFrame(c)
	if err != nil || version.Command != 0x6103 {
		t.Fatal("version query missing", err)
	}
	if err = writeFrame(c, LobbyFrame{Sender: 0x81, Kind: 2, Command: version.Command, PacketID: version.PacketID}); err != nil {
		t.Fatal(err)
	}
	hn, err := readFrame(c)
	if err != nil || hn.Command != 0x6131 || len(hn.Payload) < 14 || string(hn.Payload[3:9]) != "ABC123" {
		t.Fatal("native handles missing", err)
	}
	if nickname == nil {
		size := int(hn.Payload[10])
		if 11+size > len(hn.Payload) {
			t.Fatal("truncated native nickname")
		}
		nickname = hn.Payload[11 : 11+size]
	}
	p := append(nativeField([]byte(handle), 7), nativeField(nickname, 7)...)
	if err = writeFrame(c, LobbyFrame{Sender: 0x81, Kind: 1, Command: 0x6132, PacketID: 7, Payload: p}); err != nil {
		t.Fatal(err)
	}
	selected, err := readFrame(c)
	if err != nil || selected.Command != 0x6132 || len(selected.Payload) < 2 || string(selected.Payload[2:]) != handle {
		t.Fatal("HN select reply", err)
	}
	done, err := readFrame(c)
	if err != nil || done.Command != 0x6104 {
		t.Fatal("native login did not finish", err)
	}
}

func TestNativeHandshakeSelectsSecondJapaneseHandle(t *testing.T) {
	e := nativeEngine(nil)
	ticket, err := e.LoginKDDI(world.GameFile2, "MultiHandle", "nativeproof")
	if err != nil {
		t.Fatal(err)
	}
	server, client := net.Pipe()
	defer client.Close()
	client.SetDeadline(time.Now().Add(2 * time.Second))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go NewTransport("fixture-lobby", world.GameFile2, "127.0.0.1", 0, e).serveConn(ctx, server)
	completeHandshakeWithHandle(t, client, ticket, "DEF456", []byte{0x82, 0xa0})
	actor, err := e.KDDIHandoff(world.GameFile2, ticket)
	if err != nil || actor.Account.UserID != 1 || actor.Account.Handle != "DEF456" || actor.Account.NativeNickname != "あ" {
		t.Fatalf("selected handle was not retained for the authenticated owner: %v", err)
	}
}
func TestNativeKDDIHandshakeBothGamesAndProfile(t *testing.T) {
	for _, game := range []world.Game{world.GameFile1, world.GameFile2} {
		t.Run(string(game), func(t *testing.T) {
			e := nativeEngine(nil)
			ticket, err := e.LoginKDDI(game, "NativeOne", "nativeproof")
			if err != nil {
				t.Fatal(err)
			}
			a, b := net.Pipe()
			defer b.Close()
			b.SetDeadline(time.Now().Add(2 * time.Second))
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			s := NewTransport("fixture-lobby", game, "127.0.0.1", 0, e)
			go s.serveConn(ctx, a)
			completeHandshake(t, b, ticket)
			actor, err := e.KDDIHandoff(game, ticket)
			if err != nil {
				t.Fatal(err)
			}
			p := make([]byte, 208)
			p[200] = 4
			if err = writeFrame(b, LobbyFrame{Sender: 0x81, Kind: 1, Command: 0x6190, PacketID: 9, Payload: nativeField(p, 9)}); err != nil {
				t.Fatal(err)
			}
			f, err := readFrame(b)
			if err != nil || f.Command != 0x6190 {
				t.Fatal("profile reply", err)
			}
			actor, _ = e.Session(actor.ID)
			if len(actor.Profile) != 208 || actor.Profile[200] != 4 {
				t.Fatal("profile bypassed common core")
			}
		})
	}
}
func TestNativeKDDIRejectsUnprovedOrOtherGameTickets(t *testing.T) {
	e := nativeEngine(nil)
	ticket, _ := e.LoginKDDI(world.GameFile1, "NativeOne", "nativeproof")
	for _, p := range []string{ticket, "00000000"} {
		a, b := net.Pipe()
		b.SetDeadline(time.Now().Add(time.Second))
		s := NewTransport("file2-lobby", world.GameFile2, "127.0.0.1", 0, e)
		go s.serveConn(context.Background(), a)
		f, _ := readFrame(b)
		writeFrame(b, LobbyFrame{Sender: 0x81, Kind: 2, Command: 0x6101, PacketID: f.PacketID, Payload: ticketPayload(p, f.PacketID, true)})
		if _, err := readFrame(b); err == nil {
			t.Fatal("unproved/wrong-game ticket accepted")
		}
		b.Close()
	}
}
func TestNativeKDDITicketParsingMalformed(t *testing.T) {
	for _, p := range [][]byte{nil, []byte("123"), []byte("0000X00000"), []byte("0000000000")} {
		if _, err := ticketAnswer(p, 100, false); err == nil {
			t.Fatal("malformed ticket accepted")
		}
	}
}

func TestMeasuredLobbyLoginSuffixKeepsTicketProofStrict(t *testing.T) {
	const ticket = "12345678"
	canonical := ticketPayload(ticket, 19, true)
	actual := append(append([]byte(nil), canonical...), 0, 2, 0, 0)
	suffix := nativeEngine(nil).KDDILobbyLoginSuffix(world.GameFile2)
	got, err := ticketAnswer(actual, 19, true, suffix)
	if err != nil || got != ticket {
		t.Fatal("original sixteen-byte login rejected", err)
	}
	if _, err = ticketAnswer(actual, 19, true, nil); err == nil {
		t.Fatal("lobby suffix accepted without a module declaration")
	}
	altered := append([]byte(nil), actual...)
	altered[len(altered)-1] ^= 1
	if _, err = ticketAnswer(altered, 19, true, suffix); err == nil {
		t.Fatal("unknown suffix silently truncated")
	}
	altered = append([]byte(nil), actual...)
	altered[3] = 'x'
	if _, err = ticketAnswer(altered, 19, true, suffix); err == nil {
		t.Fatal("suffix bypassed decimal ticket proof")
	}
	if _, err = ticketAnswer(actual, 19, false, suffix); err == nil {
		t.Fatal("lobby suffix generalized to GSLOGIN")
	}
}

func TestOutbreakOriginalVersionReplyUsesOwnCounterOnlyForMeasuredShape(t *testing.T) {
	e := nativeEngine(nil)
	for _, game := range []world.Game{world.GameFile1, world.GameFile2} {
		if !e.KDDIVersionReplyAllowed(game, 30000, 6, make([]byte, 33)) {
			t.Fatalf("%s: original own-counter reply refused", game)
		}
		if e.KDDIVersionReplyAllowed(game, 30000, 6, make([]byte, 32)) {
			t.Fatalf("%s: unmeasured reply shape accepted", game)
		}
		if e.KDDIVersionReplyAllowed(game, 30000, 6, nil) {
			t.Fatalf("%s: canonical reply lost correlation", game)
		}
		if got := e.KDDILobbyLoginSuffix(game); !bytes.Equal(got, []byte{0, 2, 0, 0}) {
			t.Fatalf("%s: original login suffix differs: %x", game, got)
		}
	}
}
