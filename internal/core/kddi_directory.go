package core

import (
	"networkmemories/nm-outbreak-go/internal/modules"
	"networkmemories/nm-outbreak-go/internal/world"
)

type kddiBrowseKey struct {
	game       world.Game
	area, room uint16
}

// One native lobby socket owns a session's browse presence. Closing it removes
// only that presence, retaining the logical party and scenario result context.
func (e *Engine) AttachKDDILobby(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	s := e.sessions[id]
	if s == nil || s.Protocol != KDDI || e.kddiLobbyAttached[id] {
		return ErrSession
	}
	if e.kddiLobbyAttached == nil {
		e.kddiLobbyAttached = make(map[string]bool)
	}
	e.kddiLobbyAttached[id] = true
	if e.nativeHallMemberLocked(s, e.modules[s.Game].NativeSharedHallAreas) {
		e.emitHallPopulationLocked(s, 1)
	}
	return nil
}
func (e *Engine) DetachKDDILobby(id string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.kddiLobbyAttached[id] {
		return
	}
	if h := e.sessions[id]; h != nil && e.nativeHallMemberLocked(h, e.modules[h.Game].NativeSharedHallAreas) {
		e.emitHallPopulationLocked(h, -1)
	}
	delete(e.kddiLobbyAttached, id)
	if sub := e.roomSubscriptions[id]; sub != nil {
		delete(e.roomSubscriptions, id)
		close(sub.events)
	}
	e.cancelKDDIDraftLocked(id)
	delete(e.kddiPreviews, id)
	if s := e.sessions[id]; s != nil {
		s.presenceRevision++
		s.NativeLobbyID = 0
		s.NativeRoomID = 0
		s.Area = 0
	}
}

func (e *Engine) KDDIDirectory(game world.Game) modules.KDDIDirectory {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.modules[game].Clone().KDDI
}

// SetKDDIBrowse records only authenticated native navigation. Native browse rooms
// are directories of party slots, distinct from the shared party RoomID.
func (e *Engine) SetKDDIBrowse(id string, area, room uint16) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	var hallBefore *Session
	if h := e.sessions[id]; h != nil {
		snapshot := cloneSession(*h)
		hallBefore = &snapshot
	}
	defer func() { e.emitHallChangeLocked(hallBefore, e.sessions[id]) }()
	s := e.sessions[id]
	if s == nil || s.Protocol != KDDI || s.RoomID != "" || s.InScenario || e.kddiDrafts[id] != nil {
		return ErrSession
	}
	d := e.modules[s.Game].KDDI
	if area == 0 {
		if room != 0 {
			return ErrSession
		}
	} else {
		if int(area) > len(d.Areas) || d.Areas[area-1].Status != 3 {
			return ErrSession
		}
		if room != 0 && (int(room) > len(d.Rooms) || d.Rooms[room-1].Status != 3) {
			return ErrSession
		}
	}
	if s.NativeLobbyID != uint32(area) || s.NativeRoomID != room {
		s.presenceRevision++
	}
	s.NativeLobbyID = uint32(area)
	s.NativeRoomID = room
	s.Area = d.Shared(area, room)
	delete(e.kddiPreviews, id)
	return nil
}

// KDDISlots maintains stable coordinates for each native directory. Closing one
// party frees its slot without renumbering the remaining parties.
func (e *Engine) KDDISlots(id string) (map[uint16]world.Room, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.kddiSlotsLocked(id)
}
func (e *Engine) kddiSlotsLocked(id string) (map[uint16]world.Room, error) {
	s := e.sessions[id]
	if s == nil || s.Protocol != KDDI || s.NativeLobbyID == 0 || s.NativeRoomID == 0 {
		return nil, ErrSession
	}
	d := e.modules[s.Game].KDDI
	if s.Area == 0 {
		return map[uint16]world.Room{}, nil
	} // File1 automatic area is not shared free mode.
	key := kddiBrowseKey{s.Game, uint16(s.NativeLobbyID), s.NativeRoomID}
	if e.kddiSlots == nil {
		e.kddiSlots = make(map[kddiBrowseKey]map[string]uint16)
	}
	slots := e.kddiSlots[key]
	if slots == nil {
		slots = make(map[string]uint16)
		e.kddiSlots[key] = slots
	}
	rooms := make(map[string]world.Room)
	ordered := e.worlds.ListRooms(s.Game)
	for _, r := range ordered {
		if r.Area == s.Area {
			rooms[r.ID] = r
		}
	}
	used := make(map[uint16]bool)
	for _, draft := range e.kddiDrafts {
		if draft.key == key {
			used[draft.Slot] = true
		}
	}
	for id, n := range slots {
		if _, ok := rooms[id]; !ok {
			delete(slots, id)
		} else {
			used[n] = true
		}
	}
	for _, r := range ordered {
		if _, ok := rooms[r.ID]; !ok {
			continue
		}
		if _, ok := slots[r.ID]; ok {
			continue
		}
		for n := uint16(1); n <= d.Slots; n++ {
			if !used[n] {
				slots[r.ID] = n
				used[n] = true
				break
			}
		}
	}
	out := make(map[uint16]world.Room)
	for id, n := range slots {
		out[n] = rooms[id]
	}
	return out, nil
}

// KDDIConnected counts actual attached native lobby transports, not web tickets.
func (e *Engine) KDDIConnected(game world.Game) int {
	e.mu.Lock()
	defer e.mu.Unlock()
	n := 0
	for id, attached := range e.kddiLobbyAttached {
		if actor := e.sessions[id]; attached && actor != nil && actor.Game == game {
			n++
		}
	}
	return n
}
