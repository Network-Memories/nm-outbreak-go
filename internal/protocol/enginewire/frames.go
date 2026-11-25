// Measured File2 engine envelopes adapted from NM snap_game.py, kddi_game.py
// and ForeignGameBridge.java. Engine bodies are copied without interpretation.
package enginewire

import (
	"encoding/binary"
	"errors"
)

var ErrFrame = errors.New("unmeasured native engine frame")

type Frame struct {
	Peer int
	Body []byte
}

func WesternDecode(payload []byte, count int) ([]Frame, error) {
	if count < 2 || count > 4 {
		return nil, ErrFrame
	}
	var out []Frame
	for at := 0; at < len(payload); {
		if len(payload)-at < 2 {
			return nil, ErrFrame
		}
		word := int(binary.LittleEndian.Uint16(payload[at:]))
		at += 2
		peer, n := word>>12, word&4095
		if peer >= count && peer != 15 || n > len(payload)-at {
			return nil, ErrFrame
		}
		out = append(out, Frame{peer, append([]byte(nil), payload[at:at+n]...)})
		at += n
	}
	return out, nil
}
func WesternEncode(f Frame) ([]byte, error) {
	if f.Peer < 0 || f.Peer > 3 || len(f.Body) > 251 {
		return nil, ErrFrame
	}
	p := make([]byte, 2+len(f.Body))
	binary.LittleEndian.PutUint16(p, uint16(f.Peer<<12|len(f.Body)))
	copy(p[2:], f.Body)
	return p, nil
}

// JapaneseSplit is the measured File #1 split (java-f1-jp-jp-game.pcap and
// go-f1-mixte-0x50.pcap, 23/11/2026): a 244-byte engine body is sent by the original
// JP client as a 0x50 record (word = peer<<12 | full length, first 196 bytes) then a
// 0x40 record of the same peer whose word is 0xf000 | remaining length.
const JapaneseSplit = 196

func JapaneseEncode(f Frame) ([]byte, error) {
	if f.Peer < 0 || f.Peer > 3 || len(f.Body) > 251 {
		return nil, ErrFrame
	}
	if len(f.Body) > JapaneseSplit {
		head := make([]byte, 4+JapaneseSplit)
		head[0], head[1] = byte(len(head)), byte(0x50|f.Peer)
		binary.LittleEndian.PutUint16(head[2:], uint16(f.Peer<<12|len(f.Body)))
		copy(head[4:], f.Body[:JapaneseSplit])
		rest := f.Body[JapaneseSplit:]
		tail := make([]byte, 4+len(rest))
		tail[0], tail[1] = byte(len(tail)), byte(0x40|f.Peer)
		binary.LittleEndian.PutUint16(tail[2:], uint16(0xf000|len(rest)))
		copy(tail[4:], rest)
		return append(head, tail...), nil
	}
	if len(f.Body) == 0 {
		return []byte{2, byte(0xf0 | f.Peer)}, nil
	}
	n, kind := len(f.Body)+4, 0x40
	if n == 40 {
		n++
		kind = 0x70
	}
	p := make([]byte, n)
	p[0] = byte(n)
	p[1] = byte(kind | f.Peer)
	binary.LittleEndian.PutUint16(p[2:], uint16(f.Peer<<12|len(f.Body)))
	copy(p[4:], f.Body)
	return p, nil
}

// JapaneseData validates the native body/index word and padding escape. Control
// frames remain in the measured server-side transport adapter below.
func JapaneseData(p []byte) (Frame, bool, error) {
	if len(p) < 2 || int(p[0]) != len(p) {
		return Frame{}, false, ErrFrame
	}
	peer, kind := int(p[1]&15), p[1]&0xf0
	if kind != 0x40 && kind != 0x70 && kind != 0xf0 {
		return Frame{}, false, nil
	}
	if peer > 3 {
		return Frame{}, true, ErrFrame
	}
	if kind == 0xf0 {
		if len(p) != 2 {
			return Frame{}, true, ErrFrame
		}
		return Frame{Peer: peer}, true, nil
	}
	if len(p) < 4 {
		return Frame{}, true, ErrFrame
	}
	end := len(p)
	if kind == 0x70 {
		if len(p) != 41 {
			return Frame{}, true, ErrFrame
		}
		end--
	}
	word := int(binary.LittleEndian.Uint16(p[2:]))
	if word>>12 != peer || word&4095 != end-4 {
		return Frame{}, true, ErrFrame
	}
	return Frame{peer, append([]byte(nil), p[4:end]...)}, true, nil
}

// Reassembler joins the measured 0x50 + 0x40/0xf000 split into one 0x40 record.
type Reassembler struct {
	peer, want int
	body       []byte
	active     bool
}

// Feed returns the record to process (possibly joined), or nil while a split is pending.
func (r *Reassembler) Feed(p []byte) ([]byte, error) {
	if len(p) >= 4 && int(p[0]) == len(p) && p[1]&0xf0 == 0x50 {
		peer, word := int(p[1]&15), int(binary.LittleEndian.Uint16(p[2:]))
		if r.active || peer > 3 || word>>12 != peer || word&4095 > 251 || len(p)-4 >= word&4095 {
			return nil, ErrFrame
		}
		r.peer, r.want, r.body, r.active = peer, word&4095, append([]byte(nil), p[4:]...), true
		return nil, nil
	}
	if !r.active {
		return p, nil
	}
	if len(p) < 4 || int(p[0]) != len(p) || p[1] != byte(0x40|r.peer) {
		return nil, ErrFrame
	}
	word := int(binary.LittleEndian.Uint16(p[2:]))
	if word>>12 != 15 || word&4095 != len(p)-4 || len(r.body)+len(p)-4 != r.want {
		return nil, ErrFrame
	}
	body := append(r.body, p[4:]...)
	r.active, r.body = false, nil
	out := make([]byte, 4+len(body))
	out[0], out[1] = byte(len(out)), byte(0x40|r.peer)
	binary.LittleEndian.PutUint16(out[2:], uint16(r.peer<<12|len(body)))
	copy(out[4:], body)
	return out, nil
}
