package core

import "networkmemories/nm-outbreak-go/internal/world"

type kddiPreview struct {
	slot   uint16
	roomID string
}

// Preview and admission refer to the same authoritative room. If a previewed
// slot is freed and reused, the old character-selection screen cannot join its
// replacement silently.
func (e *Engine) PreviewKDDISlot(id string, slot uint16) (*world.Room, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	slots, err := e.kddiSlotsLocked(id)
	if err != nil {
		return nil, err
	}
	room, ok := slots[slot]
	if !ok {
		return nil, ErrSession
	}
	if e.kddiPreviews == nil {
		e.kddiPreviews = make(map[string]kddiPreview)
	}
	e.kddiPreviews[id] = kddiPreview{slot, room.ID}
	return &room, nil
}
func (e *Engine) JoinKDDISlot(id string, slot uint16, password []byte) (*world.Room, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.kddiDrafts[id] != nil {
		return nil, ErrSession
	}
	slots, err := e.kddiSlotsLocked(id)
	if err != nil {
		return nil, err
	}
	room, ok := slots[slot]
	if !ok {
		return nil, ErrSession
	}
	if p, ok := e.kddiPreviews[id]; ok && (p.slot != slot || p.roomID != room.ID) {
		return nil, ErrSession
	}
	return e.joinRoomLocked(id, room.ID, password)
}
