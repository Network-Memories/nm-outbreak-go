package kddi

import (
	"encoding/binary"
	"net"
)

// Measured KDDI BioServer lobby & gameserver opcodes.
const (
	CmdGSLogin       uint16 = 0x1031
	CmdSlotStatus    uint16 = 0x6404
	CmdSlotPlrStatus uint16 = 0x6403
	CmdSlotTitle     uint16 = 0x6402
	CmdSlotAttrib2   uint16 = 0x640b
	CmdSlotPwdProt   uint16 = 0x6405
	CmdSlotScenType  uint16 = 0x650a
	CmdPlayerStats   uint16 = 0x640a
	CmdChat          uint16 = 0x6701
)

// BuildSlotStatusPayload encodes a 3-byte KDDI SLOTSTATUS reply.
func BuildSlotStatusPayload(slot uint16, memberCount, maxPlayers int) []byte {
	status := byte(3) // recruiting
	if memberCount >= maxPlayers {
		status = 4 // full
	}
	return []byte{byte(slot >> 8), byte(slot), status}
}

// BuildSlotPlayerStatusPayload encodes a 10-byte KDDI SLOTPLRSTATUS reply.
func BuildSlotPlayerStatusPayload(slot uint16, memberCount, maxPlayers int) []byte {
	return []byte{
		byte(slot >> 8), byte(slot),
		0, byte(memberCount),
		0, 0,
		0, byte(maxPlayers),
		0, byte(memberCount),
	}
}

// BuildSlotTitlePayload encodes a KDDI SLOTTITLE reply (2B slot, 2B len, title bytes).
func BuildSlotTitlePayload(slot uint16, title []byte) []byte {
	out := make([]byte, 4+len(title))
	binary.BigEndian.PutUint16(out[0:2], slot)
	binary.BigEndian.PutUint16(out[2:4], uint16(len(title)))
	copy(out[4:], title)
	return out
}

// RelayGameConnection attaches a post-GSLOGIN KDDI game socket directly to
// world.Registry for zero-hop in-memory fan-out to all other peers (JP, PAL, US).
func (s *Server) RelayGameConnection(conn net.Conn, sessionID string, gs *GameStream) {
	defer conn.Close()
	if s.engine.RoomMixed(sessionID) {
		s.relayMixedGame(conn, sessionID, gs)
		return
	}
	rx, err := s.engine.AttachRelay(sessionID)
	if err != nil {
		return
	}
	defer s.engine.DetachRelay(sessionID)
	go func() {
		defer conn.Close()
		for pkt := range rx {
			if _, err := conn.Write(pkt.Payload); err != nil {
				return
			}
		}
	}()
	buf := make([]byte, 4096)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			return
		}
		records, err := gs.Feed(buf[:n])
		if err != nil {
			return
		}
		s.engine.MarkAlive(sessionID)
		for _, record := range records {
			if err := s.engine.Relay(sessionID, record); err != nil {
				return
			}
		}
	}
}
