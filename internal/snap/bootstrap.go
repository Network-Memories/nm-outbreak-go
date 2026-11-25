package snap

import (
	"encoding/binary"
	"log"
	"net"
	"strings"
)

func (s *Server) handleBootstrap(pc net.PacketConn, remote net.Addr, msg Message) {
	if remote == nil {
		return
	}
	fail := func() {
		flags := FlagChannelBits
		var payload []byte
		if msg.Command == 0x41 {
			flags |= FlagResponse
			payload = make([]byte, 8)
			binary.BigEndian.PutUint32(payload[4:], 0x13)
		}
		if msg.TypeFlags&FlagReliable != 0 {
			flags |= FlagReliable
		}
		sendMsg(pc, remote, Message{TypeFlags: flags, Sequence: msg.Sequence, Command: 0x2e, SessionID: msg.SessionID, AckNumber: msg.PacketSeq, Payload: payload})
	}

	switch msg.Command {
	case 0x2c:
		if len(msg.Payload) < 104 {
			return
		}
		login := readCString(msg.Payload[:40])
		if len(login) < 1 || len(login) > 15 {
			fail()
			return
		}
		for _, c := range login {
			if !strings.ContainsRune("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_", c) {
				fail()
				return
			}
		}
		title := binary.BigEndian.Uint32(msg.Payload[100:104])
		id, payload, err := s.engine.BeginSNAP(remote.String(), title, login, msg.Payload)
		if err != nil {
			log.Printf("[nm.snap.bootstrap] login challenge refused: %v", err)
			fail()
			return
		}
		sendMsg(pc, remote, Message{TypeFlags: FlagChannelBits, Sequence: msg.Sequence, Command: 0x40, SessionID: id, PacketSeq: 1, AckNumber: msg.PacketSeq, Payload: payload})
	case 0x41:
		handoff, err := s.engine.FinishSNAP(remote.String(), msg.Payload)
		if err != nil {
			log.Printf("[nm.snap.bootstrap] login answer refused: %v", err)
			fail()
			return
		}
		flags := FlagChannelBits | FlagResponse
		sequence := uint32(2)
		lastUnreliable := uint32(2)
		if msg.TypeFlags&FlagReliable != 0 && msg.SessionID == handoff.WireID {
			flags |= FlagReliable
			sequence = 0
			lastUnreliable = 1
		}
		s.engine.SetSNAPSequence(handoff.WireID, lastUnreliable)
		sendMsg(pc, remote, Message{TypeFlags: flags, Sequence: msg.Sequence, Command: 0x2d, SessionID: handoff.WireID, PacketSeq: sequence, AckNumber: msg.PacketSeq, Payload: handoff.Payload})
	}
}

func (s *Server) adoptNativeHandoff(remote net.Addr, msg Message) (Message, bool) {
	if len(msg.Payload) < 0x128 {
		return msg, false
	}

	login := strings.TrimRight(readCString(msg.Payload[16:32]), "-")
	handoff, err := s.engine.SNAPGameHandoff(s.game, msg.SessionID, remote.String(), login)
	if err != nil {
		return msg, false
	}
	if err := s.adoptSession(handoff.WireID, remote, handoff.Session.ID); err != nil {
		return msg, false
	}
	s.bindings.ClientAlias(handoff.WireID, msg.SessionID)
	for _, old := range s.bindings.SupersedeEndpoint(remote, handoff.WireID) {
		s.engine.Logout(old.CoreID)
	}
	s.bindings.InitialSequence(handoff.WireID, handoff.OutSequence)
	msg.SourceSessionID = msg.SessionID
	msg.HasSourceSession = true
	msg.SessionID = handoff.WireID
	return msg, true
}

// Game identity comes from the measured title in the bootstrap, never from a
// default PAL region or a name/host guess in a game datagram.
