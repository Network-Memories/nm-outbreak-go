package core

import "crypto/sha256"

// Waiting-room state is proved by native6504, independently of membership,
// greeting delivery and the later launch acknowledgement.
type waitingState struct {
	profile        [32]byte
	roomID, hostID string
	ready          bool
}

func (e *Engine) SignalWaitingGreeting(hostID string, target uint32) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	host := e.sessions[hostID]
	if host == nil || host.Protocol != SNAP || host.InScenario || !e.modules[host.Game].SNAPEntryGreeting {
		return ErrSession
	}
	room, ok := e.worlds.GetRoom(host.RoomID)
	if !ok || room.HostConn != hostID || room.Playing {
		return ErrSession
	}
	var guest *Session
	for _, m := range room.Members {
		p := e.sessions[m.Connection]
		if p != nil && p.Protocol == KDDI && p.WireID == target {
			guest = p
			break
		}
	}
	if guest == nil || guest.InScenario {
		return ErrSession
	}
	state := e.waiting[guest.ID]
	if state.roomID != room.ID {
		state = waitingState{roomID: room.ID}
	}
	if state.hostID == hostID {
		return nil
	}
	if !e.launchEventReadyLocked(guest.ID) {
		return ErrRoomBackpressure
	}
	state.hostID = hostID
	// The original File2 guest waits for this actual host signal before
	// refreshing its roster and sending6504. Gating it on6504 deadlocks.
	e.launchEventLocked(RoomWaitingGreeting, host, guest, room)
	if e.waiting == nil {
		e.waiting = make(map[string]waitingState)
	}
	e.waiting[guest.ID] = state
	return nil
}
func (e *Engine) MarkKDDIWaitingReady(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	actor := e.sessions[id]
	if actor == nil || actor.Protocol != KDDI || actor.InScenario {
		return ErrSession
	}
	room, ok := e.worlds.GetRoom(actor.RoomID)
	if !ok || room.Playing {
		return ErrSession
	}
	state := e.waiting[id]
	if state.roomID != room.ID {
		state = waitingState{roomID: room.ID}
	}
	digest := sha256.Sum256(actor.Profile)
	if state.ready && state.profile == digest {
		return nil
	}
	if err := e.roomNotificationsReadyLocked(room, ""); err != nil {
		return err
	}
	state.ready = true
	state.profile = digest
	e.emitRoomEventLocked(RoomWaitingReady, actor, room, len(room.Members), false)
	if e.waiting == nil {
		e.waiting = make(map[string]waitingState)
	}
	e.waiting[id] = state
	return nil
}
func (e *Engine) clearWaitingLocked(roomID string) {
	for id, state := range e.waiting {
		if state.roomID == roomID {
			delete(e.waiting, id)
		}
	}
}

// SignalKDDIWaitingGreeting forwards the native host's entry signal to its
// authenticated western guest. Membership alone does not prove this signal.
func (e *Engine) SignalKDDIWaitingGreeting(hostID, handle string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	host := e.sessions[hostID]
	if host == nil || host.Protocol != KDDI || host.InScenario || !e.modules[host.Game].SNAPEntryGreeting {
		return ErrSession
	}
	room, ok := e.worlds.GetRoom(host.RoomID)
	if !ok || room.HostConn != hostID || room.Playing {
		return ErrSession
	}
	for _, member := range room.Members {
		guest := e.sessions[member.Connection]
		if guest == nil || guest.RoomID != room.ID || guest.Protocol != SNAP || guest.InScenario || guest.Account.Handle != handle {
			continue
		}
		state := e.waiting[guest.ID]
		if state.roomID == room.ID && state.hostID == hostID {
			return nil
		}
		if !e.launchEventReadyLocked(guest.ID) {
			return ErrRoomBackpressure
		}
		e.launchEventLocked(RoomWaitingGreeting, host, guest, room)
		if e.waiting == nil {
			e.waiting = make(map[string]waitingState)
		}
		e.waiting[guest.ID] = waitingState{roomID: room.ID, hostID: hostID}
		return nil
	}
	return ErrSession
}
