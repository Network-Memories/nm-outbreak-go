// Roster/admission envelopes adapted from BioServer PacketHandler.java,
// Client.java and ClientList.java (AGPL-3.0).
package kddi

import (
	"encoding/binary"
	"errors"
	"net"
	"networkmemories/nm-outbreak-go/internal/core"
	"networkmemories/nm-outbreak-go/internal/world"
)

func nativeMember(actor core.Session) ([]byte, error) {
	if len(actor.Profile) != 208 && len(actor.Profile) != 240 {
		return nil, errors.New("native roster material unavailable")
	}
	p, err := nativeHandlePair(actor.Account.Handle, actor.Account.NativeNickname)
	if err != nil {
		return nil, err
	}
	p = append(p, word(208)...)
	p = append(p, actor.Profile[:208]...)
	return p, nil
}

func nativeRefusal() []byte {
	message := "<LF=6><BODY><CENTER>not possible<END>"
	return append(word(uint16(len(message))), []byte(message)...)
}

func (s *Server) handleAdmission(c net.Conn, f LobbyFrame, id string) bool {
	switch f.Command {
	case 0x6412, 0x6406, 0x640a, 0x6501, 0x6911, 0x6912, 0x6913:
	default:
		return false
	}
	actor, ok := s.engine.Session(id)
	if !ok || actor.Game != s.game || actor.Protocol != core.KDDI || f.Kind != 1 {
		return true
	}
	reply := LobbyFrame{Sender: SenderServer, Kind: KindTell, Command: f.Command, PacketID: f.PacketID}
	fail := func() { reply.Error = 0xff; reply.Payload = nativeRefusal(); sendFrame(c, reply) }
	switch f.Command {
	case 0x6412:
		if len(f.Payload) != 2 {
			fail()
			return true
		}
		slot := binary.BigEndian.Uint16(f.Payload)
		if _, err := s.engine.PreviewKDDISlot(id, slot); err != nil {
			fail()
			return true
		}
		// Opaque four-byte zero metadata measured in the native lobby capture
		// and common to BioServer File1/File2. It is not a score or player count.
		reply.Payload = append(word(slot), 0, 0, 0, 0)
	case 0x6406:
		if len(f.Payload) < 6 {
			fail()
			return true
		}
		slot := binary.BigEndian.Uint16(f.Payload)
		password, n, err := decodeNativeField(f.Payload[2:], f.PacketID)
		if err != nil || n+2 != len(f.Payload) {
			fail()
			return true
		}
		if _, err = s.engine.JoinKDDISlot(id, slot, password); err != nil {
			fail()
			return true
		}
		reply.Payload = word(slot)
	case 0x640a:
		if len(f.Payload) != 2 {
			fail()
			return true
		}
		slot := binary.BigEndian.Uint16(f.Payload)
		room, err := s.engine.PreviewKDDISlot(id, slot)
		if err != nil {
			fail()
			return true
		}
		reply.Payload = append(word(slot), 3, byte(len(room.Members)))
		for _, m := range room.Members {
			peer, ok := s.engine.Session(m.Connection)
			if !ok {
				fail()
				return true
			}
			data, err := nativeMember(peer)
			if err != nil {
				fail()
				return true
			}
			reply.Payload = append(reply.Payload, data...)
		}
	case 0x6501:
		if len(f.Payload) != 0 || actor.InScenario {
			fail()
			return true
		}
		if err := s.engine.LeaveRoom(id); err != nil {
			fail()
			return true
		}
	case 0x6911, 0x6912, 0x6913:
		var room *world.Room
		room, ok = s.engine.Worlds().GetRoom(actor.RoomID)
		if !ok {
			fail()
			return true
		}
		if f.Command == 0x6913 {
			if len(f.Payload) != 1 || f.Payload[0] < 1 || f.Payload[0] > 4 {
				fail()
				return true
			}
			player := f.Payload[0]
			reply.Payload = []byte{player, 0}
			for _, m := range room.Members {
				if m.Seat+1 == int(player) {
					peer, ok := s.engine.Session(m.Connection)
					if !ok {
						fail()
						return true
					}
					data, err := nativeMember(peer)
					if err != nil {
						fail()
						return true
					}
					reply.Payload = append([]byte{player, 1}, data...)
					reply.Payload = append(reply.Payload, 0, 0, 6)
					break
				}
			}
		} else {
			if len(f.Payload) != 0 {
				fail()
				return true
			}
			if f.Command == 0x6911 {
				reply.Payload = []byte{byte(len(room.Members))}
			} else {
				for _, m := range room.Members {
					if m.Connection == id {
						reply.Payload = []byte{byte(m.Seat + 1)}
						break
					}
				}
				if len(reply.Payload) == 0 {
					fail()
					return true
				}
			}
		}
	}
	sendFrame(c, reply)
	return true
}
