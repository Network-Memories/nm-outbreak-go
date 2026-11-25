package core

// AftergameMembers exposes only authenticated members which actually entered
// this round and returned. An in-game peer keeps its frozen seat but is not yet
// counted as present in the meeting room. No owner is inferred from an address.
func (e *Engine) AftergameMembers(id string) ([]Session, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	s := e.sessions[id]
	round := e.sessionRounds[id]
	if s == nil || round == nil || !round.entered[id] || !round.returned[id] || s.InScenario || s.RoomID != round.roomID {
		return nil, ErrSession
	}
	var members []Session
	for _, seat := range round.room.Members {
		peer := e.sessions[seat.Connection]
		frozen, ok := round.roster[seat.Connection]
		if !ok || peer == nil || peer.RoomID != round.roomID || peer.Account.UserID != frozen.Account.UserID || peer.InScenario || !round.entered[peer.ID] || !round.returned[peer.ID] {
			continue
		}
		members = append(members, cloneSession(*peer))
	}
	return members, nil
}

// HasScenarioRound gates the native post-game report request. A ticket, room
// membership or another player's entry is insufficient evidence.
func (e *Engine) HasScenarioRound(id string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	s := e.sessions[id]
	round := e.sessionRounds[id]
	if s == nil || round == nil || !round.entered[id] || s.RoomID != round.roomID {
		return false
	}
	frozen, ok := round.roster[id]
	return ok && frozen.Account.UserID == s.Account.UserID
}

// KDDIReportRound links structural evidence to the shared round only when the
// authenticated player's own entry and the reported native number both agree.
// It does not interpret the report's opaque summary as scores.
func (e *Engine) KDDIReportRound(id string, nativeNumber string) (Session, string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	s := e.sessions[id]
	round := e.sessionRounds[id]
	if s == nil || s.Protocol != KDDI || round == nil || !round.entered[id] || nativeNumber != round.nativeNumber {
		return Session{}, "", ErrSession
	}
	frozen, ok := round.roster[id]
	if !ok || frozen.Account.UserID != s.Account.UserID || frozen.Game != s.Game {
		return Session{}, "", ErrSession
	}
	return cloneSession(*s), round.id, nil
}

// ProjectSNAPAftergameProfile adapts only a proven Japanese return, through the
// game's measured module. Original western properties remain byte-for-byte.
func (e *Engine) ProjectSNAPAftergameProfile(id string, profile []byte) ([]byte, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	s := e.sessions[id]
	if s == nil {
		return nil, ErrSession
	}
	out := append([]byte(nil), profile...)
	round := e.sessionRounds[id]
	if s.Protocol != KDDI || s.InScenario || round == nil || !round.entered[id] || !round.returned[id] || s.RoomID != round.roomID {
		return out, nil
	}
	frozen, ok := round.roster[id]
	if !ok || frozen.Account.UserID != s.Account.UserID {
		return nil, ErrSession
	}
	project := e.modules[s.Game].SNAPAftergameProfile
	if project == nil {
		return out, nil
	}
	return project(out)
}
