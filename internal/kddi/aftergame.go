// Native aftergame envelopes follow BioServer PacketHandler.java (AGPL-3.0).
package kddi

import (
	"net"
	"networkmemories/nm-outbreak-go/internal/core"
)

func (s *Server) handleAftergame(c net.Conn, f LobbyFrame, id string) bool {
	switch f.Command {
	case 0x6210, 0x6211, 0x6212, 0x6213:
	default:
		return false
	}
	reply := LobbyFrame{Sender: SenderServer, Kind: KindTell, Command: f.Command, PacketID: f.PacketID}
	fail := func() { reply.Error = 0xff; sendFrame(c, reply) }
	actor, ok := s.engine.Session(id)
	if !ok || actor.Protocol != core.KDDI || actor.Game != s.game || f.Kind != 1 || len(f.Payload) != 0 {
		fail()
		return true
	}
	if f.Command == 0x6210 {
		if !s.engine.HasScenarioRound(id) || s.engine.ScenarioPresence(id, false) != nil {
			fail()
			return true
		}
	}
	members, err := s.engine.AftergameMembers(id)
	if err != nil {
		fail()
		return true
	}
	switch f.Command {
	case 0x6211:
		if s.engine.LeaveRoom(id) != nil {
			fail()
			return true
		}
	case 0x6212:
		reply.Payload = word(uint16(len(members)))
	case 0x6213:
		reply.Payload = []byte{0, 0, 3, byte(len(members))}
		for _, member := range members {
			body, err := nativeMember(member)
			if err != nil {
				reply.Payload = nil
				fail()
				return true
			}
			reply.Payload = append(reply.Payload, body...)
		}
	}
	sendFrame(c, reply)
	return true
}
