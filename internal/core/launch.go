package core

import (
	"bytes"
	"errors"
	"networkmemories/nm-outbreak-go/internal/protocol/snaplaunch"
	"networkmemories/nm-outbreak-go/internal/world"
	"sort"
)

type nativeLaunch struct {
	japanese  *LaunchDetails
	handshake snaplaunch.Guest
	round     *scenarioRound
	ready     chan struct{}
	closed    bool
	// pending holds the Western guests of a Japanese-hosted launch that have
	// not received their table yet; Japanese guests share the same launch.
	pending map[string]bool
}

// complete reports readiness regardless of which native client hosted.
func (l *nativeLaunch) complete() bool {
	return l != nil && l.closed && (l.japanese != nil || l.handshake.State == "complete")
}

type LaunchDetails struct {
	RoundID, NativeNumber string
	Number, Count         int
	Rules                 []byte
	Profiles              [4][240]byte
	Room                  world.Room
}

func (e *Engine) launchEventReadyLocked(id string) bool {
	sub := e.roomSubscriptions[id]
	return sub != nil && len(sub.events) < roomEventLimit
}
func (e *Engine) launchEventLocked(kind RoomEventKind, actor, recipient *Session, room *world.Room) {
	sub := e.roomSubscriptions[recipient.ID]
	key := kddiBrowseKey{recipient.Game, uint16(recipient.NativeLobbyID), recipient.NativeRoomID}
	sub.events <- RoomEvent{Kind: kind, Actor: cloneSession(*actor), Room: copyEventRoom(*room), Count: len(room.Members), NativeSlot: e.kddiSlots[key][room.ID]}
}

// ReceiveWesternLaunch is called only for a targeted File2 native control
// message. Regular same-family forwarding stays in the SNAP transport.
func (e *Engine) ReceiveWesternLaunch(hostID string, m snaplaunch.Message) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	host := e.sessions[hostID]
	if host == nil || host.Protocol != SNAP || host.WireID == 0 || e.modules[host.Game].SNAPLaunchRules == nil {
		return ErrSession
	}
	room, ok := e.worlds.GetRoom(host.RoomID)
	if !ok || room.HostConn != hostID || host.InScenario {
		return ErrSession
	}
	var guest *Session
	for _, member := range room.Members {
		s := e.sessions[member.Connection]
		if s != nil && s.Protocol == KDDI && s.WireID == m.Target {
			guest = s
			break
		}
	}
	if guest == nil || guest.InScenario {
		return ErrSession
	}
	l := e.launches[guest.ID]
	if m.Tag == 3 {
		if l != nil {
			return snaplaunch.ErrSequence
		}
		if !e.launchEventReadyLocked(guest.ID) {
			return ErrRoomBackpressure
		}
		if len(room.Members) < 2 {
			return ErrSession
		}
		if _, err := e.modules[host.Game].SNAPLaunchRules(room.Area, room.StatWord, host.Profile); err != nil {
			return err
		}
		handshake := snaplaunch.New(m.Target)
		if err := handshake.Feed(m); err != nil {
			return err
		}
		round, err := e.scenarioRoundLocked(room)
		if err != nil {
			return err
		}
		if e.launches == nil {
			e.launches = make(map[string]*nativeLaunch)
		}
		l = &nativeLaunch{handshake: handshake, round: round, ready: make(chan struct{})}
		e.launches[guest.ID] = l
		e.launchEventLocked(RoomLaunchAnnounced, host, guest, room)
		return nil
	}
	if l == nil {
		return ErrSession
	}
	// Validate a copy: malformed fragments cannot leave partially accepted state.
	next := l.handshake
	if err := next.Feed(m); err != nil {
		return err
	}
	members := append([]world.Member(nil), l.round.room.Members...)
	sort.Slice(members, func(i, j int) bool { return members[i].Seat < members[j].Seat })
	if next.Count > 0 {
		number := -1
		for i, member := range members {
			if member.Connection == guest.ID {
				number = i
			}
		}
		if next.Count != len(members) || next.Number != number {
			return errors.New("native launch differs from proved roster")
		}
	}
	if m.Tag == 6 {
		name := bytes.TrimRight(next.Names[int(m.Body[0])][:], "\x00")
		if string(name) != members[int(m.Body[0])].WesternName {
			return errors.New("native launch changed proved owner name")
		}
	}
	if next.State == "complete" {
		_, err := e.modules[host.Game].SNAPLaunchRules(l.round.room.Area, l.round.room.StatWord, next.Profiles[0][:])
		if err != nil {
			return err
		}
	}
	l.handshake = next
	if next.State == "complete" {
		close(l.ready)
		l.closed = true
	}
	return nil
}
func (e *Engine) HasNativeLaunch(id string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.launches[id] != nil
}

// AcknowledgeNativeLaunch is proof of the actual Japanese client's first launch
// query after6910. Membership alone never produces the Western04.
func (e *Engine) AcknowledgeNativeLaunch(id string) (<-chan struct{}, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	guest := e.sessions[id]
	l := e.launches[id]
	if guest == nil || guest.Protocol != KDDI || l == nil {
		return nil, ErrSession
	}
	host := e.sessions[l.round.room.HostConn]
	if host == nil {
		return nil, ErrSession
	}
	if l.japanese != nil {
		return l.ready, nil
	}
	if l.handshake.State == "announced" {
		if !e.launchEventReadyLocked(host.ID) {
			return nil, ErrRoomBackpressure
		}
		if err := l.handshake.Acknowledge(); err != nil {
			return nil, err
		}
		e.launchEventLocked(RoomLaunchAcknowledged, guest, host, &l.round.room)
	}
	return l.ready, nil
}
func (e *Engine) NativeLaunch(id string) (LaunchDetails, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	l := e.launches[id]
	s := e.sessions[id]
	if l != nil && l.japanese != nil && s != nil && l.closed && s.RoomID == l.round.roomID {
		result := *l.japanese
		result.Number = launchNumber(l.japanese, id)
		if result.Number < 0 {
			return LaunchDetails{}, ErrSession
		}
		result.Rules = append([]byte(nil), result.Rules...)
		result.Room = copyEventRoom(result.Room)
		return result, nil
	}
	if l == nil || s == nil || l.handshake.State != "complete" || s.RoomID != l.round.roomID {
		return LaunchDetails{}, ErrSession
	}
	rules, err := e.modules[s.Game].SNAPLaunchRules(l.round.room.Area, l.round.room.StatWord, l.handshake.Profiles[0][:])
	if err != nil {
		return LaunchDetails{}, err
	}
	return LaunchDetails{l.round.id, l.round.nativeNumber, l.handshake.Number, l.handshake.Count, rules, l.handshake.Profiles, copyEventRoom(l.round.room)}, nil
}
func (e *Engine) cancelLaunchesLocked(roomID string) {
	for id, l := range e.launches {
		if l.round.roomID == roomID {
			// Japanese-hosted launches are shared by several seats.
			if !l.closed {
				close(l.ready)
				l.closed = true
			}
			delete(e.launches, id)
		}
	}
}
