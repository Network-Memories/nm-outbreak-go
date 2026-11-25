package snap

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"net"
	"networkmemories/nm-outbreak-go/internal/core"
	"networkmemories/nm-outbreak-go/internal/identity"
	"networkmemories/nm-outbreak-go/internal/modules/file1"
	"networkmemories/nm-outbreak-go/internal/modules/file2"
	"networkmemories/nm-outbreak-go/internal/protocol/snapauth"
	"networkmemories/nm-outbreak-go/internal/world"
	"path/filepath"
	"testing"
	"time"
)

func serveFixture(t *testing.T, server *Server) net.PacketConn {
	t.Helper()
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pc.Close() })
	go func() {
		buf := make([]byte, 2048)
		for {
			n, remote, err := pc.ReadFrom(buf)
			if err != nil {
				return
			}
			messages, err := DecodeMessages(buf[:n])
			if err != nil {
				continue
			}
			server.handleMessages(pc, remote, messages)
		}
	}()
	return pc
}
func requestFixture(t *testing.T, client net.PacketConn, target net.Addr, message Message) Message {
	t.Helper()
	raw, err := EncodeMessages([]Message{message})
	if err != nil {
		t.Fatal(err)
	}
	client.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := client.WriteTo(raw, target); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 2048)
	for {
		n, _, err := client.ReadFrom(buf)
		if err != nil {
			t.Fatal(err)
		}
		out, err := DecodeDatagram(buf[:n])
		if err != nil {
			t.Fatal(err)
		}
		if out.Command == CmdAck {
			continue
		}
		if out.TypeFlags&FlagReliable != 0 {
			_ = sendMsg(client, target, Message{TypeFlags: FlagResponse, SessionID: out.SessionID, AckNumber: out.PacketSeq})
		}
		return out
	}
}
func answerChallenge(t *testing.T, payload []byte, password string) []byte {
	t.Helper()
	clear, err := snapauth.Decrypt(snapauth.BootstrapKey, payload)
	if err != nil || len(clear) != 280 {
		t.Fatal("native challenge malformed", err)
	}
	size := int(binary.BigEndian.Uint32(clear[4:]))
	if size < 1 || size > 128 {
		t.Fatal("native seed length invalid")
	}
	seed := string(clear[8 : 8+size])
	magic := sha1.Sum([]byte(password + seed))
	secret, err := snapauth.Decrypt(magic[:], clear[148:276])
	if err != nil {
		t.Fatal(err)
	}
	answer := make([]byte, 136)
	copy(answer[8:], secret)
	raw, err := snapauth.Encrypt(snapauth.BootstrapKey, answer)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestBootstrapProofAndGameHandoffUDP(t *testing.T) {
	for _, variant := range []struct {
		name   string
		title  uint32
		game   world.Game
		region string
	}{{"File1US", 0xCAE0, world.GameFile1, "ntsc-u"}, {"File2US", 0xCA19, world.GameFile2, "ntsc-u"}, {"File2PAL", 0xCAF3, world.GameFile2, "pal"}} {
		t.Run(variant.name, func(t *testing.T) {
			store, err := identity.OpenLocal(filepath.Join(t.TempDir(), "accounts.json"))
			if err != nil {
				t.Fatal(err)
			}
			owner, err := store.Register("NativeOne", "fixturepwd")
			if err != nil {
				t.Fatal(err)
			}
			engine := core.New(nil, store, file1.Definition(), file2.Definition())
			bootstrap := NewTransport("bootstrap", "", "127.0.0.1", 0, engine)
			bootSocket := serveFixture(t, bootstrap)
			game := NewTransport("game", variant.game, "127.0.0.1", 0, engine)
			gameSocket := serveFixture(t, game)
			gamePort := uint16(gameSocket.LocalAddr().(*net.UDPAddr).Port)
			if err := engine.ConfigureSNAP(core.SNAPConfig{AdvertiseHost: "127.0.0.1", BootstrapPort: uint16(bootSocket.LocalAddr().(*net.UDPAddr).Port), GamePorts: map[world.Game]uint16{variant.game: gamePort}}); err != nil {
				t.Fatal(err)
			}
			client, err := net.ListenPacket("udp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			login := make([]byte, 104)
			copy(login, "NativeOne")
			binary.BigEndian.PutUint32(login[100:], variant.title)
			initial := Message{TypeFlags: FlagRoom | FlagReliable, Command: 0x2c, SessionID: 0x026407d0, PacketSeq: 1, Payload: login}
			challenge := requestFixture(t, client, bootSocket.LocalAddr(), initial)
			if challenge.Command != 0x40 || challenge.SessionID == 0 || challenge.TypeFlags != FlagChannelBits || challenge.PacketSeq != 1 {
				t.Fatal("native challenge envelope differs")
			}
			duplicate := requestFixture(t, client, bootSocket.LocalAddr(), initial)
			if duplicate.SessionID != challenge.SessionID || !bytes.Equal(duplicate.Payload, challenge.Payload) {
				t.Fatal("retry replaced pending proof")
			}
			answer := answerChallenge(t, challenge.Payload, "fixturepwd")
			if _, ok := game.bindings.Get(challenge.SessionID); ok {
				t.Fatal("challenge alone authenticated a player")
			}
			response := requestFixture(t, client, bootSocket.LocalAddr(), Message{TypeFlags: FlagRoom | FlagReliable, Command: 0x41, SessionID: challenge.SessionID, PacketSeq: 2, Payload: answer})
			if response.Command != 0x2d || response.PacketSeq != 0 || response.TypeFlags != (FlagChannelBits|FlagResponse|FlagReliable) {
				t.Fatal("valid proof did not receive native success")
			}
			destination, err := snapauth.Decrypt(snapauth.BootstrapKey, response.Payload)
			if err != nil || binary.BigEndian.Uint32(destination[44:]) != uint32(gamePort) || readCString(destination[:40]) != "NativeOne" {
				t.Fatal("native destination invalid", err)
			}
			// A different local source port models the measured bootstrap -> game NAT rebind.
			peer, err := net.ListenPacket("udp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer peer.Close()
			kics := make([]byte, 0x128)
			for i := 16; i < 32; i++ {
				kics[i] = '-'
			}
			copy(kics[16:32], "NativeOne")
			admitted := requestFixture(t, peer, gameSocket.LocalAddr(), Message{TypeFlags: FlagRoom | FlagReliable, Command: 0x01, SessionID: challenge.SessionID, PacketSeq: 1, Payload: kics})
			if admitted.Command != 0x29 || admitted.SessionID != challenge.SessionID || admitted.TypeFlags != (FlagChannelBits|FlagReliable) || admitted.PacketSeq != 0 {
				t.Fatal("proved handoff not adopted with native numbering")
			}
			binding, ok := game.bindings.Get(admitted.SessionID)
			if !ok {
				t.Fatal("native binding missing")
			}
			actor, ok := engine.Session(binding.CoreID)
			if !ok || actor.Account.UserID != owner.UserID || actor.Account.Handle != owner.Handle || actor.Region != variant.region {
				t.Fatal("handoff changed owner, HN or measured region")
			}
			profile := make([]byte, 240)
			profile[200] = 4
			set := requestFixture(t, peer, gameSocket.LocalAddr(), Message{TypeFlags: FlagRoom | FlagReliable, Command: 0x0c, SessionID: admitted.SessionID, PacketSeq: 2, Payload: profile})
			if set.Command != 0x28 {
				t.Fatal("authenticated native profile not accepted")
			}
			lobby := make([]byte, 4)
			binary.BigEndian.PutUint32(lobby, uint32(SharedAreaToLobby(variant.game, 1)))
			requestFixture(t, peer, gameSocket.LocalAddr(), Message{TypeFlags: FlagChannelBits | FlagReliable, Command: 0x06, SessionID: admitted.SessionID, PacketSeq: 3, Payload: lobby})
			create := make([]byte, 44)
			binary.BigEndian.PutUint32(create[16:], 2)
			stat := uint32(file1.OpenRoomStatBase)
			if variant.game == world.GameFile2 {
				stat, _ = file2.EncodeStatWord(1, 2, 0)
			}
			binary.BigEndian.PutUint32(create[40:], stat)
			created := requestFixture(t, peer, gameSocket.LocalAddr(), Message{TypeFlags: FlagChannelBits | FlagReliable, Command: 0x04, SessionID: admitted.SessionID, PacketSeq: 4, Payload: create})
			if created.Command != 0x28 || binary.BigEndian.Uint32(created.Payload[4:]) == 0 {
				t.Fatal("authenticated room creation failed")
			}
			rooms := engine.Worlds().ListRooms(variant.game)
			if len(rooms) != 1 || rooms[0].Members[0].Connection != actor.ID || rooms[0].Title != "" {
				t.Fatal("native room not owned by the common engine")
			}
		})
	}
}

func TestBootstrapRejectsWrongProofAndUnknownAccount(t *testing.T) {
	store, err := identity.OpenLocal(filepath.Join(t.TempDir(), "accounts.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Register("NativeOne", "fixturepwd"); err != nil {
		t.Fatal(err)
	}
	engine := core.New(nil, store, file2.Definition())
	server := NewTransport("bootstrap", "", "127.0.0.1", 0, engine)
	pc := serveFixture(t, server)
	engine.ConfigureSNAP(core.SNAPConfig{AdvertiseHost: "127.0.0.1", BootstrapPort: 9090, GamePorts: map[world.Game]uint16{world.GameFile2: 9092}})
	client, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	login := make([]byte, 104)
	copy(login, "NativeOne")
	binary.BigEndian.PutUint32(login[100:], 0xCAF3)
	challenge := requestFixture(t, client, pc.LocalAddr(), Message{Command: 0x2c, SessionID: 12, Payload: login})
	wrong := answerChallenge(t, challenge.Payload, "wrongpass")
	refused := requestFixture(t, client, pc.LocalAddr(), Message{Command: 0x41, SessionID: challenge.SessionID, Payload: wrong})
	if refused.Command != 0x2e {
		t.Fatal("wrong password proof accepted")
	}
	valid := answerChallenge(t, challenge.Payload, "fixturepwd")
	refused = requestFixture(t, client, pc.LocalAddr(), Message{Command: 0x41, SessionID: challenge.SessionID, Payload: valid})
	if refused.Command != 0x2e {
		t.Fatal("invalidated challenge reused")
	}
	copy(login[:40], make([]byte, 40))
	copy(login, "MissingUser")
	refused = requestFixture(t, client, pc.LocalAddr(), Message{Command: 0x2c, SessionID: 12, Payload: login})
	if refused.Command != 0x2e || len(engine.Worlds().ListRooms(world.GameFile2)) != 0 {
		t.Fatal("unknown account created a player")
	}
}
