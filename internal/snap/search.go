// Attribute search and list pagination ported from openSNAP plugins/common.py
// and outbreak{,2}/plugin.py (AGPL-3.0).
package snap

import (
	"encoding/binary"
	"errors"
	"net"
	"networkmemories/nm-outbreak-go/internal/core"
	"networkmemories/nm-outbreak-go/internal/world"
	"strings"
)

type searchCondition struct {
	Name     string
	Operator byte
	Value    []byte
}

func parseSearch(p []byte) (uint32, []searchCondition, error) {
	if len(p) < 8 {
		return 0, nil, errors.New("short attribute search")
	}
	limit := binary.BigEndian.Uint32(p)
	n := int(p[4])
	offset := 8
	var rows []searchCondition
	for i := 0; i < n; i++ {
		if offset+5 > len(p) {
			return 0, nil, errors.New("truncated search")
		}
		flags := p[offset+4]
		size := 0
		switch flags >> 5 {
		case 0:
			size = 4
		case 1:
			size = 8
		case 2:
			size = 16
		default:
			return 0, nil, errors.New("unknown search field")
		}
		if offset+5+size > len(p) {
			return 0, nil, errors.New("truncated search field")
		}
		rows = append(rows, searchCondition{string(p[offset : offset+4]), flags & 31, append([]byte(nil), p[offset+5:offset+5+size]...)})
		offset += 5 + size
	}
	return limit, rows, nil
}
func responseFlags(req Message, flags uint16) uint16 {
	if req.TypeFlags&FlagReliable != 0 && (!req.HasSourceSession || req.SourceSessionID == req.SessionID) {
		flags |= FlagReliable
	}
	return flags
}
func (s *Server) sendList(pc net.PacketConn, remote net.Addr, req Message, flags uint16, header func(int) []byte, rows [][]byte) error {
	const maxPayload = 876
	for _, row := range rows {
		if len(header(0))+len(row) > maxPayload {
			return errors.New("native list row exceeds receive buffer")
		}
	}
	index := 0
	for {
		var page []byte
		count := 0
		for index < len(rows) && len(header(0))+len(page)+len(rows[index]) <= maxPayload {
			page = append(page, rows[index]...)
			count++
			index++
		}
		last := index == len(rows)
		m := Message{TypeFlags: responseFlags(req, flags), Command: req.Command, Sequence: req.Sequence, SessionID: req.SessionID, AckNumber: req.PacketSeq, Payload: append(header(count), page...)}
		if !last {
			m.TypeFlags &^= FlagRoom
			m.Sequence = 1
		}
		if err := s.sendReply(pc, remote, m); err != nil {
			return err
		}
		if last {
			return nil
		}
	}
}
func (s *Server) queryAreas(pc net.PacketConn, remote net.Addr, req Message) {
	limit, conditions, err := parseSearch(req.Payload)
	if err != nil {
		return
	}
	var selected []Area
	low, high := "", ""
	for _, c := range conditions {
		switch {
		case c.Name == "NAME" && c.Operator == 1:
			for _, a := range DefaultAreas(s.game) {
				if a.Name == readCString(c.Value) {
					selected = []Area{a}
					break
				}
			}
		case c.Name == "OID\x00" && c.Operator == 1 && len(c.Value) == 4:
			for _, a := range DefaultAreas(s.game) {
				if a.ID == binary.BigEndian.Uint32(c.Value) {
					selected = []Area{a}
					break
				}
			}
		case c.Name == "NAME" && c.Operator == 4:
			low = readCString(c.Value)
		case c.Name == "NAME" && c.Operator == 6:
			high = readCString(c.Value)
		}
	}
	if low != "" && high != "" {
		selected = nil
		for _, a := range DefaultAreas(s.game) {
			if len(a.Name) == len(low) && a.Name >= low && a.Name <= high {
				selected = append(selected, a)
			}
		}
	}
	capacity := uint32(400)
	if s.game == world.GameFile1 {
		capacity = 100
	}
	var rows [][]byte
	for i, a := range selected {
		if uint32(i) >= limit {
			break
		}
		row := make([]byte, 36)
		copy(row, a.Name)
		binary.BigEndian.PutUint32(row[16:], s.engine.NativePopulation(s.game, core.SNAP, a.ID))
		binary.BigEndian.PutUint32(row[20:], capacity)
		binary.BigEndian.PutUint32(row[24:], capacity)
		binary.BigEndian.PutUint32(row[32:], a.ID)
		rows = append(rows, row)
	}
	_ = s.sendList(pc, remote, req, FlagChannelBits|FlagResponse, func(count int) []byte {
		h := make([]byte, 12)
		binary.BigEndian.PutUint32(h, limit)
		binary.BigEndian.PutUint32(h[4:], 1)
		binary.BigEndian.PutUint32(h[8:], uint32(count))
		return h
	}, rows)
}
func (s *Server) queryAttribute(pc net.PacketConn, remote net.Addr, req Message) {
	if len(req.Payload) != 8 {
		return
	}
	id := binary.BigEndian.Uint32(req.Payload)
	attr := string(req.Payload[4:8])
	var users, capacity uint32
	if req.TypeFlags&FlagChannelBits == FlagRoom {
		if wid, ok := s.bindings.RoomWorldID(id); ok {
			if room, ok := s.engine.Worlds().GetRoom(wid); ok {
				users = uint32(len(room.Members))
				capacity = uint32(room.MaxPlayers)
			}
		}
	} else {
		for _, a := range DefaultAreas(s.game) {
			if a.ID == id {
				users = s.engine.NativePopulation(s.game, core.SNAP, id)
				capacity = 400
				if s.game == world.GameFile1 {
					capacity = 100
				}
				break
			}
		}
	}
	value := uint32(0)
	if attr == "USER" {
		value = users
	} else if attr == "MAXI" {
		value = capacity
	}
	p := append([]byte(nil), req.Payload...)
	p = append(p, 0, 0, 0, 0)
	binary.BigEndian.PutUint32(p[8:], value)
	_ = s.sendReply(pc, remote, Message{TypeFlags: responseFlags(req, req.TypeFlags&FlagChannelBits|FlagResponse), Sequence: req.Sequence, Command: req.Command, SessionID: req.SessionID, AckNumber: req.PacketSeq, Payload: p})
}

