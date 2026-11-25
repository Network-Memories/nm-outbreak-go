package snap

import (
	"encoding/binary"
	"fmt"
	"net"
	"sort"

	"networkmemories/nm-outbreak-go/internal/core"
)

func (s *Server) sendJapaneseLaunchPart(pc net.PacketConn, remote net.Addr, receiver, host, tag uint32, body []byte) error {
	payload := make([]byte, 12, len(body)+12)
	binary.BigEndian.PutUint32(payload, 1)
	binary.LittleEndian.PutUint32(payload[8:], tag)
	payload = append(payload, body...)
	return s.sendReply(pc, remote, Message{TypeFlags: FlagReliable | FlagRoom, Command: CmdSendTarget, SessionID: host, RouteSessionID: receiver, AckNumber: s.reliable.lastACK(remote, receiver), Payload: payload})
}

func (s *Server) sendJapaneseLaunchTable(pc net.PacketConn, remote net.Addr, receiver, host uint32, table core.LaunchDetails) error {
	send := func(tag uint32, body []byte) error {
		return s.sendJapaneseLaunchPart(pc, remote, receiver, host, tag, body)
	}
	if err := send(5, []byte{byte(table.Number), byte(table.Count)}); err != nil {
		return err
	}
	members := table.Room.Members
	sort.Slice(members, func(i, j int) bool { return members[i].Seat < members[j].Seat })
	recipient := fmt.Sprintf("%08x", receiver)
	for i, member := range members {
		body := make([]byte, 34)
		body[0] = byte(i)
		for j := range recipient {
			digit := recipient[j]
			if digit >= 'a' {
				digit = digit - 'a' + 10
			} else {
				digit -= '0'
			}
			body[2+j] = '0' + digit
		}
		copy(body[18:], member.WesternName)
		if err := send(6, body); err != nil {
			return err
		}
	}
	for i := range members {
		for offset := 0; offset < 240; offset += 50 {
			data := table.Profiles[i][offset:min(offset+50, 240)]
			body := make([]byte, 54)
			body[0], body[1] = byte(i), byte(len(data))
			binary.LittleEndian.PutUint16(body[2:], uint16(offset))
			copy(body[4:], data)
			if err := send(7, body); err != nil {
				return err
			}
		}
	}
	return send(8, nil)
}
