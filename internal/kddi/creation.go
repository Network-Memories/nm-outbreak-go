// Staged slot configuration and rule-query layouts follow BioServer
// PacketHandler.java / RuleSet.java (AGPL-3.0).
package kddi

import (
	"encoding/binary"
	"net"
	"networkmemories/nm-outbreak-go/internal/core"
	"networkmemories/nm-outbreak-go/internal/world"
)

func (s *Server) handleCreation(c net.Conn, f LobbyFrame, id string) bool {
	switch f.Command {
	case 0x6407, 0x6509, 0x6609, 0x660a, 0x660b, 0x6504, 0x6601, 0x6602, 0x6603, 0x6604, 0x6605, 0x6606, 0x6607, 0x6608, 0x660e, 0x660c:
	default:
		return false
	}
	actor, ok := s.engine.Session(id)
	if !ok || actor.Protocol != core.KDDI || actor.Game != s.game || f.Kind != 1 {
		return true
	}
	reply := LobbyFrame{Sender: SenderServer, Kind: KindTell, Command: f.Command, PacketID: f.PacketID}
	fail := func() { reply.Error = 0xff; reply.Payload = nativeRefusal(); sendFrame(c, reply) }
	var err error
	switch f.Command {
	case 0x6407:
		if len(f.Payload) != 2 {
			fail()
			return true
		}
		slot := binary.BigEndian.Uint16(f.Payload)
		err = s.engine.BeginKDDICreation(id, slot)
		reply.Payload = word(slot)
	case 0x6509:
		if len(f.Payload) != 4 {
			fail()
			return true
		}
		media, scenario := binary.BigEndian.Uint16(f.Payload), binary.BigEndian.Uint16(f.Payload[2:])
		err = s.engine.SetKDDICreationScene(id, media, int(scenario))
		if err == nil {
			v, _ := s.engine.KDDICreationForOwner(id)
			reply.Payload = append(word(v.Slot), f.Payload...)
		}
	case 0x6609, 0x660a:
		value, n, e := decodeNativeField(f.Payload, f.PacketID)
		if e != nil || n != len(f.Payload) {
			fail()
			return true
		}
		if f.Command == 0x6609 {
			err = s.engine.SetKDDICreationTitle(id, string(value))
		} else {
			err = s.engine.SetKDDICreationPassword(id, value)
		}
	case 0x660b:
		if len(f.Payload) != 2 {
			fail()
			return true
		}
		err = s.engine.SetKDDICreationRule(id, f.Payload[0], f.Payload[1])
		reply.Payload = []byte{1}
	case 0x6504:
		if len(f.Payload) != 1 {
			fail()
			return true
		}
		if _, draft := s.engine.KDDICreationForOwner(id); draft {
			_, err = s.engine.CommitKDDICreation(id)
		} else {
			room, exists := s.engine.Worlds().GetRoom(actor.RoomID)
			member := false
			if exists {
				for _, m := range room.Members {
					if m.Connection == id {
						member = true
						break
					}
				}
			}
			if !member || actor.InScenario {
				err = core.ErrSession
			}
		}
		if err == nil && f.Payload[0] == 1 {
			err = s.engine.MarkKDDIWaitingReady(id)
		}
		reply.Payload = append([]byte(nil), f.Payload...)
	case 0x660c:
		if len(f.Payload) != 0 {
			fail()
			return true
		}
		if _, ok := s.engine.KDDICreationForOwner(id); !ok {
			fail()
			return true
		}
	default:
		need := 2
		switch f.Command {
		case 0x6604, 0x6605, 0x6606, 0x6607:
			need = 3
		case 0x6608, 0x660e:
			need = 4
		}
		if len(f.Payload) != need {
			fail()
			return true
		}
		slot := binary.BigEndian.Uint16(f.Payload)
		schema, values, e := s.engine.KDDIRules(id, slot)
		if e != nil {
			fail()
			return true
		}
		switch f.Command {
		case 0x6601, 0x6602:
			reply.Payload = append([]byte{1}, word(slot)...)
		case 0x6603:
			reply.Payload = []byte{byte(len(schema))}
		default:
			index := f.Payload[2]
			if int(index) >= len(schema) {
				fail()
				return true
			}
			rule := schema[index]
			switch f.Command {
			case 0x6604:
				reply.Payload = append([]byte{index}, word(uint16(len(rule.Name)))...)
				reply.Payload = append(reply.Payload, []byte(rule.Name)...)
			case 0x6605:
				editable := byte(0)
				if rule.Editable {
					editable = 1
				}
				reply.Payload = []byte{index, editable}
			case 0x6606:
				reply.Payload = []byte{index, values[index]}
			case 0x6607:
				reply.Payload = []byte{index, byte(len(rule.Choices))}
			case 0x6608, 0x660e:
				choice := f.Payload[3]
				if int(choice) >= len(rule.Choices) {
					fail()
					return true
				}
				reply.Payload = []byte{index, choice}
				if f.Command == 0x660e {
					reply.Payload = append(reply.Payload, 0)
				} else {
					text := rule.Choices[choice]
					reply.Payload = append(reply.Payload, word(uint16(len(text)))...)
					reply.Payload = append(reply.Payload, []byte(text)...)
				}
			}
		}
	}
	if err != nil {
		fail()
		return true
	}
	sendFrame(c, reply)
	return true
}

func creationSlotPayload(game world.Game, command, slot uint16, d core.KDDICreationView) []byte {
	players := int(d.Rules[0]) + 2
	switch command {
	case CmdSlotStatus:
		return append(word(slot), 2)
	case CmdSlotPlrStatus:
		return BuildSlotPlayerStatusPayload(slot, 1, players)
	case CmdSlotTitle:
		return BuildSlotTitlePayload(slot, []byte(d.Title))
	case CmdSlotAttrib2:
		return append(word(slot), 0, byte(players), 0, 4, 0, 1, 0, 4, 0, 1)
	case CmdSlotPwdProt:
		flag := byte(0)
		if d.Protected {
			flag = 1
		}
		return append(word(slot), flag)
	case CmdSlotScenType:
		scen := d.Scenario
		if game == world.GameFile1 && scen > 0 {
			scen--
		}
		p := append(word(slot), word(d.Media)...)
		return append(p, word(uint16(scen))...)
	}
	return nil
}
