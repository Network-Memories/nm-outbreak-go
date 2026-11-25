package snap

import (
	"encoding/binary"
)

const (
	FlagResponse    uint16 = 0x4000
	FlagReliable    uint16 = 0x8000
	FlagRoom        uint16 = 0x2000
	FlagChannelBits uint16 = 0x3000

	ResultOK      uint32 = 0
	ErrorRefused  uint32 = 1
	ProfileLength int    = 240
)

func packFixed(s string, size int) []byte {
	out := make([]byte, size)
	copy(out, s)
	return out
}

// BuildResult encodes a standard SNAP 8-byte result payload (>2L: command, value).
func BuildResult(req Message, sessionID uint32, value uint32, channelFlags uint16) Message {
	payload := make([]byte, 8)
	binary.BigEndian.PutUint32(payload[0:4], uint32(req.Command))
	binary.BigEndian.PutUint32(payload[4:8], value)
	flags := channelFlags | FlagResponse
	if req.TypeFlags&FlagReliable != 0 {
		flags |= FlagReliable
	}
	return Message{
		TypeFlags: flags,
		Sequence:  req.Sequence,
		Command:   CmdResultWrapper,
		SessionID: sessionID,
		PacketSeq: req.PacketSeq,
		AckNumber: req.PacketSeq,
		Payload:   payload,
	}
}

// BuildRoomRow encodes a 36-byte SNAP room list row (>16s5L).
func BuildRoomRow(name string, memberCount uint32, rules uint32, maxPlayers uint32, roomID uint32) []byte {
	out := make([]byte, 36)
	copy(out[0:16], packFixed(name, 16))
	binary.BigEndian.PutUint32(out[16:20], memberCount)
	binary.BigEndian.PutUint32(out[20:24], 0)
	binary.BigEndian.PutUint32(out[24:28], rules)
	binary.BigEndian.PutUint32(out[28:32], maxPlayers)
	binary.BigEndian.PutUint32(out[32:36], roomID)
	return out
}

// BuildMemberRecord encodes a SNAP room member record (16s + >2L + 240B profile).
func BuildMemberRecord(username string, sessionID uint32, profile []byte) []byte {
	p := make([]byte, ProfileLength)
	copy(p, profile)
	out := make([]byte, 16+8+ProfileLength)
	copy(out[0:16], packFixed(username, 16))
	binary.BigEndian.PutUint32(out[16:20], sessionID)
	binary.BigEndian.PutUint32(out[20:24], uint32(len(p)))
	copy(out[24:], p)
	return out
}
