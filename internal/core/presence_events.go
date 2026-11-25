package core

import "networkmemories/nm-outbreak-go/internal/world"

// Cross-dialect hall changes reuse the bounded native event stream. No polling,
// network I/O, account provisioning or fictitious party member is introduced.
func (e *Engine) emitHallChangeLocked(before, after *Session) {
	actor := after
	if actor == nil {
		actor = before
	}
	if actor == nil {
		return
	}
	areas := e.modules[actor.Game].NativeSharedHallAreas
	old := e.nativeHallMemberLocked(before, areas)
	next := e.nativeHallMemberLocked(after, areas)
	if old && (!next || before.NativeLobbyID != after.NativeLobbyID) {
		e.emitHallPopulationLocked(before, -1)
	}
	if next && (!old || before.NativeLobbyID != after.NativeLobbyID) {
		e.emitHallPopulationLocked(after, 1)
	}
}
func (e *Engine) emitHallPopulationLocked(actor *Session, delta int) {
	for id, sub := range e.roomSubscriptions {
		peer := e.sessions[id]
		if peer == nil || peer.Protocol == actor.Protocol || peer.Game != actor.Game || peer.NativeLobbyID != actor.NativeLobbyID || !e.nativeHallMemberLocked(peer, e.modules[peer.Game].NativeSharedHallAreas) {
			continue
		}
		ev := RoomEvent{Kind: HallPopulationChanged, Actor: cloneSession(*actor), Room: world.Room{Game: actor.Game}, PopulationArea: actor.NativeLobbyID, Count: delta, populationRecipientRevision: peer.presenceRevision}
		select {
		case sub.events <- ev:
		default:
			sub.overflow.Store(true)
			delete(e.roomSubscriptions, id)
			close(sub.events)
		}
	}
}
func (e *Engine) HallPopulationDeliverable(id string, event RoomEvent) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	peer := e.sessions[id]
	return peer != nil && event.Kind == HallPopulationChanged && (event.Count == 1 || event.Count == -1) && peer.Game == event.Room.Game && peer.NativeLobbyID == event.PopulationArea && peer.presenceRevision == event.populationRecipientRevision && e.nativeHallMemberLocked(peer, e.modules[peer.Game].NativeSharedHallAreas)
}
