package enginewire

import (
	"bytes"
	"encoding/binary"
	"math"
)

// KDDIPeer adapts the Japanese transport for one actual Western participant.
// Construct only after both proved members entered the shared native round.
// These are server transport probes, never invented game/player statistics.
type KDDIPeer struct {
	Local, Remote         int
	probes                [8][7]byte
	echoed                [8]bool
	started, probed, done bool
}

func NewKDDIPeer(local, remote int) (*KDDIPeer, error) {
	if local < 0 || local > 3 || remote < 0 || remote > 3 || local == remote {
		return nil, ErrFrame
	}
	return &KDDIPeer{Local: local, Remote: remote}, nil
}
func (s *KDDIPeer) Start() [][]byte {
	if s.started {
		return nil
	}
	s.started = true
	return [][]byte{{3, byte(0x90 | s.Remote), 0}}
}
func (s *KDDIPeer) Receive(p []byte, seconds float32) ([][]byte, *Frame, error) {
	if !s.started || len(p) < 2 || int(p[0]) != len(p) || int(p[1]&15) != s.Local {
		return nil, nil, ErrFrame
	}
	f, data, err := JapaneseData(p)
	if err != nil {
		return nil, nil, err
	}
	if data {
		return nil, &f, nil
	}
	kind := p[1] & 0xf0
	var out [][]byte
	finish := func() {
		if s.done {
			return
		}
		for _, yes := range s.echoed {
			if !yes {
				return
			}
		}
		out = append(out, []byte{3, byte(0x90 | s.Remote), 2})
		s.done = true
	}
	switch kind {
	case 0x90:
		if len(p) != 3 || p[2] > 2 {
			return nil, nil, ErrFrame
		}
		if p[2] == 0 && !s.probed {
			s.probed = true
			out = append(out, []byte{3, byte(0x90 | s.Remote), 1})
			for i := range s.probes {
				b := s.probes[i][:]
				b[0] = 7
				b[1] = byte(0x20 | s.Remote)
				b[2] = byte(i)
				binary.LittleEndian.PutUint32(b[3:], math.Float32bits(seconds))
				out = append(out, append([]byte(nil), b...))
			}
		}
		if p[2] == 2 {
			finish()
		}
	case 0x20:
		if len(p) != 7 || p[2] > 7 {
			return nil, nil, ErrFrame
		}
		out = append(out, append([]byte{8, byte(0x30 | s.Remote), byte(s.Local)}, p[2:]...))
	case 0x30:
		if len(p) != 8 || !s.probed || int(p[2]) != s.Remote || p[3] > 7 {
			return nil, nil, ErrFrame
		}
		i := int(p[3])
		probe := s.probes[i][:]
		if !bytes.Equal(p[3:], probe[2:]) {
			return nil, nil, ErrFrame
		}
		s.echoed[i] = true
		finish()
	default:
		return nil, nil, ErrFrame
	}
	return out, nil, nil
}

// KDDIPeers adapts the Japanese transport for every actual Western participant
// of a round. Measured with three original Japanese clients (reference server,
// java-f1-jp3-game.pcap): each client broadcasts its phases (0x90) and probes
// (0x20), and echoes every other peer's probes with an explicit target (0x30).
// Records aimed at an actual Japanese peer are relayed verbatim elsewhere.
type KDDIPeers struct {
	Local int
	peers []*KDDIPeer
}

func NewKDDIPeers(local int, remotes []int) (*KDDIPeers, error) {
	if len(remotes) == 0 || len(remotes) > 3 {
		return nil, ErrFrame
	}
	m := &KDDIPeers{Local: local}
	seen := map[int]bool{}
	for _, r := range remotes {
		if seen[r] {
			return nil, ErrFrame
		}
		seen[r] = true
		p, err := NewKDDIPeer(local, r)
		if err != nil {
			return nil, err
		}
		m.peers = append(m.peers, p)
	}
	return m, nil
}
func (m *KDDIPeers) Start() [][]byte {
	var out [][]byte
	for _, p := range m.peers {
		out = append(out, p.Start()...)
	}
	return out
}

// Receive feeds one actual record of the local client. Data records yield one
// frame; transport records are answered by each simulated Western peer, and an
// echo aimed at an actual Japanese peer is ignored here.
func (m *KDDIPeers) Receive(p []byte, seconds float32) ([][]byte, *Frame, error) {
	if len(p) < 2 || int(p[0]) != len(p) || int(p[1]&15) != m.Local {
		return nil, nil, ErrFrame
	}
	if f, data, err := JapaneseData(p); err != nil {
		return nil, nil, err
	} else if data {
		return nil, &f, nil
	}
	var out [][]byte
	if p[1]&0xf0 == 0x30 {
		if len(p) != 8 {
			return nil, nil, ErrFrame
		}
		for _, peer := range m.peers {
			if peer.Remote == int(p[2]) {
				return peer.Receive(p, seconds)
			}
		}
		return nil, nil, nil
	}
	for _, peer := range m.peers {
		replies, _, err := peer.Receive(p, seconds)
		if err != nil {
			return nil, nil, err
		}
		out = append(out, replies...)
	}
	return out, nil, nil
}
