package core

import "time"

// SessionSilenceLimit bounds how long a session may stay without any proof
// of life (native inbound traffic or an open Japanese lobby socket) before it
// is logged out and its room place released. Native keepalives are 20-30 s;
// the measured Japanese lobby-to-game and after-game reconnections take
// about one minute.
const SessionSilenceLimit = 180 * time.Second

// RoundSilenceLimit applies to members of a scenario round. Between the end
// of play and the after-game lobby reconnection an original client sends
// nothing while its player reads the results and saves (measured: several
// minutes, paced by the player), so the round keeps its seats much longer.
const RoundSilenceLimit = 30 * time.Minute

// adoptSessionLocked starts the proof-of-life clock of a new session. Other
// sessions of the same account are left alone: measured logins legitimately
// overlap (bootstrap then game handoff), so only silence releases a place.
func (e *Engine) adoptSessionLocked(s *Session) {
	if e.lastAlive == nil {
		e.lastAlive = make(map[string]time.Time)
	}
	e.lastAlive[s.ID] = time.Now()
}

// MarkAlive is called by a transport for every authenticated inbound native
// message of a session.
func (e *Engine) MarkAlive(id string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.sessions[id] == nil {
		return
	}
	if e.lastAlive == nil {
		e.lastAlive = make(map[string]time.Time)
	}
	e.lastAlive[id] = time.Now()
}

// ReapSilent logs out sessions without proof of life since SessionSilenceLimit.
// An attached Japanese lobby socket carries its own heartbeat and read deadline.
func (e *Engine) ReapSilent(now time.Time) []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	var reaped []string
	for id := range e.sessions {
		if e.kddiLobbyAttached[id] {
			continue
		}
		last, ok := e.lastAlive[id]
		if !ok {
			if e.lastAlive == nil {
				e.lastAlive = make(map[string]time.Time)
			}
			e.lastAlive[id] = now
			continue
		}
		limit := SessionSilenceLimit
		if e.sessionRounds[id] != nil {
			limit = RoundSilenceLimit
		}
		if now.Sub(last) < limit {
			continue
		}
		e.logoutLocked(id)
		reaped = append(reaped, id)
	}
	return reaped
}

// RunReaper releases silent sessions until done is closed.
func (e *Engine) RunReaper(done <-chan struct{}, every time.Duration, report func(ids []string)) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-done:
			return
		case now := <-t.C:
			if ids := e.ReapSilent(now); len(ids) > 0 && report != nil {
				report(ids)
			}
		}
	}
}
