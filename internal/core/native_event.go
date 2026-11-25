package core

// RelayKDDIEvent forwards a native EVENTDAT (0x670d) between two original
// Japanese clients of the same waiting room. The event bytes are opaque and
// relayed verbatim as EVENTDATBC (0x670e), as the reference KDDI server does.
// Cross-family greetings keep their dedicated measured paths.
func (e *Engine) RelayKDDIEvent(senderID, handle string, event []byte) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	sender := e.sessions[senderID]
	if sender == nil || sender.Protocol != KDDI || sender.InScenario || sender.RoomID == "" || len(handle) != 6 {
		return ErrSession
	}
	room, ok := e.worlds.GetRoom(sender.RoomID)
	if !ok || room.Playing {
		return ErrSession
	}
	member := false
	for _, m := range room.Members {
		if m.Connection == senderID {
			member = true
		}
	}
	if !member {
		return ErrSession
	}
	for _, m := range room.Members {
		peer := e.sessions[m.Connection]
		if peer == nil || peer.ID == senderID || peer.Protocol != KDDI || peer.InScenario ||
			peer.RoomID != room.ID || peer.Account.Handle != handle {
			continue
		}
		if !e.launchEventReadyLocked(peer.ID) {
			return ErrRoomBackpressure
		}
		sub := e.roomSubscriptions[peer.ID]
		sub.events <- RoomEvent{Kind: RoomNativeEvent, Actor: cloneSession(*sender), Room: copyEventRoom(*room),
			Count: len(room.Members), Journal: append([]byte(nil), event...)}
		return nil
	}
	return ErrSession
}

// RelayKDDIAftergameEvent forwards a native EVENTDAT between two original
// Japanese clients that both returned from the same round, verbatim, as the
// reference server does (File 1 JAP+JAP after-game: 32 records of 90 bytes).
// Cross-family journals keep their dedicated path (ReceiveAftergameJournal).
func (e *Engine) RelayKDDIAftergameEvent(senderID, handle string, event []byte) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	sender := e.sessions[senderID]
	round := e.sessionRounds[senderID]
	if sender == nil || round == nil || sender.Protocol != KDDI || sender.InScenario || sender.RoomID != round.roomID ||
		!round.entered[senderID] || !round.returned[senderID] || len(handle) != 6 {
		return ErrSession
	}
	for id := range round.roster {
		peer := e.sessions[id]
		if peer == nil || peer.ID == senderID || peer.Protocol != KDDI || peer.InScenario || peer.RoomID != round.roomID ||
			e.sessionRounds[id] != round || !round.entered[id] || !round.returned[id] || peer.Account.Handle != handle {
			continue
		}
		if frozen := round.roster[id]; frozen.Account.UserID != peer.Account.UserID {
			return ErrSession
		}
		if !e.launchEventReadyLocked(peer.ID) {
			return ErrRoomBackpressure
		}
		room, ok := e.worlds.GetRoom(round.roomID)
		if !ok {
			return ErrSession
		}
		e.roomSubscriptions[peer.ID].events <- RoomEvent{Kind: RoomNativeEvent, Actor: cloneSession(*sender), Room: copyEventRoom(*room),
			Count: len(room.Members), Journal: append([]byte(nil), event...)}
		return nil
	}
	return ErrSession
}
