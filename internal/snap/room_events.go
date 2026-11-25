package snap

import (
	"encoding/binary"
	"errors"
	"log"
	"net"
	"networkmemories/nm-outbreak-go/internal/core"
	"networkmemories/nm-outbreak-go/internal/modules/file1"
	"networkmemories/nm-outbreak-go/internal/modules/file2"
	"networkmemories/nm-outbreak-go/internal/world"
)

func westernProfile(profile []byte, room world.Room, actor string) ([]byte, error) {
	p := append([]byte(nil), profile...)
	if len(p) == 208 {
		if actor == room.HostConn {
			if room.Game == world.GameFile1 {
				return file1.HostPropertyToUS(p, room.Area, room.RuleValues, room.Difficulty)
			}
			return file2.HostPropertyToWestern(p, room.Area, room.RuleValues, room.Difficulty)
		}
		p = append(p, make([]byte, 32)...)
	}
	if len(p) != 240 {
		return nil, errors.New("native profile unavailable")
	}
	return p, nil
}

func (s *Server) watchRoom(pc net.PacketConn, id string) {
	s.notifyMu.Lock()
	if s.notifications == nil {
		s.notifications = make(map[string]*core.RoomSubscription)
	}
	if s.notifications[id] != nil {
		s.notifyMu.Unlock()
		return
	}
	sub, err := s.engine.SubscribeRoom(id)
	if err != nil {
		s.notifyMu.Unlock()
		return
	}
	s.notifications[id] = sub
	s.notifyMu.Unlock()
	go func() {
		defer func() {
			s.engine.UnsubscribeRoom(id, sub)
			s.notifyMu.Lock()
			if s.notifications[id] == sub {
				delete(s.notifications, id)
			}
			s.notifyMu.Unlock()
		}()
		for {
			s.engine.FlushAftergameJournals(id)
			var event core.RoomEvent
			select {
			case <-s.notifyDone:
				return
			case next, ok := <-sub.Events():
				if !ok {
					if sub.Overflowed() {
						log.Printf("[nm.snap.%s] native room notifications congested", s.name)
					}
					return
				}
				event = next
			}
			peer, ok := s.engine.Session(id)
			if !ok || peer.InScenario || peer.Game != event.Room.Game {
				continue
			}
			if peer.RoomID != event.Room.ID && !(event.Disbanded && peer.RoomID == "") {
				continue
			}
			b, ok := s.bindings.FindCore(id)
			if !ok || b.Addr == nil {
				continue
			}
			if event.Kind == core.HallPopulationChanged {
				if !s.engine.HallPopulationDeliverable(id, event) {
					continue
				}
				cmd := CmdJoin
				if event.Count < 0 {
					cmd = CmdLeave
				}
				// File2 area callbacks consume the count delta and do not read this
				// payload. Zero denotes no SNAP actor ID; do not allocate a fake peer.
				payload := make([]byte, 4)
				binary.BigEndian.PutUint32(payload, event.Actor.WireID)
				if err := s.sendReply(pc, b.Addr, Message{TypeFlags: FlagReliable | FlagChannelBits | FlagResponse, Command: cmd, SessionID: b.ID, RouteSessionID: b.ID, AckNumber: s.reliable.lastACK(b.Addr, b.ID), Payload: payload}); err != nil {
					return
				}
				continue
			}
			if event.Kind == core.RoomChat && event.ChatArea != 0 {
				if peer.Game != world.GameFile2 || !s.engine.RoomChatDeliverable(id, event) {
					continue
				}
				body, err := file2.EncodeWesternAreaChat(event.Actor.Account.NativeAlias, event.ChatText)
				if err != nil {
					log.Printf("[nm.snap.%s] native area chat cannot fit recipient fields", s.name)
					continue
				}
				// Historical native area delivery uses the neutral source0;
				// identity remains the server-verified name inside the envelope.
				if err = s.sendReply(pc, b.Addr, Message{TypeFlags: FlagReliable | FlagResponse | 0x1000 | 0x0400, Command: CmdSend, SessionID: 0, RouteSessionID: b.ID, AckNumber: s.reliable.lastACK(b.Addr, b.ID), Payload: body}); err != nil {
					log.Printf("[nm.snap.%s] native area chat delivery failed: %v", s.name, err)
					return
				}
				continue
			}
			actor := event.Actor
			if actor.WireID == 0 {
				log.Printf("[nm.snap.%s] native room event lacks proved routing ID", s.name)
				continue
			}
			flags := uint16(FlagRoom | FlagResponse)
			command := uint8(0)
			var payload []byte
			if event.Kind == core.RoomChat {
				if peer.Game != world.GameFile2 || !s.engine.RoomChatDeliverable(id, event) {
					continue
				}
				body, err := file2.EncodeWesternChat(actor.Account.NativeAlias, event.ChatText, event.ChatAftergame)
				if err != nil {
					log.Printf("[nm.snap.%s] native chat cannot fit recipient fields", s.name)
					continue
				}
				if err = s.sendReply(pc, b.Addr, Message{TypeFlags: FlagReliable | FlagRoom | FlagResponse | 0x0400, Command: CmdSend, SessionID: actor.WireID, RouteSessionID: b.ID, AckNumber: s.reliable.lastACK(b.Addr, b.ID), Payload: body}); err != nil {
					log.Printf("[nm.snap.%s] native room chat delivery failed: %v", s.name, err)
					return
				}
				continue
			}
			if event.Kind == core.RoomAftergameJournal {
				if !s.engine.AftergameJournalSupported(peer.Game) {
					continue
				}
				blocks, err := file2.JournalBlocks(event.Journal, false)
				if err != nil {
					continue
				}
				for _, payload := range blocks {
					if err = s.sendReply(pc, b.Addr, Message{TypeFlags: FlagReliable | FlagRoom, Command: CmdSendTarget, SessionID: actor.WireID, RouteSessionID: b.ID, AckNumber: s.reliable.lastACK(b.Addr, b.ID), Payload: payload}); err != nil {
						log.Printf("[nm.snap.%s] aftergame journal delivery failed: %v", s.name, err)
						return
					}
				}
				continue
			}
			switch event.Kind {
			case core.RoomLaunchAnnounced:
				if err := s.sendJapaneseLaunchPart(pc, b.Addr, b.ID, actor.WireID, 3, nil); err != nil {
					return
				}
				continue
			case core.RoomWaitingGreeting:
				payload = make([]byte, 20)
				binary.BigEndian.PutUint32(payload, 1)
				binary.LittleEndian.PutUint32(payload[8:], 1)
				if err := s.sendReply(pc, b.Addr, Message{TypeFlags: FlagReliable | FlagRoom, Command: CmdSendTarget, SessionID: actor.WireID, RouteSessionID: b.ID, AckNumber: s.reliable.lastACK(b.Addr, b.ID), Payload: payload}); err != nil {
					return
				}
				continue
			case core.RoomLaunchAcknowledged:
				payload = make([]byte, 12+768)
				binary.BigEndian.PutUint32(payload, 1)
				binary.LittleEndian.PutUint32(payload[8:], 4)
				if err := s.sendReply(pc, b.Addr, Message{TypeFlags: FlagReliable | FlagRoom, Command: CmdSendTarget, SessionID: actor.WireID, RouteSessionID: b.ID, AckNumber: s.reliable.lastACK(b.Addr, b.ID), Payload: payload}); err != nil {
					log.Printf("[nm.snap.%s] mixed launch ACK failed: %v", s.name, err)
					return
				}
				continue
			case core.RoomJoined:
				p, err := westernProfile(actor.Profile, event.Room, actor.ID)
				if err != nil {
					continue
				}
				command = CmdJoin
				flags |= FlagReliable
				payload = BuildMemberRecord(actor.Account.NativeAlias, actor.WireID, p)
			case core.RoomProfileChanged, core.RoomAftergameReturned:
				if event.Kind == core.RoomAftergameReturned && actor.Protocol != core.KDDI {
					continue
				}
				p, err := westernProfile(actor.Profile, event.Room, actor.ID)
				if err != nil {
					continue
				}
				p, err = s.engine.ProjectSNAPAftergameProfile(actor.ID, p)
				if err != nil {
					continue
				}
				command = CmdChangeUserProperty
				payload = make([]byte, 8+len(p))
				binary.BigEndian.PutUint32(payload, actor.WireID)
				binary.BigEndian.PutUint32(payload[4:], uint32(len(p)))
				copy(payload[8:], p)
			case core.RoomLeft:
				command = CmdLeave
				flags |= FlagReliable
				payload = make([]byte, 4)
				binary.BigEndian.PutUint32(payload, actor.WireID)
			default:
				continue
			}
			if err := s.sendReply(pc, b.Addr, Message{TypeFlags: flags, Command: command, SessionID: b.ID, AckNumber: s.reliable.lastACK(b.Addr, b.ID), Payload: payload}); err != nil {
				log.Printf("[nm.snap.%s] native room notification failed: %v", s.name, err)
				return
			}
		}
	}()
}
