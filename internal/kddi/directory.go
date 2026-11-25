// Native directory envelopes ported from BioServer File1/File2 PacketHandler.java
// (AGPL-3.0); game-specific directory shapes are declared by their modules.
package kddi

import (
	"encoding/binary"
	"net"
	"networkmemories/nm-outbreak-go/internal/core"
)

func word(n uint16) []byte { return []byte{byte(n >> 8), byte(n)} }
func directoryText(n uint16, text string) []byte {
	p := append(word(n), word(uint16(len(text)))...)
	return append(p, []byte(text)...)
}

func (s *Server) handleDirectory(c net.Conn, f LobbyFrame, id string) bool {
	switch f.Command {
	case 0x6145, 0x6881, 0x6882, 0x614c, 0x6203, 0x6204, 0x6205, 0x6206, 0x620a, 0x6207, 0x6209, 0x6301, 0x6302, 0x6303, 0x6304, 0x6305, 0x6308, 0x6401, 0x6408:
	default:
		return false
	}
	a, ok := s.engine.Session(id)
	if !ok || a.Protocol != core.KDDI || a.Game != s.game || f.Kind != 1 {
		return true
	}
	d := s.engine.KDDIDirectory(s.game)
	reply := LobbyFrame{Sender: SenderServer, Kind: KindTell, Command: f.Command, PacketID: f.PacketID}
	fail := func() { reply.Error = 0xff; reply.Payload = nil; sendFrame(c, reply) }
	needsNumber := false
	switch f.Command {
	case 0x6204, 0x6205, 0x6206, 0x620a, 0x6207, 0x6302, 0x6303, 0x6304, 0x6305, 0x6308:
		needsNumber = true
	}
	var n uint16
	if needsNumber {
		if len(f.Payload) != 2 {
			fail()
			return true
		}
		n = binary.BigEndian.Uint16(f.Payload)
	} else if f.Command != 0x6882 && len(f.Payload) != 0 && !(f.Command == 0x6209 && s.engine.KDDIAreaExitAllowed(s.game, f.Payload)) {
		fail()
		return true
	}
	switch f.Command {
	case 0x6145:
		// Native ranking projection is not implemented. Report unavailable;
		// legacy sample scores are not real results and must never be returned.
		fail()
		return true
	case 0x6881:
		// The first byte counts the blocks configured for this game.
		reply.Payload = s.engine.KDDIDownloadManifest(s.game)
		if len(reply.Payload) == 0 {
			fail()
			return true
		}
	case 0x6882:
		reply.Payload = s.engine.KDDIDownloadBlock(s.game, f.Payload)
		if reply.Payload == nil {
			fail()
			return true
		}
	case 0x614c:
		// BioServer MessageOfTheDay.getPacket: no configured message is
		// number zero followed by an empty big-endian string. Never invent
		// an announcement or copy a private deployment's existing content.
		reply.Payload = []byte{0, 0, 0}
	case 0x6203:
		reply.Payload = word(uint16(len(d.Areas)))
	case 0x6204, 0x6205, 0x6206, 0x620a, 0x6207:
		if n == 0 || int(n) > len(d.Areas) {
			fail()
			return true
		}
		area := d.Areas[n-1]
		switch f.Command {
		case 0x6204:
			reply.Payload = directoryText(n, area.Name)
		case 0x620a:
			reply.Payload = directoryText(n, area.Description)
		case 0x6206:
			reply.Payload = append(word(n), area.Status)
		case 0x6207:
			if s.engine.SetKDDIBrowse(id, n, 0) != nil {
				fail()
				return true
			}
			reply.Payload = word(n)
		case 0x6205:
			reply.Payload = append(word(n), 0, 0, 0, 0, 0xff, 0xff, 0, 0)
			lobby, rooms := s.engine.KDDIBrowsePopulation(a.Game, uint32(n))
			binary.BigEndian.PutUint16(reply.Payload[2:], lobby)
			binary.BigEndian.PutUint16(reply.Payload[4:], rooms)
		}
	case 0x6209:
		if s.engine.SetKDDIBrowse(id, 0, 0) != nil {
			fail()
			return true
		}
	case 0x6301, 0x6302, 0x6303, 0x6304, 0x6305, 0x6308, 0x6401, 0x6408:
		if a.NativeLobbyID == 0 {
			fail()
			return true
		}
		switch f.Command {
		case 0x6301:
			reply.Payload = word(uint16(len(d.Rooms)))
		case 0x6401:
			if a.NativeRoomID == 0 {
				fail()
				return true
			}
			reply.Payload = word(d.Slots)
		case 0x6408:
			if s.engine.SetKDDIBrowse(id, uint16(a.NativeLobbyID), 0) != nil {
				fail()
				return true
			}
		default:
			if n == 0 || int(n) > len(d.Rooms) {
				fail()
				return true
			}
			room := d.Rooms[n-1]
			switch f.Command {
			case 0x6302:
				reply.Payload = directoryText(n, room.Name)
			case 0x6304:
				reply.Payload = append(word(n), room.Status)
			case 0x6305:
				if s.engine.SetKDDIBrowse(id, uint16(a.NativeLobbyID), n) != nil {
					fail()
					return true
				}
				reply.Payload = word(n)
			case 0x6303, 0x6308:
				reply.Payload = append(word(n), 0, 0, 0, 0, 0xff, 0xff, 0, 0)
				var count uint16
				for _, peer := range s.engine.Sessions() {
					if peer.Game == a.Game && peer.Protocol == core.KDDI && peer.NativeLobbyID == a.NativeLobbyID && peer.NativeRoomID == n {
						count++
					}
				}
				binary.BigEndian.PutUint16(reply.Payload[2:], count)
			}
		}
	}
	sendFrame(c, reply)
	return true
}
