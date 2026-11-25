// Capcom APP codec ported from openSNAP opensnap_app/capcom/protocol.py (AGPL-3.0).
package capcom

import (
	"encoding/binary"
	"errors"
	"io"
)

var errCodec = errors.New("capcom: malformed native frame or field")

const mask = "MINAMIOH"

type Frame struct {
	Direction             byte
	Command               uint16
	Sequence, Error, Tail byte
	Payload               []byte
}

func ReadFrame(r io.Reader) (Frame, error) {
	var h [8]byte
	if _, err := io.ReadFull(r, h[:]); err != nil {
		return Frame{}, err
	}
	f := Frame{Direction: h[2], Command: binary.BigEndian.Uint16(h[3:5]), Sequence: h[5], Error: h[6], Tail: h[7], Payload: make([]byte, int(binary.BigEndian.Uint16(h[:2])))}
	_, err := io.ReadFull(r, f.Payload)
	return f, err
}
func (f Frame) Encode() ([]byte, error) {
	if len(f.Payload) > 65535 {
		return nil, errCodec
	}
	b := make([]byte, 8+len(f.Payload))
	binary.BigEndian.PutUint16(b, uint16(len(f.Payload)))
	b[2] = f.Direction
	binary.BigEndian.PutUint16(b[3:5], f.Command)
	b[5] = f.Sequence
	b[6] = f.Error
	b[7] = f.Tail
	copy(b[8:], f.Payload)
	return b, nil
}
func EncodeField(plain []byte, seq byte) ([]byte, error) {
	if len(plain) > 65533 {
		return nil, errCodec
	}
	b := make([]byte, 4+len(plain))
	binary.BigEndian.PutUint16(b, uint16(len(plain)+2))
	sum := int(seq) * 0x101
	for i, v := range plain {
		sum += int(v)
		b[i+4] = v ^ mask[i&7] ^ byte(int(seq)+i)
	}
	binary.BigEndian.PutUint16(b[2:], uint16(sum&0x7fff))
	return b, nil
}
func DecodeField(b []byte, seq byte) ([]byte, int, error) {
	if len(b) < 4 {
		return nil, 0, errCodec
	}
	n := int(binary.BigEndian.Uint16(b)) + 2
	if n < 4 || n > len(b) {
		return nil, 0, errCodec
	}
	p := make([]byte, n-4)
	sum := int(seq) * 0x101
	for i, v := range b[4:n] {
		p[i] = v ^ mask[i&7] ^ byte(int(seq)+i)
		sum += int(p[i])
	}
	if uint16(sum&0x7fff) != binary.BigEndian.Uint16(b[2:]) {
		return nil, 0, errCodec
	}
	return p, n, nil
}
