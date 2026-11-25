package core

import (
	"errors"
	"networkmemories/nm-outbreak-go/internal/world"
	"sync/atomic"
)

type RoomEventKind byte

const (
	RoomJoined RoomEventKind = iota + 1
	RoomProfileChanged
	RoomLeft
	RoomLaunchAnnounced
	RoomLaunchAcknowledged
	RoomWaitingGreeting
	RoomWaitingReady
	RoomAftergameReturned
	RoomAftergameJournal
	RoomChat
	HallPopulationChanged
	RoomNativeEvent
)
const roomEventLimit = 64

var ErrRoomBackpressure = errors.New("native room notifications congested")

// Snapshots survive logout and are private to each destination. No transport
// code or socket I/O runs while the engine lock is held.
type RoomEvent struct {
	PopulationArea              uint32
	populationRecipientRevision uint64
	ChatArea                    uint32
	chatRecipientRevision       uint64

	ChatText      string
	ChatAftergame bool

	Journal []byte // immutable private snapshot of actual native data

	Kind       RoomEventKind
	Actor      Session
	Room       world.Room
	Count      int
	Disbanded  bool
	NativeSlot uint16
}
type RoomSubscription struct {
	events   chan RoomEvent
	overflow atomic.Bool
}

func (s *RoomSubscription) Events() <-chan RoomEvent { return s.events }
func (s *RoomSubscription) Overflowed() bool         { return s.overflow.Load() }

func (e *Engine) SubscribeRoom(id string) (*RoomSubscription, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	actor := e.sessions[id]
	if actor == nil {
		return nil, ErrSession
	}
	if e.roomSubscriptions == nil {
		e.roomSubscriptions = make(map[string]*RoomSubscription)
	}
	if old := e.roomSubscriptions[id]; old != nil {
		return old, nil
	}
	reserve := e.modules[actor.Game].PlayersMax
	if reserve < 4 {
		reserve = 4
	}
	s := &RoomSubscription{events: make(chan RoomEvent, roomEventLimit+reserve)}
	e.roomSubscriptions[id] = s
	return s, nil
}
func (e *Engine) UnsubscribeRoom(id string, s *RoomSubscription) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.roomSubscriptions[id] == s {
		delete(e.roomSubscriptions, id)
		close(s.events)
	}
}
func copyEventRoom(r world.Room) world.Room {
	r.RuleValues = append([]byte(nil), r.RuleValues...)
	r.Members = append([]world.Member(nil), r.Members...)
	for i := range r.Members {
		r.Members[i].Profile = append([]byte(nil), r.Members[i].Profile...)
	}
	if r.Settings != nil {
		m := make(map[string]string, len(r.Settings))
		for k, v := range r.Settings {
			m[k] = v
		}
		r.Settings = m
	}
	return r
}
func (e *Engine) roomNotificationsReadyLocked(room *world.Room, actor string) error {
	if room == nil {
		return nil
	}
	for _, m := range room.Members {
		peer := e.sessions[m.Connection]
		sub := e.roomSubscriptions[m.Connection]
		if m.Connection != actor && peer != nil && !peer.InScenario && sub != nil && len(sub.events) >= roomEventLimit {
			return ErrRoomBackpressure
		}
	}
	return nil
}
func (e *Engine) discardRoomEventsLocked(id string) {
	if s := e.roomSubscriptions[id]; s != nil {
		for {
			select {
			case <-s.events:
			default:
				return
			}
		}
	}
}
func (e *Engine) emitRoomEventLocked(kind RoomEventKind, actor *Session, room *world.Room, count int, disbanded bool) {
	if room == nil {
		return
	}
	for _, m := range room.Members {
		peer := e.sessions[m.Connection]
		sub := e.roomSubscriptions[m.Connection]
		if (m.Connection == actor.ID && kind != RoomWaitingReady) || peer == nil || peer.InScenario || sub == nil {
			continue
		}
		key := kddiBrowseKey{peer.Game, uint16(peer.NativeLobbyID), peer.NativeRoomID}
		slot := e.kddiSlots[key][room.ID]
		event := RoomEvent{Kind: kind, Actor: cloneSession(*actor), Room: copyEventRoom(*room), Count: count, Disbanded: disbanded, NativeSlot: slot}
		// Normal events preflight at 64; reserved space admits all departures from
		// the measured four-player room even while its recipient is congested.
		select {
		case sub.events <- event:
		default:
			sub.overflow.Store(true)
			delete(e.roomSubscriptions, m.Connection)
			close(sub.events)
		}
	}
}
