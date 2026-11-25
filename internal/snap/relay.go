// Native same-dialect relay follows openSNAP Outbreak _handle_send{,_target}.
// Cross-dialect payload interpretation belongs to the game module adapters.
package snap

import (
	"bytes"
	"encoding/binary"
	"log"
	"net"
	"networkmemories/nm-outbreak-go/internal/core"
	"networkmemories/nm-outbreak-go/internal/modules/file2"
	"networkmemories/nm-outbreak-go/internal/protocol/snaplaunch"
	"networkmemories/nm-outbreak-go/internal/world"
	"time"
)

func (s *Server) relayNative(pc net.PacketConn, remote net.Addr, req Message, sender core.Session) {
	const relayBit uint16 = 0x0400
	chat := req.TypeFlags&relayBit != 0
	if chat && req.Command == CmdSend && req.TypeFlags&FlagChannelBits == FlagChannelBits && sender.Game == world.GameFile2 && s.engine.AreaChatSupported(sender.ID) && file2.IsWesternAreaChatEnvelope(req.Payload) {
		text, err := file2.DecodeWesternAreaChat(req.Payload, sender.Account.NativeAlias)
		if err == nil {
			err = s.engine.PublishAreaChat(sender.ID, text)
		}
		if err != nil {
			log.Printf("[nm.snap.%s] native area chat refused: %v", s.name, err)
		}
		return
	}
	if chat && req.Command == CmdSend && req.TypeFlags&FlagChannelBits == FlagRoom && sender.Game == world.GameFile2 && file2.IsWesternChatEnvelope(req.Payload) {
		text, aftergame, err := file2.DecodeWesternChat(req.Payload, sender.Account.NativeAlias)
		if err == nil {
			err = s.engine.PublishRoomChat(sender.ID, aftergame, text)
		}
		if err != nil {
			log.Printf("[nm.snap.%s] native room chat refused: %v", s.name, err)
		}
		return
	}

	if sender.InScenario && !chat && req.Command == CmdSend && req.TypeFlags&FlagChannelBits == FlagRoom && !req.EngineHandled {
		s.relayMixedEngine(sender, req.Payload)
	}
	payload := append([]byte(nil), req.Payload...)
	var target uint32
	if req.Command == CmdSendTarget {
		if len(payload) < 10 {
			return
		}
		target = binary.BigEndian.Uint32(payload[4:8])
		if !chat {
			if len(payload) == 12 && binary.BigEndian.Uint32(payload) == 1 && req.TypeFlags&FlagChannelBits == FlagRoom {
				tag := binary.LittleEndian.Uint32(payload[8:])
				if tag == 10 || tag == 2 || tag == 14 {
					if wait, err := s.engine.WesternGuestWait(sender.ID, target, time.Now()); err == nil {
						answer, value := uint32(15), uint32(0)
						switch tag {
						case 10:
							answer, value = 11, uint32(wait)*30
						case 2:
							answer, value = 12, 2
						}
						reply := make([]byte, 12)
						binary.BigEndian.PutUint32(reply, 1)
						binary.LittleEndian.PutUint32(reply[8:], answer)
						if answer != 15 {
							reply = binary.LittleEndian.AppendUint32(reply, value)
						}
						if err := s.sendReply(pc, remote, Message{TypeFlags: FlagReliable | FlagRoom, Command: CmdSendTarget, SessionID: target, RouteSessionID: req.SessionID, AckNumber: req.PacketSeq, Payload: reply}); err != nil {
							log.Printf("[nm.snap.%s] waiting reply failed: %v", s.name, err)
						}
						return
					}
				}
			}
			for _, peer := range s.engine.Sessions() {
				if peer.Game == sender.Game && peer.Protocol == core.KDDI && peer.WireID == target {
					if s.engine.AftergameJournalSupported(sender.Game) {
						if more, chunk, err := file2.JournalChunk(payload, false); err == nil {
							if err = s.engine.ReceiveAftergameJournal(sender.ID, peer.ID, more, chunk); err != nil {
								log.Printf("[nm.snap.%s] aftergame journal refused: %v", s.name, err)
							}
							return
						}
					}
					if len(payload) == 20 && binary.BigEndian.Uint32(payload) == 1 && binary.LittleEndian.Uint32(payload[8:]) == 1 && bytes.Equal(payload[12:], make([]byte, 8)) {
						if err := s.engine.SignalWaitingGreeting(sender.ID, target); err != nil {
							log.Printf("[nm.snap.%s] native waiting greeting refused: %v", s.name, err)
							return
						}
						// File #1 only (go-f1-us-us-launch.pcap, 23/11/2026): the original US host sends
						// tag 3 only to a guest that answered its greeting 01 with 0a, its 0b with 02,
						// and then sent 0e. The admitted Japanese guest has no such SNAP exchange.
						if sender.Game == world.GameFile1 {
							s.answerAsJapaneseGuest(pc, remote, req, target, 0x0a)
						}
						return
					}
					if sender.Game == world.GameFile1 && len(payload) >= 12 && binary.BigEndian.Uint32(payload) == 1 {
						switch binary.LittleEndian.Uint32(payload[8:]) {
						case 0x0b:
							if len(payload) == 16 {
								s.answerAsJapaneseGuest(pc, remote, req, target, 0x02)
								s.answerAsJapaneseGuest(pc, remote, req, target, 0x0e)
							}
							return
						case 0x0c, 0x0f:
							if len(payload) == 12 {
								return
							}
						}
					}
					m, known, err := snaplaunch.Decode(payload)
					if known {
						if err == nil && m.Tag == 4 {
							table, handled, launchErr := s.engine.JapaneseLaunchGuestTable(sender.ID, target, m.Body)
							if handled {
								if launchErr == nil {
									launchErr = s.sendJapaneseLaunchTable(pc, remote, req.SessionID, target, table)
									if launchErr == nil {
										s.engine.CompleteJapaneseLaunchGuest(sender.ID, table.RoundID)
									}
								}
								if launchErr != nil {
									log.Printf("[nm.snap.%s] Japanese launch reply failed: %v", s.name, launchErr)
								}
								return
							}
						}
						if err == nil {
							err = s.engine.ReceiveWesternLaunch(sender.ID, m)
						}
						if err != nil {
							log.Printf("[nm.snap.%s] mixed launch refused: %v", s.name, err)
						}
						return
					}
				}
			}
		}

		for i := 4; i < 8; i++ {
			payload[i] = 0
		}
	}
	var peers []core.Session
	if chat && req.TypeFlags&FlagChannelBits == FlagChannelBits {
		peers = s.engine.Sessions()
	} else if room, ok := s.engine.Worlds().GetRoom(sender.RoomID); ok {
		for _, m := range room.Members {
			if peer, ok := s.engine.Session(m.Connection); ok {
				peers = append(peers, peer)
			}
		}
	}
	for _, peer := range peers {
		if peer.ID == sender.ID || peer.Game != sender.Game || peer.Protocol != core.SNAP {
			continue
		}
		binding, ok := s.bindings.FindCore(peer.ID)
		if !ok || binding.Addr == nil {
			continue
		}
		if req.Command == CmdSendTarget {
			if binding.ID != target || peer.InScenario {
				continue
			}
			if !chat && (sender.RoomID == "" || peer.RoomID != sender.RoomID) {
				continue
			}
		} else if req.TypeFlags&FlagChannelBits == FlagChannelBits && chat {
			if sender.NativeLobbyID == 0 || peer.NativeLobbyID != sender.NativeLobbyID || peer.RoomID != "" || peer.InScenario {
				continue
			}
		} else {
			if sender.RoomID == "" || peer.RoomID != sender.RoomID {
				continue
			}
			if chat && peer.InScenario {
				continue
			}
		}
		flags := req.TypeFlags &^ FlagMulti
		if req.Command == CmdSendTarget {
			flags = FlagReliable | (req.TypeFlags & (FlagChannelBits | relayBit))
		} else if chat {
			if req.TypeFlags&FlagChannelBits == FlagChannelBits {
				flags = 0x1000 | relayBit | FlagResponse
			} else {
				flags = FlagRoom | relayBit | FlagResponse
			}
		}
		_ = s.sendReply(pc, binding.Addr, Message{TypeFlags: flags, Command: req.Command, Sequence: req.Sequence, SessionID: req.SessionID, RouteSessionID: binding.ID, AckNumber: s.reliable.lastACK(binding.Addr, binding.ID), Payload: payload})
	}
	// Reliable requests are ACKed by the enclosing datagram, including multi children.
	if req.TypeFlags&FlagReliable == 0 && !req.EmbeddedInMulti {
		_ = sendMsg(pc, remote, Message{TypeFlags: FlagRoom | FlagResponse, Command: CmdAck, SessionID: req.SessionID, AckNumber: req.PacketSeq})
	}
}

// answerAsJapaneseGuest sends one measured empty File #1 waiting-room message from the
// admitted Japanese guest's routing ID to the Western host (no body, as captured).
func (s *Server) answerAsJapaneseGuest(pc net.PacketConn, remote net.Addr, req Message, guest uint32, tag uint32) {
	reply := make([]byte, 12)
	binary.BigEndian.PutUint32(reply, 1)
	binary.LittleEndian.PutUint32(reply[8:], tag)
	if err := s.sendReply(pc, remote, Message{TypeFlags: FlagReliable | FlagRoom, Command: CmdSendTarget, SessionID: guest, RouteSessionID: req.SessionID, AckNumber: req.PacketSeq, Payload: reply}); err != nil {
		log.Printf("[nm.snap.%s] Japanese guest waiting answer %02x failed: %v", s.name, tag, err)
	}
}
