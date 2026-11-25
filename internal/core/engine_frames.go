package core

import (
	"errors"
	"networkmemories/nm-outbreak-go/internal/protocol/enginewire"
	"sort"
)

var ErrEngineBackpressure = errors.New("native engine destination congested")
var ErrEngineAttached = errors.New("native engine transport already attached")

const engineQueueSize = 256

type EnginePeer struct {
	ID         string
	Protocol   Protocol
	InScenario bool
}

type EngineContext struct {
	RoundID      string
	Index, Count int
	Peers        []EnginePeer
}
type EngineDelivery struct {
	RoundID string
	FromID  string
	WireID  uint32
	Frame   enginewire.Frame
	// Raw is an actual native Japanese game record relayed verbatim to another
	// Japanese member of the same mixed round (the reference server broadcasts
	// every record unchanged). Frame is unset when Raw is present.
	Raw []byte
}
type engineMailbox struct {
	events chan EngineDelivery
	lease  *EngineAttachment
	closed bool
}
type EngineAttachment struct {
	Context EngineContext
	mailbox *engineMailbox
	id      string
}

func (a *EngineAttachment) Events() <-chan EngineDelivery { return a.mailbox.events }

// RoomMixed identifies real opposite-family members; it never counts an IP or
// manufactures a peer to permit fallback into an unqualified raw relay.
func (e *Engine) RoomMixed(id string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	s := e.sessions[id]
	if s == nil {
		return false
	}
	room, ok := e.worlds.GetRoom(s.RoomID)
	if !ok {
		return false
	}
	for _, m := range room.Members {
		if peer := e.sessions[m.Connection]; peer != nil && peer.Protocol != s.Protocol {
			return true
		}
	}
	return false
}
func (e *Engine) engineContextLocked(id string) (EngineContext, *scenarioRound, error) {
	s := e.sessions[id]
	round := e.sessionRounds[id]
	if s == nil || !s.InScenario || round == nil || s.RoomID != round.roomID || e.roomRounds[round.roomID] != round {
		return EngineContext{}, nil, ErrSession
	}
	if limit := e.modules[s.Game].MixedEnginePeers; limit < 2 || len(round.roster) > limit {
		return EngineContext{}, nil, ErrSession
	}
	members := make([]EnginePeer, 0, len(round.room.Members))
	// Frozen seats map native engine indexes; current sessions supply presence.
	order := make([]int, len(round.room.Members))
	for i := range order {
		order[i] = i
	}
	sort.Slice(order, func(i, j int) bool { return round.room.Members[order[i]].Seat < round.room.Members[order[j]].Seat })
	index := -1
	mixed := false
	for i, sourceIndex := range order {
		m := round.room.Members[sourceIndex]
		peer := e.sessions[m.Connection]
		if peer == nil || peer.RoomID != round.roomID {
			return EngineContext{}, nil, ErrSession
		}
		if peer.ID == id {
			index = i
		}
		if peer.Protocol != s.Protocol {
			mixed = true
		}
		if peer.Protocol == KDDI {
			l := e.launches[peer.ID]
			if !l.complete() || l.round != round {
				return EngineContext{}, nil, ErrSession
			}
		}
		members = append(members, EnginePeer{peer.ID, peer.Protocol, peer.InScenario})
	}
	if index < 0 || !mixed {
		return EngineContext{}, nil, ErrSession
	}
	return EngineContext{round.id, index, len(members), members}, round, nil
}
func (e *Engine) MixedEngineContext(id string) (EngineContext, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	ctx, _, err := e.engineContextLocked(id)
	return ctx, err
}
func engineMailboxFor(round *scenarioRound, id string) *engineMailbox {
	if round.engineMailboxes == nil {
		round.engineMailboxes = make(map[string]*engineMailbox)
	}
	m := round.engineMailboxes[id]
	if m == nil {
		m = &engineMailbox{events: make(chan EngineDelivery, engineQueueSize)}
		round.engineMailboxes[id] = m
	}
	return m
}
func (e *Engine) AttachEngine(id string) (*EngineAttachment, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	ctx, round, err := e.engineContextLocked(id)
	if err != nil {
		return nil, err
	}
	box := engineMailboxFor(round, id)
	if box.closed {
		return nil, ErrSession
	}
	if box.lease != nil {
		return nil, ErrEngineAttached
	}
	a := &EngineAttachment{ctx, box, id}
	box.lease = a
	return a, nil
}
func (e *Engine) DetachEngine(a *EngineAttachment) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if a.mailbox.lease == a {
		a.mailbox.lease = nil
	}
}
func (e *Engine) EngineAttachmentActive(a *EngineAttachment) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	ctx, _, err := e.engineContextLocked(a.id)
	return err == nil && ctx.RoundID == a.Context.RoundID && a.mailbox.lease == a && !a.mailbox.closed
}

// PublishEngine validates the whole batch before committing any frame. Expected
// peers have bounded mailboxes before their game socket attaches, preserving the
// actual early engine data instead of silently dropping it during handoff.
func (e *Engine) PublishEngine(id, roundID string, frames []enginewire.Frame) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	ctx, round, err := e.engineContextLocked(id)
	if err != nil || ctx.RoundID != roundID {
		return ErrSession
	}
	if len(frames) > engineQueueSize {
		return errors.New("native engine batch exceeds bounded mailbox")
	}
	sender := e.sessions[id]
	for _, f := range frames {
		if f.Peer != ctx.Index || len(f.Body) > 251 {
			return errors.New("engine sender differs from proved native place")
		}
	}
	var targets []*engineMailbox
	for _, peer := range ctx.Peers {
		if peer.ID == id || peer.Protocol == sender.Protocol || round.returned[peer.ID] {
			continue
		}
		box := engineMailboxFor(round, peer.ID)
		if box.closed || len(frames) > cap(box.events)-len(box.events) {
			return ErrEngineBackpressure
		}
		targets = append(targets, box)
	}
	for _, box := range targets {
		for _, f := range frames {
			copyFrame := enginewire.Frame{Peer: f.Peer, Body: append([]byte(nil), f.Body...)}
			box.events <- EngineDelivery{RoundID: roundID, FromID: sender.ID, WireID: sender.WireID, Frame: copyFrame}
		}
	}
	return nil
}

// PublishKDDIRaw relays one actual Japanese record unchanged to the other
// Japanese members of the sender's mixed round, as the reference server does.
func (e *Engine) PublishKDDIRaw(id, roundID string, record []byte) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	ctx, round, err := e.engineContextLocked(id)
	if err != nil || ctx.RoundID != roundID {
		return ErrSession
	}
	sender := e.sessions[id]
	if sender.Protocol != KDDI || len(record) < 2 || int(record[0]) != len(record) || int(record[1]&15) != ctx.Index {
		return errors.New("native record differs from proved sender place")
	}
	var targets []*engineMailbox
	for _, peer := range ctx.Peers {
		if peer.ID == id || peer.Protocol != KDDI || round.returned[peer.ID] {
			continue
		}
		box := engineMailboxFor(round, peer.ID)
		if box.closed || len(box.events) == cap(box.events) {
			return ErrEngineBackpressure
		}
		targets = append(targets, box)
	}
	for _, box := range targets {
		box.events <- EngineDelivery{RoundID: roundID, FromID: sender.ID, WireID: sender.WireID, Raw: append([]byte(nil), record...)}
	}
	return nil
}
func (e *Engine) closeEngineRoundLocked(round *scenarioRound) {
	for _, box := range round.engineMailboxes {
		if !box.closed {
			close(box.events)
			box.closed = true
		}
	}
}
