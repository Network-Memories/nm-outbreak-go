// Roster framing and post-scenario id=0 lookup follow openSNAP Outbreak plugins.
package snap

import (
	"encoding/binary"
	"net"
)

func (s *Server) queryRoster(pc net.PacketConn, remote net.Addr, req Message) {
	if len(req.Payload) < 4 {
		return
	}
	requested := binary.BigEndian.Uint32(req.Payload)
	var rows [][]byte
	if req.TypeFlags&FlagChannelBits == FlagRoom {
		wid, ok := s.bindings.RoomWorldID(requested)
		if requested == 0 {
			if b, exists := s.bindings.Get(req.SessionID); exists {
				if actor, exists := s.engine.Session(b.CoreID); exists {
					wid = actor.RoomID
					ok = wid != ""
				}
			}
		}
		if ok {
			if room, exists := s.engine.Worlds().GetRoom(wid); exists {
				for _, member := range room.Members {
					profile, err := westernProfile(member.Profile, *room, member.Connection)
					if err != nil {
						return
					}
					profile, err = s.engine.ProjectSNAPAftergameProfile(member.Connection, profile)
					if err != nil {
						return
					}
					wire, err := s.engine.SNAPID(member.Connection)
					if err != nil {
						return
					}
					name := member.WesternName
					if name == "" {
						return
					}
					rows = append(rows, BuildMemberRecord(name, wire, profile))
				}
			}
		}
	}
	_ = s.sendList(pc, remote, req, req.TypeFlags&FlagChannelBits|FlagResponse, func(count int) []byte {
		h := make([]byte, 12)
		binary.BigEndian.PutUint32(h, requested)
		binary.BigEndian.PutUint32(h[4:], uint32(len(rows)))
		binary.BigEndian.PutUint32(h[8:], uint32(count))
		return h
	}, rows)
}
