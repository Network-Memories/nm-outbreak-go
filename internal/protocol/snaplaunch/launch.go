// Native File2 host handshake adapted from NM-OUTBREAK snap_launch.py.
// Measured PAL/US 03..08 envelopes; see THIRD_PARTY_NOTICES.md.
package snaplaunch

import (
	"encoding/binary"
	"errors"
)

var ErrSequence = errors.New("unmeasured or out-of-order native launch")
var sizes = map[uint32]int{3: 0, 4: 768, 5: 2, 6: 34, 7: 54, 8: 0}

type Message struct {
	Target uint32
	Tag    uint32
	Body   []byte
}

func Decode(p []byte) (Message, bool, error) {
	if len(p) < 12 || binary.BigEndian.Uint32(p) != 1 {
		return Message{}, false, nil
	}
	tag := binary.LittleEndian.Uint32(p[8:])
	n, known := sizes[tag]
	if !known {
		return Message{}, false, nil
	}
	if len(p) != 12+n {
		return Message{}, true, ErrSequence
	}
	return Message{binary.BigEndian.Uint32(p[4:]), tag, append([]byte(nil), p[12:]...)}, true, nil
}

// Guest receives the original host's table only after the actual guest queries
// its native launch. It never infers readiness from membership or a timer.
type Guest struct {
	Target        uint32
	State         string
	Number, Count int
	Recipient     [16]byte
	Names         [4][16]byte
	Profiles      [4][240]byte
	names         [4]bool
	chunks        [4]int
}

func New(target uint32) Guest { return Guest{Target: target, State: "idle"} }
func (g *Guest) Acknowledge() error {
	if g.State != "announced" {
		return ErrSequence
	}
	g.State = "acknowledged"
	return nil
}
func (g *Guest) Feed(m Message) error {
	if m.Target != g.Target || len(m.Body) != sizes[m.Tag] {
		return ErrSequence
	}
	b := m.Body
	switch m.Tag {
	case 3:
		if g.State != "idle" {
			return ErrSequence
		}
		g.State = "announced"
	case 5:
		if g.State != "acknowledged" || b[0] < 1 || b[0] > 3 || b[1] < 2 || b[1] > 4 || b[0] >= b[1] {
			return ErrSequence
		}
		g.Number, g.Count = int(b[0]), int(b[1])
		g.State = "numbered"
	case 6:
		if g.State != "numbered" && g.State != "names" {
			return ErrSequence
		}
		i := int(b[0])
		if i >= g.Count || b[1] != 0 || g.names[i] {
			return ErrSequence
		}
		var recipient [16]byte
		copy(recipient[:], b[2:18])
		if g.State == "numbered" {
			g.Recipient = recipient
		} else if recipient != g.Recipient {
			return ErrSequence
		}
		copy(g.Names[i][:], b[18:])
		g.names[i] = true
		g.State = "names"
	case 7:
		if g.State != "names" && g.State != "properties" {
			return ErrSequence
		}
		for i := 0; i < g.Count; i++ {
			if !g.names[i] {
				return ErrSequence
			}
		}
		i, n, off := int(b[0]), int(b[1]), int(binary.LittleEndian.Uint16(b[2:]))
		if i >= g.Count || off%50 != 0 || off/50 > 4 || g.chunks[i] != off/50 {
			return ErrSequence
		}
		want := 50
		if off == 200 {
			want = 40
		}
		if n != want {
			return ErrSequence
		}
		for _, v := range b[4+n:] {
			if v != 0 {
				return ErrSequence
			}
		}
		copy(g.Profiles[i][off:], b[4:4+n])
		g.chunks[i]++
		g.State = "properties"
	case 8:
		if g.State != "properties" {
			return ErrSequence
		}
		for i := 0; i < g.Count; i++ {
			if g.chunks[i] != 5 {
				return ErrSequence
			}
		}
		g.State = "complete"
	default:
		return ErrSequence
	}
	return nil
}
