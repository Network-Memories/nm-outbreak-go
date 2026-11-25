package core

import (
	"crypto/rand"
	"encoding/binary"
)

// BindSNAPID attaches a proved native routing ID; it cannot change an existing identity.
func (e *Engine) BindSNAPID(id string, wire uint32) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	s := e.sessions[id]
	if s == nil || s.Protocol != SNAP || wire == 0 || (s.WireID != 0 && s.WireID != wire) {
		return ErrSession
	}
	for _, other := range e.sessions {
		if other.ID != id && other.WireID == wire {
			return ErrSession
		}
	}
	s.WireID = wire
	return nil
}

// SNAPID uses the real western SID or allocates one stable adapter ID for a
// proved Japanese session. A roster seat is never used as a player identity.
func (e *Engine) SNAPID(id string) (uint32, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.snapIDLocked(id)
}
func (e *Engine) snapIDLocked(id string) (uint32, error) {
	s := e.sessions[id]
	if s == nil {
		return 0, ErrSession
	}
	if s.WireID != 0 {
		return s.WireID, nil
	}
	for {
		var raw [4]byte
		if _, err := rand.Read(raw[:]); err != nil {
			return 0, err
		}
		wire := binary.BigEndian.Uint32(raw[:])
		if wire == 0 || e.snapLogins[wire] != nil {
			continue
		}
		used := false
		for _, other := range e.sessions {
			if other.WireID == wire {
				used = true
				break
			}
		}
		if !used {
			s.WireID = wire
			return wire, nil
		}
	}
}
