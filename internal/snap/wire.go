// SNAP framing ported from openSNAP protocol/codec.py (AGPL-3.0).
// See THIRD_PARTY_NOTICES.md and licenses/openSNAP-AGPL-3.0.txt.
package snap

import (
	"encoding/binary"
	"errors"
)

const (
	HeaderSize          = 16
	FooterSize          = 4
	FooterMarker uint32 = 0xba476611
	TypeMask     uint16 = 0xfc00
	LengthMask   uint16 = 0x03ff
	FlagMulti    uint16 = 0x0800
)

// Message is one native SNAP message. A datagram may contain several messages
// and ends in a fixed marker, not a CRC. PacketSeq is the transport sequence;
// Sequence is the one-byte command packet number.
type Message struct {
	EngineHandled    bool   // internal: engine batch committed before transport consumption
	RouteSessionID   uint32 // Internal recipient; wire header may identify a relayed sender.
	TypeFlags        uint16
	Sequence         uint8
	Command          uint8
	SessionID        uint32
	PacketSeq        uint32
	AckNumber        uint32
	Payload          []byte
	EmbeddedInMulti  bool
	SourceSessionID  uint32
	HasSourceSession bool
}

func DecodeMessages(raw []byte) ([]Message, error) {
	if len(raw) < HeaderSize+FooterSize {
		return nil, errors.New("snap: datagram too short")
	}
	end := len(raw) - FooterSize
	if binary.BigEndian.Uint32(raw[end:]) != FooterMarker {
		return nil, errors.New("snap: invalid footer marker")
	}
	var messages []Message
	embedded := false
	for offset := 0; offset < end; {
		if end-offset < HeaderSize {
			return nil, errors.New("snap: truncated message header")
		}
		word := binary.BigEndian.Uint16(raw[offset:])
		size := int(word & LengthMask)
		if size < HeaderSize || size > end-offset {
			return nil, errors.New("snap: invalid encoded message length")
		}
		m := Message{TypeFlags: word & TypeMask, Sequence: raw[offset+2], Command: raw[offset+3], SessionID: binary.BigEndian.Uint32(raw[offset+4:]), PacketSeq: binary.BigEndian.Uint32(raw[offset+8:]), AckNumber: binary.BigEndian.Uint32(raw[offset+12:]), Payload: append([]byte(nil), raw[offset+HeaderSize:offset+size]...), EmbeddedInMulti: embedded}
		if embedded {
			m.PacketSeq = 0
		}
		messages = append(messages, m)
		offset += size
		if m.TypeFlags&FlagMulti != 0 {
			embedded = true
		}
	}
	return messages, nil
}

// DecodeDatagram is a strict single-message convenience API; it never discards children.
func DecodeDatagram(raw []byte) (Message, error) {
	messages, err := DecodeMessages(raw)
	if err != nil {
		return Message{}, err
	}
	if len(messages) != 1 {
		return Message{}, errors.New("snap: multiple messages require DecodeMessages")
	}
	return messages[0], nil
}

func EncodeMessages(messages []Message) ([]byte, error) {
	if len(messages) == 0 {
		return nil, errors.New("snap: empty message batch")
	}
	var out []byte
	for _, m := range messages {
		size := HeaderSize + len(m.Payload)
		if size > int(LengthMask) || m.TypeFlags&LengthMask != 0 {
			return nil, errors.New("snap: flags or payload exceed native message length")
		}
		start := len(out)
		out = append(out, make([]byte, size)...)
		binary.BigEndian.PutUint16(out[start:], m.TypeFlags|uint16(size))
		out[start+2] = m.Sequence
		out[start+3] = m.Command
		binary.BigEndian.PutUint32(out[start+4:], m.SessionID)
		seq := m.PacketSeq
		if m.EmbeddedInMulti {
			seq = 0
		}
		binary.BigEndian.PutUint32(out[start+8:], seq)
		binary.BigEndian.PutUint32(out[start+12:], m.AckNumber)
		copy(out[start+HeaderSize:], m.Payload)
	}
	var footer [4]byte
	binary.BigEndian.PutUint32(footer[:], FooterMarker)
	out = append(out, footer[:]...)
	return out, nil
}

// EncodeDatagram is retained for bounded single-message fixtures. Runtime writers
// use EncodeMessages so an oversized reply can be reported instead of truncated.
func EncodeDatagram(m Message) []byte {
	raw, err := EncodeMessages([]Message{m})
	if err != nil {
		return nil
	}
	return raw
}
