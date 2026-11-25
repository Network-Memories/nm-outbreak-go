package core

import "networkmemories/nm-outbreak-go/internal/world"

// SetArea records an actual native lobby join, not an IP-derived presence.
func (e *Engine) SetArea(id string, area int) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	var hallBefore *Session
	if h := e.sessions[id]; h != nil {
		snapshot := cloneSession(*h)
		hallBefore = &snapshot
	}
	defer func() { e.emitHallChangeLocked(hallBefore, e.sessions[id]) }()
	s := e.sessions[id]
	if s == nil {
		return ErrSession
	}
	valid := area == 0
	for _, a := range e.modules[s.Game].Areas {
		if a == area {
			valid = true
			break
		}
	}
	if !valid {
		return ErrSession
	}
	if s.Area != area {
		s.presenceRevision++
	}
	s.Area = area
	return nil
}
func (e *Engine) AreaPopulation(game world.Game) uint16 {
	e.mu.Lock()
	defer e.mu.Unlock()
	n := 0
	for _, s := range e.sessions {
		if s.Game == game && s.Area > 0 {
			n++
		}
	}
	if n > 65535 {
		return 65535
	}
	return uint16(n)
}

func (e *Engine) SetNativeLobby(id string, native uint32) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	var hallBefore *Session
	if h := e.sessions[id]; h != nil {
		snapshot := cloneSession(*h)
		hallBefore = &snapshot
	}
	defer func() { e.emitHallChangeLocked(hallBefore, e.sessions[id]) }()
	s := e.sessions[id]
	if s == nil {
		return ErrSession
	}
	if s.NativeLobbyID != native {
		s.presenceRevision++
	}
	s.NativeLobbyID = native
	return nil
}
func (e *Engine) NativePopulation(game world.Game, protocol Protocol, native uint32) uint32 {
	e.mu.Lock()
	defer e.mu.Unlock()
	var n uint32
	for _, s := range e.sessions {
		if s.Game == game && s.NativeLobbyID == native && (s.Protocol == protocol || e.nativeHallMemberLocked(s, e.modules[game].NativeSharedHallAreas)) {
			n++
		}
	}
	return n
}

// OtherProtocolHallPopulation supplements a native directory with actual hall
// presences of the other dialect. Party/directory/game counters stay unchanged
// until their native partition is measured; this never counts a web ticket.
func (e *Engine) OtherProtocolHallPopulation(game world.Game, protocol Protocol, native uint32) uint16 {
	e.mu.Lock()
	defer e.mu.Unlock()
	var n uint32
	for _, s := range e.sessions {
		if s.Game == game && s.Protocol != protocol && s.NativeLobbyID == native && e.nativeHallMemberLocked(s, e.modules[game].NativeSharedHallAreas) {
			n++
		}
	}
	if n > 65535 {
		return 65535
	}
	return uint16(n)
}

// KDDIBrowsePopulation counts only attached lobby transports. Native room IDs
// describe browse directories, not scenario membership; keep that distinction.
func (e *Engine) KDDIBrowsePopulation(game world.Game, native uint32) (uint16, uint16) {
	e.mu.Lock()
	defer e.mu.Unlock()
	var lobby, rooms uint32
	for _, s := range e.sessions {
		if s.Game != game || s.Protocol != KDDI || s.NativeLobbyID != native || !e.kddiLobbyAttached[s.ID] {
			continue
		}
		if s.NativeRoomID == 0 {
			lobby++
		} else {
			rooms++
		}
	}
	for _, s := range e.sessions {
		if s.Game == game && s.Protocol != KDDI && s.NativeLobbyID == native && e.nativeHallMemberLocked(s, e.modules[game].NativeSharedHallAreas) {
			lobby++
		}
	}
	if lobby > 65535 {
		lobby = 65535
	}
	if rooms > 65535 {
		rooms = 65535
	}
	return uint16(lobby), uint16(rooms)
}
