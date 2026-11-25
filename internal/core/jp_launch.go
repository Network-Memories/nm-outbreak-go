package core

import (
	"bytes"
	"sort"

	"networkmemories/nm-outbreak-go/internal/modules/file1"
	"networkmemories/nm-outbreak-go/internal/modules/file2"
	"networkmemories/nm-outbreak-go/internal/world"
)

// JapaneseLaunchStart freezes the actual seats before any client can enter
// gameplay. Western guests receive the table only after their own 04 query;
// Japanese guests share the host launch and query it natively (6911..6917).
func (e *Engine) JapaneseLaunchStart(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	host := e.sessions[id]
	if host == nil || host.Protocol != KDDI || (host.Game != world.GameFile1 && host.Game != world.GameFile2) || host.InScenario || !e.kddiLobbyAttached[id] {
		return ErrSession
	}
	room, ok := e.worlds.GetRoom(host.RoomID)
	limit := e.modules[host.Game].MixedEnginePeers
	if !ok || room.HostConn != id || room.Playing || len(room.Members) < 2 || len(room.Members) > limit || e.roomRounds[room.ID] != nil {
		return ErrSession
	}
	members := append([]world.Member(nil), room.Members...)
	sort.Slice(members, func(i, j int) bool { return members[i].Seat < members[j].Seat })
	if members[0].Connection != id || host.WireID == 0 || !e.launchEventReadyLocked(id) {
		return ErrSession
	}
	var profiles [4][240]byte
	var guests []*Session
	pending := map[string]bool{}
	western := 0
	for i, m := range members[1:] {
		guest := e.sessions[m.Connection]
		if guest == nil || guest.InScenario || guest.RoomID != room.ID || !e.launchEventReadyLocked(guest.ID) {
			return ErrSession
		}
		switch guest.Protocol {
		case SNAP:
			state := e.waiting[guest.ID]
			if state.roomID != room.ID || state.hostID != id || guest.WireID == 0 || len(guest.Profile) != 240 {
				return ErrSession
			}
			copy(profiles[i+1][:], guest.Profile)
			pending[guest.ID] = true
			western++
		case KDDI:
			// A Japanese guest property is projected exactly as the Western
			// room roster does (208 actual bytes, 32 zero host-only bytes).
			if len(guest.Profile) != 208 || !e.kddiLobbyAttached[guest.ID] {
				return ErrSession
			}
			copy(profiles[i+1][:], guest.Profile)
		default:
			return ErrSession
		}
		guests = append(guests, guest)
	}
	if western == 0 {
		// All-Japanese rooms keep the native relay; only mixed rounds launch here.
		return ErrSession
	}
	var (
		profile []byte
		rules   []byte
		err     error
	)
	if host.Game == world.GameFile1 {
		profile, err = file1.HostPropertyToUS(host.Profile, room.Area, room.RuleValues, room.Difficulty)
		if err != nil {
			return err
		}
		rules, err = file1.KDDILaunchRules(room.RuleValues, room.Difficulty)
		if err != nil {
			return err
		}
	} else {
		profile, err = file2.HostPropertyToWestern(host.Profile, room.Area, room.RuleValues, room.Difficulty)
		if err != nil {
			return err
		}
		rules, err = file2.LaunchRules(room.Area, room.StatWord, profile)
		if err != nil {
			return err
		}
	}
	round, err := e.scenarioRoundLocked(room)
	if err != nil {
		return err
	}
	details := LaunchDetails{RoundID: round.id, NativeNumber: round.nativeNumber, Number: 0, Count: len(members), Rules: rules, Room: copyEventRoom(*room)}
	copy(details.Profiles[0][:], profile)
	for i := 1; i < len(members); i++ {
		details.Profiles[i] = profiles[i]
	}
	if e.launches == nil {
		e.launches = make(map[string]*nativeLaunch)
	}
	l := &nativeLaunch{round: round, ready: make(chan struct{}), japanese: &details, pending: pending}
	e.launches[id] = l
	for _, guest := range guests {
		if guest.Protocol == KDDI {
			e.launches[guest.ID] = l
		}
		e.launchEventLocked(RoomLaunchAnnounced, host, guest, room)
	}
	e.launchEventLocked(RoomLaunchAnnounced, host, host, room)
	return nil
}

// launchNumber is the frozen seat order of a participant in a launch.
func launchNumber(d *LaunchDetails, id string) int {
	members := append([]world.Member(nil), d.Room.Members...)
	sort.Slice(members, func(i, j int) bool { return members[i].Seat < members[j].Seat })
	for i, m := range members {
		if m.Connection == id {
			return i
		}
	}
	return -1
}

func (e *Engine) JapaneseLaunchGuestTable(id string, target uint32, body []byte) (LaunchDetails, bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	guest := e.sessions[id]
	if guest == nil || guest.Protocol != SNAP {
		return LaunchDetails{}, false, nil
	}
	room, ok := e.worlds.GetRoom(guest.RoomID)
	if !ok {
		return LaunchDetails{}, false, nil
	}
	host := e.sessions[room.HostConn]
	if host == nil || host.Protocol != KDDI {
		return LaunchDetails{}, false, nil
	}
	l := e.launches[host.ID]
	if l == nil || l.japanese == nil || l.closed || l.round.roomID != room.ID || host.WireID != target ||
		guest.InScenario || len(body) != 768 || !bytes.Equal(body, make([]byte, 768)) || !l.pending[id] {
		return LaunchDetails{}, true, ErrSession
	}
	if _, ok := l.round.roster[id]; !ok || id == host.ID {
		return LaunchDetails{}, true, ErrSession
	}
	result := *l.japanese
	result.Number = launchNumber(l.japanese, id)
	if result.Number < 1 {
		return LaunchDetails{}, true, ErrSession
	}
	result.Room = copyEventRoom(result.Room)
	return result, true, nil
}

func (e *Engine) CompleteJapaneseLaunchGuest(id, roundID string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	guest := e.sessions[id]
	if guest == nil {
		return
	}
	for _, l := range e.launches {
		if l.japanese != nil && !l.closed && l.round.id == roundID && l.round.roomID == guest.RoomID {
			if _, ok := l.round.roster[id]; !ok || l.round.room.HostConn == id || !l.pending[id] {
				return
			}
			delete(l.pending, id)
			if len(l.pending) == 0 {
				l.closed = true
				close(l.ready)
			}
			return
		}
	}
}
