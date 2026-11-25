package kddi

import (
	"bytes"
	"log"
	"net"
	"networkmemories/nm-outbreak-go/internal/modules/file2"
)

func (s *Server) handleJournal(c net.Conn, f LobbyFrame, id string) bool {
	if f.Command != 0x670d {
		return false
	}
	response := LobbyFrame{Sender: SenderServer, Kind: KindTell, Command: f.Command, PacketID: f.PacketID, Error: 0xff, Payload: nativeRefusal()}
	defer func() { sendFrame(c, response) }()
	if f.Kind != 1 {
		return true
	}
	handle, n, err := decodeNativeField(f.Payload, f.PacketID)
	if err != nil || len(handle) != 6 {
		return true
	}
	event, used, err := decodeNativeField(f.Payload[n:], f.PacketID)
	if err != nil || n+used != len(f.Payload) {
		return true
	}
	accept := func() {
		response.Error = 0
		response.Payload = append(word(6), handle...)
	}
	// Waiting room only: after-game journals keep their measured File2 path.
	if _, notAftergame := s.engine.AftergameMembers(id); notAftergame != nil &&
		s.engine.RelayKDDIEvent(id, string(handle), event) == nil {
		accept()
		return true
	}
	if bytes.Equal(event, make([]byte, 8)) {
		if s.engine.SignalKDDIWaitingGreeting(id, string(handle)) == nil {
			accept()
		} else {
			log.Printf("[nm.kddi.%s] EVENTDAT refused: target=%s event=%x", s.name, handle, event)
		}
		return true
	}
	// Same-family after-game events are relayed verbatim (reference server).
	if s.engine.RelayKDDIAftergameEvent(id, string(handle), event) == nil {
		accept()
		return true
	}
	if !s.engine.AftergameJournalSupported(s.game) {
		log.Printf("[nm.kddi.%s] EVENTDAT refused: target=%s len=%d", s.name, handle, len(event))
		return true
	}
	more, chunk, err := file2.JournalChunk(event, true)
	if err != nil {
		return true
	}
	members, err := s.engine.AftergameMembers(id)
	if err != nil {
		return true
	}
	for _, peer := range members {
		if peer.ID == id || peer.Account.Handle != string(handle) {
			continue
		}
		if err = s.engine.ReceiveAftergameJournal(id, peer.ID, more, chunk); err != nil {
			return true
		}
		response.Error = 0
		response.Payload = append(word(6), handle...)
		return true
	}
	return true
}
