package kddi

import (
	"context"
	"log"
	"net"
	"networkmemories/nm-outbreak-go/internal/core"
	"networkmemories/nm-outbreak-go/internal/modules/file2"
	"networkmemories/nm-outbreak-go/internal/world"
	"sync"
	"time"
)

// Query replies and broadcasts share one framed writer. Partial writes cannot
// interleave, and a slow native lobby never holds the engine lock.
type lobbyFrameConn struct {
	net.Conn
	mu sync.Mutex
}

func (c *lobbyFrameConn) WriteLobbyFrame(f LobbyFrame) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	return writeFrameRaw(c.Conn, f)
}

func (s *Server) watchNativeRoom(ctx context.Context, c net.Conn, id string, nextID func() uint16) {
	sub, err := s.engine.SubscribeRoom(id)
	if err != nil {
		return
	}
	go func() {
		defer s.engine.UnsubscribeRoom(id, sub)
		for {
			s.engine.FlushAftergameJournals(id)
			var event core.RoomEvent
			select {
			case <-ctx.Done():
				return
			case next, ok := <-sub.Events():
				if !ok {
					if sub.Overflowed() {
						log.Printf("[nm.kddi.%s] native room notifications congested", s.name)
						c.Close()
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
			slot := event.NativeSlot
			send := func(command uint16, payload []byte) bool {
				return writeFrame(c, LobbyFrame{Sender: SenderServer, Kind: 0x10, Command: command, PacketID: nextID(), Payload: payload}) == nil
			}
			if event.Kind == core.HallPopulationChanged {
				if !s.engine.HallPopulationDeliverable(id, event) {
					continue
				}
				hall, directory := s.engine.KDDIBrowsePopulation(peer.Game, event.PopulationArea)
				body := append(word(uint16(event.PopulationArea)), word(hall)...)
				body = append(body, word(directory)...)
				body = append(body, 0xff, 0xff, 0, 0)
				if !send(0x6205, body) {
					c.Close()
					return
				}
				continue
			}
			if event.Kind == core.RoomChat {
				if peer.Game != world.GameFile2 || !s.engine.RoomChatDeliverable(id, event) {
					continue
				}
				body, err := file2.EncodeJapaneseChat(event.Actor.Account.Handle, event.Actor.Account.NativeNickname, event.ChatText)
				if err != nil {
					log.Printf("[nm.kddi.%s] native chat cannot fit recipient fields", s.name)
					continue
				}
				if !send(0x6702, body) {
					c.Close()
					return
				}
				continue
			}
			if event.Kind == core.RoomNativeEvent {
				if len(event.Actor.Account.Handle) != 6 || len(event.Journal) > 0xffff {
					continue
				}
				body := append(word(6), []byte(event.Actor.Account.Handle)...)
				body = append(body, word(uint16(len(event.Journal)))...)
				body = append(body, event.Journal...)
				if !send(0x670e, body) {
					c.Close()
					return
				}
				continue
			}
			if event.Kind == core.RoomAftergameJournal {
				if !s.engine.AftergameJournalSupported(peer.Game) || len(event.Actor.Account.Handle) != 6 {
					continue
				}
				blocks, err := file2.JournalBlocks(event.Journal, true)
				if err != nil {
					continue
				}
				for _, block := range blocks {
					body := append(word(6), []byte(event.Actor.Account.Handle)...)
					body = append(body, word(uint16(len(block)))...)
					body = append(body, block...)
					if !send(0x670e, body) {
						c.Close()
						return
					}
				}
				continue
			}
			if members, err := s.engine.AftergameMembers(id); err == nil {
				ok := true
				switch event.Kind {
				case core.RoomAftergameReturned, core.RoomProfileChanged:
					body, err := nativeMember(event.Actor)
					if err != nil {
						continue
					}
					ok = send(0x6215, body)
				case core.RoomLeft:
					if len(event.Actor.Account.Handle) != 6 {
						continue
					}
					ok = send(0x6214, append(word(6), []byte(event.Actor.Account.Handle)...))
				default:
					continue
				}
				if !ok || !send(0x6212, word(uint16(len(members)))) {
					c.Close()
					return
				}
				continue
			}
			if slot == 0 {
				continue
			}
			switch event.Kind {
			case core.RoomWaitingGreeting:
				if len(event.Actor.Account.Handle) != 6 {
					continue
				}
				body := append(word(6), []byte(event.Actor.Account.Handle)...)
				body = append(body, 0, 8)
				body = append(body, make([]byte, 8)...)
				if !send(0x670e, body) {
					c.Close()
					return
				}
				continue
			case core.RoomWaitingReady:
				number := 0
				for _, m := range event.Room.Members {
					if m.Connection == event.Actor.ID {
						number = m.Seat + 1
						break
					}
				}
				if number < 1 || number > 4 {
					continue
				}
				if !send(0x6506, []byte{0, byte(number), 0, 0}) {
					c.Close()
					return
				}
				continue
			case core.RoomLaunchAnnounced:
				if !send(0x6404, append(word(slot), 4)) || !send(0x6910, nil) {
					c.Close()
					return
				}
				continue
			case core.RoomJoined:
				data, err := nativeMember(event.Actor)
				if err != nil {
					log.Printf("[nm.kddi.%s] native room profile unavailable", s.name)
					continue
				}
				if !send(0x6503, data) {
					c.Close()
					return
				}
			case core.RoomLeft:
				if event.Disbanded {
					text := "<LF=6><BODY><CENTER>host cancelled game<END>"
					if !send(0x6505, append(word(uint16(len(text))), []byte(text)...)) {
						c.Close()
						return
					}
				} else {
					if len(event.Actor.Account.Handle) != 6 {
						continue
					}
					if !send(0x6502, append(word(6), []byte(event.Actor.Account.Handle)...)) {
						c.Close()
						return
					}
				}
			default:
				continue // Profile-only updates require the measured ready lifecycle.
			}
			status := BuildSlotStatusPayload(slot, event.Count, event.Room.MaxPlayers)
			if event.Disbanded {
				status[2] = 1
			} else if event.Room.Playing || event.Room.StatWord&0x40000000 != 0 {
				status[2] = 4
			}
			if !send(0x6403, BuildSlotPlayerStatusPayload(slot, event.Count, event.Room.MaxPlayers)) || !send(0x6404, status) || !send(0x640b, append(word(slot), 0, byte(event.Room.MaxPlayers), 0, 4, 0, 1, 0, 4, 0, 1)) {
				c.Close()
				return
			}
		}
	}()
}
