// Native launch query forms adapted from BioServer PacketHandler.java.
// The table comes from the actual Western host, never demonstration scores.
package kddi

import (
	"encoding/binary"
	"net"
	"networkmemories/nm-outbreak-go/internal/core"
	"sort"
	"strconv"
	"time"
)

func (s *Server) handleLaunch(c net.Conn, f LobbyFrame, id string) bool {
	if f.Command == 0x6508 {
		actor, ok := s.engine.Session(id)
		if ok && actor.Game == s.game && f.Kind == 0x10 && len(f.Payload) == 0 {
			_ = s.engine.JapaneseLaunchStart(id)
		}
		return true
	}
	switch f.Command {
	case 0x6911, 0x6912, 0x6913, 0x6914, 0x6915, 0x6916, 0x6917, 0x6002:
	default:
		return false
	}
	if !s.engine.HasNativeLaunch(id) {
		return false
	}
	actor, ok := s.engine.Session(id)
	if !ok || actor.Game != s.game || actor.Protocol != core.KDDI || f.Kind != 1 {
		return true
	}
	reply := LobbyFrame{Sender: SenderServer, Kind: KindTell, Command: f.Command, PacketID: f.PacketID}
	fail := func() { reply.Error = 0xff; reply.Payload = nativeRefusal(); sendFrame(c, reply) }
	if !validLaunchPayload(f.Command, f.Payload) {
		fail()
		return true
	}
	// 6002 confirms a complete launch; it cannot acknowledge03.
	if f.Command != 0x6002 {
		ready, err := s.engine.AcknowledgeNativeLaunch(id)
		if err != nil {
			fail()
			return true
		}
		timer := time.NewTimer(5 * time.Second)
		select {
		case <-ready:
		case <-timer.C:
			fail()
			return true
		}
		timer.Stop()
	}
	launch, err := s.engine.NativeLaunch(id)
	if err != nil {
		fail()
		return true
	}
	switch f.Command {
	case 0x6917:
		number := f.Payload[0]
		if int(number) > launch.Count {
			fail()
			return true
		}
		reply.Payload = s.engine.KDDIEmptyPlayerScore(s.game, number)
		if len(reply.Payload) == 0 {
			fail()
			return true
		}
	case 0x6911:
		reply.Payload = []byte{byte(launch.Count)}
	case 0x6912:
		reply.Payload = []byte{byte(launch.Number + 1)}
	case 0x6913:
		number := int(f.Payload[0])
		reply.Payload = []byte{byte(number), 0}
		if number <= launch.Count {
			members := launch.Room.Members
			sort.Slice(members, func(i, j int) bool { return members[i].Seat < members[j].Seat })
			peer, exists := s.engine.Session(members[number-1].Connection)
			if !exists {
				fail()
				return true
			}
			peer.Profile = append([]byte(nil), launch.Profiles[number-1][:208]...)
			data, err := nativeMember(peer)
			if err != nil {
				fail()
				return true
			}
			reply.Payload = append([]byte{byte(number), 1}, data...)
			reply.Payload = append(reply.Payload, 0, 0, 6)
		}
	case 0x6914:
		reply.Payload = launch.Rules
	case 0x6915:
		reply.Payload = append(word(15), []byte(launch.NativeNumber)...)
		reply.Payload = append(reply.Payload, 0, 0)
	case 0x6916:
		host, portText, err := net.SplitHostPort(s.gameAddress)
		port, e := strconv.Atoi(portText)
		ip := net.ParseIP(host).To4()
		if err != nil || e != nil || port < 1 || port > 65535 || ip == nil {
			fail()
			return true
		}
		reply.Payload = append(word(4), ip...)
		reply.Payload = append(reply.Payload, 0, 2, 0, 0, 0, 0, 0x1e, 0)
		binary.BigEndian.PutUint16(reply.Payload[8:10], uint16(port))
	case 0x6002:
		if s.engine.ScenarioPresence(id, true) != nil {
			fail()
			return true
		}
	}
	sendFrame(c, reply)
	return true
}

// Original File2 sends an opaque zero byte for6912. The older empty
// fixture form remains supported; neither form supplies or changes a seat.
func validLaunchPayload(command uint16, payload []byte) bool {
	switch command {
	case 0x6912:
		return len(payload) == 0 || len(payload) == 1 && payload[0] == 0
	case 0x6913:
		return len(payload) == 1 && payload[0] >= 1 && payload[0] <= 4
	case 0x6917:
		return len(payload) == 1 && payload[0] >= 1 && payload[0] <= 4
	default:
		return len(payload) == 0
	}
}