func (s *Server) searchRooms(pc net.PacketConn, remote net.Addr, req Message) {
	limit, conditions, err := parseSearch(req.Payload)
	if err != nil {
		return
	}
	var rooms []*world.Room
	var masks [][2]uint32
	for _, c := range conditions {
		if len(c.Value) == 4 && c.Operator == 1 {
			id := binary.BigEndian.Uint32(c.Value)
			if c.Name == "OID\x00" {
				rooms = nil
				if wid, ok := s.bindings.RoomWorldID(id); ok {
					if room, ok := s.engine.Worlds().GetRoom(wid); ok {
						rooms = []*world.Room{room}
					}
				}
			} else if c.Name == "LOID" {
				rooms = nil
				for _, room := range s.engine.Worlds().ListRooms(s.game) {
					if room.Area == LobbyToSharedArea(s.game, int(id)) {
						rooms = append(rooms, &room)
					}
				}
			}
		} else if c.Name == "STAT" && c.Operator == 1 && len(c.Value) == 8 {
			masks = append(masks, [2]uint32{binary.BigEndian.Uint32(c.Value), binary.BigEndian.Uint32(c.Value[4:])})
		}
	}
	var rows [][]byte
	for _, room := range rooms {
		matches := true
		for _, mask := range masks {
			if room.StatWord&mask[0] != mask[1] {
				matches = false
			}
		}
		if !matches {
			continue
		}
		if limit != 0 && uint32(len(rows)) >= limit {
			break
		}
		row := make([]byte, 40)
		copy(row, packFixed(room.Title, 16))
		if s.game == world.GameFile2 {
			binary.BigEndian.PutUint32(row[20:], uint32(len(room.Members)))
			binary.BigEndian.PutUint32(row[32:], uint32(room.MaxPlayers))
		}
		binary.BigEndian.PutUint32(row[28:], room.StatWord)
		binary.BigEndian.PutUint32(row[36:], s.bindings.RoomNumericID(room.ID))
		rows = append(rows, row)
	}
	_ = s.sendList(pc, remote, req, FlagChannelBits|FlagResponse, func(count int) []byte {
		h := make([]byte, 12)
		binary.BigEndian.PutUint32(h[4:], uint32(len(rows)))
		binary.BigEndian.PutUint32(h[8:], uint32(count))
		return h
	}, rows)
}

// Native friend search is an online lookup, not the creation of a separate graph.
func (s *Server) searchUsers(pc net.PacketConn, remote net.Addr, req Message) {
	name := strings.TrimRight(readCString(req.Payload), "-")
	var rows [][]byte
	for _, peer := range s.engine.Sessions() {
		if peer.Game != s.game || peer.Protocol != core.SNAP || peer.Account.NativeAlias != name || len(peer.Profile) != 240 {
			continue
		}
		row := make([]byte, 32+len(peer.Profile))
		copy(row, packFixed(peer.Account.NativeAlias, 16))
		binary.BigEndian.PutUint32(row[16:], 1)
		binary.BigEndian.PutUint32(row[20:], peer.NativeLobbyID)
		if peer.RoomID != "" {
			binary.BigEndian.PutUint32(row[24:], s.bindings.RoomNumericID(peer.RoomID))
		}
		binary.BigEndian.PutUint32(row[28:], uint32(len(peer.Profile)))
		copy(row[32:], peer.Profile)
		rows = append(rows, row)
	}
	_ = s.sendList(pc, remote, req, FlagChannelBits|FlagResponse, func(count int) []byte {
		h := make([]byte, 12)
		binary.BigEndian.PutUint32(h[4:], uint32(len(rows)))
		binary.BigEndian.PutUint32(h[8:], uint32(count))
		return h
	}, rows)
}
