package core

import "networkmemories/nm-outbreak-go/internal/world"

// A journal belongs to a proved round and a directed pair, never an IP or a
// reusable native handle. Its opaque bytes are not scores or synthetic events.
type journalRoute struct{ from, to string }
type journalStream struct {
	data                []byte
	complete, delivered bool
}

func (e *Engine) ReceiveAftergameJournal(id, target string, more bool, chunk []byte) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	actor, peer := e.sessions[id], e.sessions[target]
	round := e.sessionRounds[id]
	if actor == nil || peer == nil || id == target || actor.Protocol == peer.Protocol || round == nil || e.sessionRounds[target] != round || actor.Game != peer.Game || actor.RoomID != round.roomID || peer.RoomID != round.roomID || actor.InScenario || peer.InScenario || !round.entered[id] || !round.entered[target] || !round.returned[id] || !round.returned[target] {
		return ErrSession
	}
	for _, s := range []*Session{actor, peer} {
		frozen, ok := round.roster[s.ID]
		if !ok || frozen.Account.UserID != s.Account.UserID {
			return ErrSession
		}
	}
	if e.roomSubscriptions[id] == nil || e.roomSubscriptions[target] == nil {
		return ErrSession
	}
	size := e.modules[actor.Game].AftergameJournalBytes
	step := 32
	if actor.Protocol == KDDI {
		step = 64
	}
	if size != 2048 || len(chunk) != step {
		return ErrSession
	}
	_, ok := e.worlds.GetRoom(round.roomID)
	if !ok {
		return ErrSession
	}
	if round.journals == nil {
		round.journals = make(map[journalRoute]*journalStream)
	}
	key := journalRoute{id, target}
	stream := round.journals[key]
	if stream == nil {
		stream = &journalStream{}
		round.journals[key] = stream
	}
	if stream.complete {
		return ErrSession
	}
	next := len(stream.data) + step
	if next > size || more != (next < size) {
		return ErrSession
	}
	stream.data = append(stream.data, chunk...)
	stream.complete = next == size
	e.flushAftergameJournalsLocked(round)
	return nil
}

// A transport calls this as it consumes room events. A slow recipient holds a
// bounded, complete journal in the round instead of losing an already ACKed
// SNAP chunk. No network I/O runs under the engine lock.
func (e *Engine) FlushAftergameJournals(id string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if round := e.sessionRounds[id]; round != nil {
		e.flushAftergameJournalsLocked(round)
	}
}
func (e *Engine) flushAftergameJournalsLocked(round *scenarioRound) {
	room, ok := e.worlds.GetRoom(round.roomID)
	if !ok {
		return
	}
	for route, data := range round.journals {
		ready := round.journals[journalRoute{route.to, route.from}]
		if !data.complete || data.delivered || ready == nil || len(ready.data) == 0 {
			continue
		}
		from, to := e.sessions[route.from], e.sessions[route.to]
		valid := true
		for _, peer := range []*Session{from, to} {
			if peer == nil {
				valid = false
				break
			}
			frozen, ok := round.roster[peer.ID]
			if !ok || frozen.Account.UserID != peer.Account.UserID || e.sessionRounds[peer.ID] != round || peer.RoomID != round.roomID || peer.InScenario || !round.returned[peer.ID] {
				valid = false
				break
			}
		}
		if !valid {
			continue
		}
		sub := e.roomSubscriptions[route.to]
		if sub == nil || len(sub.events) >= roomEventLimit {
			continue
		}
		// Each recipient has actually begun its own native journal exchange; its
		// receiver table is ready. Copies prevent native writers changing history.
		select {
		case sub.events <- RoomEvent{Kind: RoomAftergameJournal, Actor: cloneSession(*from), Room: copyEventRoom(*room), Journal: append([]byte(nil), data.data...)}:
			data.delivered = true
		default:
		}
	}
}

// AftergameJournalSupported reports a game whose native after-game journal
// envelopes are measured (2048 bytes: 32-byte SN@P and 64-byte KDDI blocks).
func (e *Engine) AftergameJournalSupported(game world.Game) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.modules[game].AftergameJournalBytes == 2048
}
