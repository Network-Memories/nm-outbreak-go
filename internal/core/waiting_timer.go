package core

import (
	"errors"
	"networkmemories/nm-outbreak-go/internal/modules"
	"networkmemories/nm-outbreak-go/internal/world"
	"time"
)

func (e *Engine) nativeRuleValuesLocked(room world.Room, schema []modules.KDDIRule) ([]byte, error) {
	values := append([]byte(nil), room.RuleValues...)
	if len(values) != len(schema) {
		host := e.sessions[room.HostConn]
		project := e.modules[room.Game].SNAPWaitingRules
		if host == nil || host.Protocol != SNAP || project == nil {
			return nil, errors.New("native rule projection unavailable")
		}
		var err error
		values, err = project(room.Area, room.StatWord, host.Profile)
		if err != nil {
			return nil, err
		}
	}
	if len(values) != len(schema) {
		return nil, errors.New("native rule count differs")
	}
	for i, v := range values {
		if int(v) >= len(schema[i].Choices) || (!schema[i].Editable && v != schema[i].Default) {
			return nil, errors.New("native rule differs from declared module")
		}
	}
	return values, nil
}

// KDDIRemaining uses a server-owned creation instant and received rule choices.
// Join/profile/stat updates cannot reset the wait. Zero is a timer value only,
// never proof that an original host announced or completed its launch.
func (e *Engine) KDDIRemaining(id string, slot uint16, now time.Time) (uint16, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	actor := e.sessions[id]
	if actor == nil {
		return 0, ErrSession
	}
	slots, err := e.kddiSlotsLocked(id)
	if err != nil {
		return 0, err
	}
	room, exists := slots[slot]
	if !exists || room.CreatedAt.IsZero() {
		return 0, ErrSession
	}
	return e.waitingRemainingLocked(room, actor.NativeLobbyID, now)
}

func (e *Engine) waitingRemainingLocked(room world.Room, nativeArea uint32, now time.Time) (uint16, error) {
	module := e.modules[room.Game]
	if nativeArea == 0 || nativeArea > uint32(len(module.KDDI.Areas)) || room.CreatedAt.IsZero() {
		return 0, ErrSession
	}
	schema := module.KDDI.Areas[nativeArea-1].Rules
	values, err := e.nativeRuleValuesLocked(room, schema)
	if err != nil {
		return 0, err
	}
	if int(module.WaitingRule) >= len(values) || int(values[module.WaitingRule]) >= len(module.WaitingSeconds) {
		return 0, ErrSession
	}
	seconds := module.WaitingSeconds[values[module.WaitingRule]]
	remaining := room.CreatedAt.Add(time.Duration(seconds)*time.Second).Sub(now) / time.Second
	if remaining < 0 {
		remaining = 0
	}
	if remaining > time.Duration(seconds) {
		remaining = time.Duration(seconds)
	}
	return uint16(remaining), nil
}

// WesternGuestWait resolves the timer only for a greeted guest and its actual
// Japanese host. The same scope also protects the other waiting replies.
func (e *Engine) WesternGuestWait(id string, target uint32, now time.Time) (uint16, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	guest := e.sessions[id]
	if guest == nil || guest.Protocol != SNAP || guest.InScenario || !e.modules[guest.Game].SNAPEntryGreeting {
		return 0, ErrSession
	}
	room, ok := e.worlds.GetRoom(guest.RoomID)
	if !ok || room.Playing || e.roomRounds[room.ID] != nil {
		return 0, ErrSession
	}
	host := e.sessions[room.HostConn]
	state := e.waiting[id]
	if host == nil || host.Protocol != KDDI || host.InScenario || host.WireID != target ||
		state.roomID != room.ID || state.hostID != host.ID {
		return 0, ErrSession
	}
	return e.waitingRemainingLocked(*room, host.NativeLobbyID, now)
}
