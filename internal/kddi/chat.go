package kddi

import (
	"log"
	"net"
	"networkmemories/nm-outbreak-go/internal/modules/file2"
	"networkmemories/nm-outbreak-go/internal/world"
)

func (s *Server) handleChat(c net.Conn, f LobbyFrame, id string) bool {
	if f.Command != 0x6701 {
		return false
	}
	if s.game != world.GameFile2 || f.Kind != 0x10 {
		return true
	}
	plain, n, err := decodeNativeField(f.Payload, f.PacketID)
	// One opaque trailer byte is outside the length-delimited shifted text.
	// BioServer2 getChatOutData ignores it. Never infer room/round or identity
	// from this byte; the authenticated engine supplies that context.
	if err != nil || n+1 != len(f.Payload) {
		return true
	}
	text, err := file2.DecodeJapaneseChatText(plain)
	if err != nil {
		return true
	}
	if actor, ok := s.engine.Session(id); ok && actor.RoomID == "" {
		err = s.engine.PublishAreaChat(id, text)
	} else {
		_, err = s.engine.AftergameMembers(id)
		err = s.engine.PublishRoomChat(id, err == nil, text)
	}
	if err != nil {
		log.Printf("[nm.kddi.%s] native room chat refused: %v", s.name, err)
	}
	// CHATIN is BROADCAST, not QUERY. Its native local echo is a CHATOUT event.
	return true
}
