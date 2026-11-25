package core

import (
	"networkmemories/nm-outbreak-go/internal/world"
	"unicode"
	"unicode/utf8"
)

// Native browse directories and actual party membership are distinct. The
// Japanese area broadcast in BioServer2 explicitly excludes browse room!=0.
func (e *Engine) areaChatMemberLocked(s *Session) bool {
	if s == nil {
		return false
	}
	return e.nativeHallMemberLocked(s, e.modules[s.Game].NativeChatAreas)
}

func (e *Engine) nativeHallMemberLocked(s *Session, areas []uint32) bool {
	if s == nil || s.InScenario || s.RoomID != "" || s.NativeLobbyID == 0 || s.Area == 0 || s.NativeRoomID != 0 {
		return false
	}
	if s.Protocol == KDDI && !e.kddiLobbyAttached[s.ID] {
		return false
	}
	for _, area := range areas {
		if s.NativeLobbyID == area && e.modules[s.Game].KDDI.Shared(uint16(area), 0) == s.Area {
			return true
		}
	}
	return false
}
func (e *Engine) AreaChatSupported(id string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.areaChatMemberLocked(e.sessions[id])
}

// PublishAreaChat uses the authenticated area's measured cohort, not a claimed
// name or an address. A single bounded subscription carries both room and area
// events; no extra account, chat graph or transport service is introduced.
func (e *Engine) PublishAreaChat(id, text string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	a := e.sessions[id]
	if !e.areaChatMemberLocked(a) || len(text) < 1 || len(text) > 256 || !utf8.ValidString(text) {
		return ErrSession
	}
	for _, r := range text {
		if unicode.IsControl(r) {
			return ErrSession
		}
	}
	var targets []*Session
	for _, b := range e.sessions {
		sub := e.roomSubscriptions[b.ID]
		if sub == nil || b.Game != a.Game || b.NativeLobbyID != a.NativeLobbyID || !e.areaChatMemberLocked(b) || (a.ID == b.ID && a.Protocol == SNAP) {
			continue
		}
		if len(sub.events) >= roomEventLimit {
			return ErrRoomBackpressure
		}
		targets = append(targets, b)
	}
	for _, b := range targets {
		e.roomSubscriptions[b.ID].events <- RoomEvent{Kind: RoomChat, Actor: cloneSession(*a), Room: world.Room{Game: a.Game}, ChatArea: a.NativeLobbyID, ChatText: text, chatRecipientRevision: b.presenceRevision}
	}
	return nil
}
func (e *Engine) areaChatDeliverableLocked(id string, event RoomEvent) bool {
	a, b := e.sessions[event.Actor.ID], e.sessions[id]
	return event.Kind == RoomChat && event.ChatArea != 0 && e.areaChatMemberLocked(a) && e.areaChatMemberLocked(b) && a.Account.UserID == event.Actor.Account.UserID && a.Game == b.Game && a.Game == event.Room.Game && a.NativeLobbyID == event.ChatArea && b.NativeLobbyID == event.ChatArea && a.presenceRevision == event.Actor.presenceRevision && b.presenceRevision == event.chatRecipientRevision
}
