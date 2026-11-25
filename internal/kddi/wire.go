package kddi

import (
	"encoding/binary"
	"errors"
)

const LobbyHeaderSize = 12

// LobbyFrame matches the exact 12-byte KDDI BioServer header (>BBHHHB + 3B reserved + payload).
type LobbyFrame struct {
	raw      []byte
	Sender   uint8
	Kind     uint8
	Command  uint16
	PacketID uint16
	Error    uint8
	Reserved [3]byte
	Payload  []byte
}

func (f LobbyFrame) Encode() ([]byte, error) {
	if len(f.Payload) > 65535 {
		return nil, errors.New("kddi: payload exceeds uint16")
	}
	res := f.Reserved
	if res == [3]byte{} {
		res = [3]byte{0xff, 0xff, 0xff}
	}
	out := make([]byte, LobbyHeaderSize+len(f.Payload))
	out[0] = f.Sender
	out[1] = f.Kind
	binary.BigEndian.PutUint16(out[2:4], f.Command)
	binary.BigEndian.PutUint16(out[4:6], uint16(len(f.Payload)))
	binary.BigEndian.PutUint16(out[6:8], f.PacketID)
	out[8] = f.Error
	copy(out[9:12], res[:])
	copy(out[12:], f.Payload)
	return out, nil
}

// LobbyStream incrementally decodes KDDI LobbyFrames from a TCP byte stream.
type LobbyStream struct {
	buf []byte
}

func (s *LobbyStream) Feed(data []byte) []LobbyFrame {
	s.buf = append(s.buf, data...)
	var frames []LobbyFrame
	for len(s.buf) >= LobbyHeaderSize {
		length := int(binary.BigEndian.Uint16(s.buf[4:6]))
		end := LobbyHeaderSize + length
		if len(s.buf) < end {
			break
		}
		var res [3]byte
		copy(res[:], s.buf[9:12])
		frames = append(frames, LobbyFrame{
			Sender:   s.buf[0],
			Kind:     s.buf[1],
			Command:  binary.BigEndian.Uint16(s.buf[2:4]),
			PacketID: binary.BigEndian.Uint16(s.buf[6:8]),
			Error:    s.buf[8],
			Reserved: res,
			Payload:  append([]byte(nil), s.buf[12:end]...),
		})
		s.buf = s.buf[end:]
	}
	return frames
}

// GameStream decodes post-authentication KDDI GameServer records (1-byte length prefix).
type GameStream struct {
	buf []byte
}

func (s *GameStream) Feed(data []byte) ([][]byte, error) {
	s.buf = append(s.buf, data...)
	var records [][]byte
	for len(s.buf) > 0 {
		ln := int(s.buf[0])
		if ln == 0 {
			return nil, errors.New("kddi: zero-length game record")
		}
		if len(s.buf) < ln {
			break
		}
		records = append(records, append([]byte(nil), s.buf[:ln]...))
		s.buf = s.buf[ln:]
	}
	return records, nil
}
