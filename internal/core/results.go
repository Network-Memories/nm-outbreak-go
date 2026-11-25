package core

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"networkmemories/nm-outbreak-go/internal/results"
	"networkmemories/nm-outbreak-go/internal/world"
)

type scenarioRound struct {
	journals map[journalRoute]*journalStream

	engineMailboxes   map[string]*engineMailbox
	nativeNumber      string
	room              world.Room
	roomID            string
	id                string
	game              world.Game
	roster            map[string]Session
	entered, returned map[string]bool
}

// ConfigureResults must run before the listeners start. One ledger serves both dialects.
func (e *Engine) ConfigureResults(ledger *results.Ledger) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if ledger == nil || len(e.sessions) > 0 {
		return errors.New("configure results before native sessions")
	}
	e.resultLedger = ledger
	return nil
}

// ScenarioPresence receives a measured native transition from an authenticated
// transport. Every member of a mixed room shares one server-generated run ID.
func (e *Engine) ScenarioPresence(id string, inScenario bool) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	s := e.sessions[id]
	if s == nil {
		return ErrSession
	}
	if !inScenario {
		round := e.sessionRounds[id]
		firstReturn := round != nil && round.entered[id] && !round.returned[id]
		room, _ := e.worlds.GetRoom(s.RoomID)
		if firstReturn {
			if err := e.roomNotificationsReadyLocked(room, id); err != nil {
				return err
			}
		}
		s.InScenario = false
		if round != nil {
			round.returned[id] = true
			if firstReturn && room != nil {
				e.emitRoomEventLocked(RoomAftergameReturned, s, room, len(room.Members), false)
			}
			all := true
			for entered := range round.roster {
				if !round.returned[entered] {
					all = false
				}
			}
			if all && e.roomRounds[round.roomID] == round {
				delete(e.roomRounds, round.roomID)
				e.closeEngineRoundLocked(round)
				e.cancelLaunchesLocked(round.roomID)
				e.worlds.SetPlaying(round.roomID, false)
			}
		}
		return nil
	}
	room, ok := e.worlds.GetRoom(s.RoomID)
	if !ok {
		return ErrSession
	}
	round, err := e.scenarioRoundLocked(room)
	if err != nil {
		return err
	}
	if _, ok := round.roster[id]; !ok {
		return ErrSession
	}
	s.InScenario = true
	e.worlds.SetPlaying(room.ID, true)
	round.entered[id] = true
	delete(round.returned, id)
	e.sessionRounds[id] = round
	return nil
}

// scenarioRoundLocked freezes the same roster and UUID before either native
// family enters its scenario. Results still require each real entry transition.
func (e *Engine) scenarioRoundLocked(room *world.Room) (*scenarioRound, error) {
	if e.roomRounds == nil {
		e.roomRounds = make(map[string]*scenarioRound)
		e.sessionRounds = make(map[string]*scenarioRound)
	}
	round := e.roomRounds[room.ID]
	if round == nil {
		var raw [16]byte
		if _, err := rand.Read(raw[:]); err != nil {
			return nil, err
		}
		round = &scenarioRound{roomID: room.ID, id: hex.EncodeToString(raw[:]), game: room.Game, roster: map[string]Session{}, entered: map[string]bool{}, returned: map[string]bool{}}
		for _, m := range room.Members {
			member := e.sessions[m.Connection]
			if member == nil {
				return nil, ErrSession
			}
			round.roster[m.Connection] = cloneSession(*member)
		}
		round.room = copyEventRoom(*room)
		round.nativeNumber = fmt.Sprintf("%015d", binary.BigEndian.Uint64(raw[:8])%999999999999999+1)
		e.roomRounds[room.ID] = round
	}
	return round, nil
}
func (e *Engine) RecordResult(id string, stats results.Stats) error {
	e.mu.Lock()
	s := e.sessions[id]
	round := e.sessionRounds[id]
	ledger := e.resultLedger
	if s == nil || round == nil || !round.entered[id] || ledger == nil {
		e.mu.Unlock()
		return ErrSession
	}
	proved, ok := round.roster[id]
	if !ok || proved.Account.UserID != s.Account.UserID || proved.Game != s.Game {
		e.mu.Unlock()
		return ErrSession
	}
	definition := e.modules[s.Game]
	if (stats.Scenario != nil && int(*stats.Scenario) >= definition.ResultScenarios) || (stats.Mode != nil && int(*stats.Mode) >= definition.ResultModes) {
		e.mu.Unlock()
		return errors.New("unmeasured result scenario or category")
	}
	record := results.Record{RoundID: round.id, Game: round.game, OwnerID: proved.Account.UserID, Stats: stats, Sources: []results.Source{{Protocol: string(proved.Protocol), Region: proved.Region}}}
	e.mu.Unlock()
	return ledger.Accept(record)
}
func (e *Engine) Results() *results.Ledger { e.mu.Lock(); defer e.mu.Unlock(); return e.resultLedger }

// APP has no independent password field. Match the claimed alias to an already
// proved SNAP identity and its active native round; the IP is only a transport
// check, never an identity selector. Ambiguous contexts stay unranked.
func (e *Engine) APPResultContext(game world.Game, login, host string) (Session, string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	var found *Session
	var roundID string
	for _, s := range e.sessions {
		if s.Game != game || s.Protocol != SNAP || s.NativeLogin != login || s.NativeHost == "" || s.NativeHost != host {
			continue
		}
		round := e.sessionRounds[s.ID]
		if round == nil || !round.entered[s.ID] {
			continue
		}
		proved, ok := round.roster[s.ID]
		if !ok || proved.Account.UserID != s.Account.UserID {
			continue
		}
		if found != nil {
			return Session{}, "", ErrSession
		}
		found = s
		roundID = round.id
	}
	if found == nil {
		return Session{}, "", ErrSession
	}
	return cloneSession(*found), roundID, nil
}
