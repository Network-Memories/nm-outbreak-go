package core

import (
	"unicode"
	"unicode/utf8"
)

func (e *Engine) aftergameChatLocked(s *Session) bool {
	r := e.sessionRounds[s.ID]
	if r == nil || s.InScenario || s.RoomID != r.roomID || !r.entered[s.ID] || !r.returned[s.ID] {
		return false
	}
	f, ok := r.roster[s.ID]
	return ok && f.Account.UserID == s.Account.UserID
}
func (e *Engine) chatPeersLocked(a, b *Session, aftergame bool) bool {
	if a == nil || b == nil || a.RoomID == "" || b.RoomID != a.RoomID || a.Game != b.Game || a.InScenario || b.InScenario || e.aftergameChatLocked(a) != aftergame || e.aftergameChatLocked(b) != aftergame {
		return false
	}
	return !aftergame || e.sessionRounds[a.ID] == e.sessionRounds[b.ID]
}

// PublishRoomChat uses neither a claimed name nor an address for identity.
// Waiting-room and returned-aftergame cohorts stay separate from active games.
func (e *Engine) PublishRoomChat(id string, aftergame bool, text string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	a := e.sessions[id]
	if a == nil || !e.modules[a.Game].NativeRoomChat || len(text) < 1 || len(text) > 256 || !utf8.ValidString(text) {
		return ErrSession
	}
	for _, r := range text {
		if unicode.IsControl(r) {
			return ErrSession
		}
	}
	if !e.chatPeersLocked(a, a, aftergame) {
		return ErrSession
	}
	room, ok := e.worlds.GetRoom(a.RoomID)
	if !ok {
		return ErrSession
	}
	var targets []*RoomSubscription
	for _, m := range room.Members {
		b := e.sessions[m.Connection]
		sub := e.roomSubscriptions[m.Connection]
		if sub == nil || !e.chatPeersLocked(a, b, aftergame) || (b.ID == a.ID && a.Protocol == SNAP) {
			continue
		}
		if len(sub.events) >= roomEventLimit {
			return ErrRoomBackpressure
		}
		targets = append(targets, sub)
	}
	for _, sub := range targets {
		sub.events <- RoomEvent{Kind: RoomChat, Actor: cloneSession(*a), Room: copyEventRoom(*room), ChatText: text, ChatAftergame: aftergame}
	}
	return nil
}

// Queued text expires when either authenticated participant leaves or changes
// overlay/round. A new occupant of a native slot never inherits this message.
func (e *Engine) RoomChatDeliverable(id string, event RoomEvent) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if event.ChatArea != 0 {
		return e.areaChatDeliverableLocked(id, event)
	}
	a, b := e.sessions[event.Actor.ID], e.sessions[id]
	return event.Kind == RoomChat && a != nil && a.Account.UserID == event.Actor.Account.UserID && a.RoomID == event.Room.ID && e.chatPeersLocked(a, b, event.ChatAftergame)
}
