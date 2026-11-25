package kddi

import (
	"encoding/binary"
	"log"
	"net"
	"os"
	"time"

	"networkmemories/nm-outbreak-go/internal/world"
)

const (
	SenderServer uint8  = 0x18
	KindTell     uint8  = 0x02
	CmdKeepAlive uint16 = 0x6001
)

func sendFrame(conn net.Conn, f LobbyFrame) {
	if traceKDDI {
		log.Printf("[nm.kddi] -> cmd=%04x kind=%d err=%02x len=%d", f.Command, f.Kind, f.Error, len(f.Payload))
	}
	_ = writeFrame(conn, f)
}

var traceKDDI = os.Getenv("NM_OUTBREAK_TRACE_COMMANDS") == "1"

func (s *Server) handleLobbyFrame(conn net.Conn, f LobbyFrame, sessionID string) {
	if traceKDDI {
		log.Printf("[nm.kddi.%s] <- cmd=%04x kind=%d len=%d", s.name, f.Command, f.Kind, len(f.Payload))
	}
	if s.handleChat(conn, f, sessionID) {
		return
	}
	if s.handleJournal(conn, f, sessionID) {
		return
	}
	if s.handleAftergame(conn, f, sessionID) {
		return
	}
	if s.handleLaunch(conn, f, sessionID) {
		return
	}
	if s.handleCreation(conn, f, sessionID) {
		return
	}
	if s.handleAdmission(conn, f, sessionID) {
		return
	}
	if s.handleDirectory(conn, f, sessionID) {
		return
	}
	switch f.Command {
	case 0x6002:
		// File1 sends this empty command before reconnecting from handle
		// selection. Actual launch completion is handled above.
		if s.game != world.GameFile1 || f.Kind != 1 || len(f.Payload) != 0 {
			return
		}
		actor, ok := s.engine.Session(sessionID)
		if ok && actor.Area == 0 && actor.NativeLobbyID == 0 && actor.NativeRoomID == 0 && actor.RoomID == "" && !actor.InScenario {
			sendFrame(conn, LobbyFrame{Sender: SenderServer, Kind: KindTell, Command: f.Command, PacketID: f.PacketID})
		}
	case 0x6409:
		if len(f.Payload) != 2 || f.Kind != 1 {
			return
		}
		slot := binary.BigEndian.Uint16(f.Payload)
		seconds, err := s.engine.KDDIRemaining(sessionID, slot, time.Now())
		response := LobbyFrame{Sender: SenderServer, Kind: KindTell, Command: f.Command, PacketID: f.PacketID}
		if err != nil {
			response.Error = 0xff
			response.Payload = nativeRefusal()
		} else {
			response.Payload = append(word(slot), word(seconds)...)
		}
		sendFrame(conn, response)
	case CmdKeepAlive:
		sendFrame(conn, LobbyFrame{
			Sender:   SenderServer,
			Kind:     KindTell,
			Command:  CmdKeepAlive,
			PacketID: f.PacketID,
		})

	case CmdSlotStatus, CmdSlotPlrStatus, CmdSlotTitle, CmdSlotAttrib2, CmdSlotPwdProt, CmdSlotScenType:
		if len(f.Payload) != 2 || f.Kind != 1 {
			return
		}
		slot := binary.BigEndian.Uint16(f.Payload[0:2])
		rooms, err := s.engine.KDDISlots(sessionID)
		if err != nil || slot == 0 || slot > s.engine.KDDIDirectory(s.game).Slots {
			return
		}
		if draft, ok := s.engine.KDDICreation(sessionID, slot); ok {
			sendFrame(conn, LobbyFrame{Sender: SenderServer, Kind: KindTell, Command: f.Command, PacketID: f.PacketID, Payload: creationSlotPayload(s.game, f.Command, slot, draft)})
			return
		}
		var payload []byte
		if rm, exists := rooms[slot]; exists {
			switch f.Command {
			case CmdSlotStatus:
				payload = BuildSlotStatusPayload(slot, len(rm.Members), rm.MaxPlayers)
				if rm.Playing || rm.StatWord&0x40000000 != 0 {
					payload[2] = 4
				}
			case CmdSlotPlrStatus:
				payload = BuildSlotPlayerStatusPayload(slot, len(rm.Members), rm.MaxPlayers)
			case CmdSlotTitle:
				payload = BuildSlotTitlePayload(slot, []byte(rm.Title))
			case CmdSlotAttrib2:
				payload = []byte{byte(slot >> 8), byte(slot), 0, byte(rm.MaxPlayers), 0, 4, 0, 1, 0, 4, 0, 1}
			case CmdSlotPwdProt:
				prot := byte(0)
				if rm.Protected {
					prot = 1
				}
				payload = []byte{byte(slot >> 8), byte(slot), prot}
			case CmdSlotScenType:
				scen := byte(rm.Scenario)
				if scen == 0 {
					scen = 1
				}
				if s.game == world.GameFile1 && scen > 0 {
					scen--
				}
				media := rm.NativeMedia
				if media == 0 {
					if s.game == world.GameFile1 {
						media = 0x01
					} else {
						media = 0x11
					}
				}
				payload = append(word(slot), word(media)...)
				payload = append(payload, 0, scen)
			}
		} else {
			// Empty slot
			switch f.Command {
			case CmdSlotStatus:
				payload = []byte{byte(slot >> 8), byte(slot), 1}
			case CmdSlotPlrStatus:
				payload = []byte{byte(slot >> 8), byte(slot), 0, 0, 0, 0, 0, 0, 0, 0}
			case CmdSlotTitle:
				payload = BuildSlotTitlePayload(slot, nil)
			case CmdSlotAttrib2:
				payload = []byte{byte(slot >> 8), byte(slot), 0, 0, 0, 4, 0, 1, 0, 4, 0, 1}
			case CmdSlotPwdProt:
				payload = []byte{byte(slot >> 8), byte(slot), 0}
			case CmdSlotScenType:
				payload = []byte{byte(slot >> 8), byte(slot), 0, 0, 0, 0}
			}
		}
		sendFrame(conn, LobbyFrame{
			Sender:   SenderServer,
			Kind:     KindTell,
			Command:  f.Command,
			PacketID: f.PacketID,
			Payload:  payload,
		})

		// Native chat must be decoded only after a core-authenticated lobby handshake.
		// No username/IP-only publication is accepted here.
	}
}
