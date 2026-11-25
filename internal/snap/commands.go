package snap

import (
	"encoding/binary"
	"errors"
	"io"
	"net"
	"networkmemories/nm-outbreak-go/internal/core"
	"sync"
	"time"

	"networkmemories/nm-outbreak-go/internal/modules/file1"
	"networkmemories/nm-outbreak-go/internal/modules/file2"
	"networkmemories/nm-outbreak-go/internal/world"
)

// Measured SNAP command codes (SLUS_207.65, SLES_533.19, SLUS_209.84).
const (
	CmdCreateGameRoom     uint8 = 0x04
	CmdJoin               uint8 = 0x06
	CmdLeave              uint8 = 0x07
	CmdChangeAttribute    uint8 = 0x08
	CmdQueryAttribute     uint8 = 0x09
	CmdQueryUser          uint8 = 0x0a
	CmdQueryGameRooms     uint8 = 0x0b
	CmdChangeUserProperty uint8 = 0x0c
	CmdLoginToKics        uint8 = 0x01
	CmdLogoutClient       uint8 = 0x02
	CmdChangeUserStatus   uint8 = 0x0d
	CmdSend               uint8 = 0x0f
	CmdSendTarget         uint8 = 0x10
	CmdSendEcho           uint8 = 0x14
	CmdAck                uint8 = 0x00
	CmdResultWrapper      uint8 = 0x28
	CmdResultLogin        uint8 = 0x29
	CmdQueryArea          uint8 = 0x48
)

// Binding contains only native UDP routing state. Identity, profile and room membership live in core.
type Binding struct {
	ClientID    uint32
	HasClientID bool
	CoreID      string
	ID          uint32
	Addr        net.Addr
	AreaID      int
	OutSequence uint32
	OutReliable uint32
}

type BindingTable struct {
	mu       sync.RWMutex
	bindings map[uint32]*Binding
	roomIDs  map[string]uint32
	byNumID  map[uint32]string
	nextRoom uint32
}

func NewBindingTable() *BindingTable {
	return &BindingTable{
		bindings: make(map[uint32]*Binding),
		roomIDs:  make(map[string]uint32),
		byNumID:  make(map[uint32]string),
		nextRoom: 1,
	}
}

func (t *BindingTable) Put(s *Binding) {
	t.mu.Lock()
	defer t.mu.Unlock()
	cp := *s
	t.bindings[s.ID] = &cp
}

func (t *BindingTable) Get(id uint32) (*Binding, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	s, ok := t.bindings[id]
	if !ok {
		return nil, false
	}
	cp := *s
	return &cp, true
}

func (t *BindingTable) Delete(id uint32) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.bindings, id)
}

func (t *BindingTable) RoomNumericID(worldID string) uint32 {
	t.mu.Lock()
	defer t.mu.Unlock()
	if id, ok := t.roomIDs[worldID]; ok {
		return id
	}
	id := t.nextRoom
	t.nextRoom++
	t.roomIDs[worldID] = id
	t.byNumID[id] = worldID
	return id
}

func (t *BindingTable) RoomWorldID(numID uint32) (string, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	w, ok := t.byNumID[numID]
	return w, ok
}

func readCString(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}

func sendMsg(pc net.PacketConn, remote net.Addr, m Message) error {
	raw, err := EncodeMessages([]Message{m})
	if err != nil {
		return err
	}
	n, err := pc.WriteTo(raw, remote)
	if err == nil && n != len(raw) {
		return io.ErrShortWrite
	}
	return err
}

// adoptSession binds a native transport ID only after the core has proved its owner.
// The bootstrap verifier must call this internally; no packet can provide a core ID.
func (s *Server) adoptSession(id uint32, remote net.Addr, coreID string) error {
	actor, ok := s.engine.Session(coreID)
	if !ok || actor.Game != s.game || actor.Protocol != core.SNAP || id == 0 || remote == nil {
		return core.ErrSession
	}
	if err := s.engine.BindSNAPID(coreID, id); err != nil {
		return err
	}
	if old, ok := s.bindings.Get(id); ok {
		if old.CoreID != coreID {
			return errors.New("native session already bound")
		}
		if old.Addr.String() == remote.String() {
			return nil
		}
		s.bindings.Rebind(id, remote)
		return nil
	}
	s.bindings.Put(&Binding{ID: id, CoreID: coreID, Addr: remote})
	return nil
}

func (s *Server) dispatchCommand(pc net.PacketConn, remote net.Addr, msg Message) {
	if s.bindings == nil || remote == nil {
		return
	}
	bound, ok := s.bindings.Get(msg.SessionID)
	if !ok || bound.Addr == nil || bound.Addr.Network() != remote.Network() || bound.Addr.String() != remote.String() {
		return
	}
	actor, ok := s.engine.Session(bound.CoreID)
	if !ok || actor.Game != s.game || actor.Protocol != core.SNAP {
		return
	}
	switch msg.Command {
	case CmdLoginToKics:
		s.reliable.restartIncoming(remote, msg.SessionID)
		// Adoption requires the bootstrap proof above; a claimed username is ignored.
		port := 0
		if addr, ok := pc.LocalAddr().(*net.UDPAddr); ok {
			port = addr.Port
		}
		payload := make([]byte, 12)
		binary.BigEndian.PutUint32(payload, uint32(port))
		binary.BigEndian.PutUint32(payload[4:], 0x01234567)
		binary.BigEndian.PutUint32(payload[8:], msg.SessionID)
		flags := FlagChannelBits
		if msg.TypeFlags&FlagReliable != 0 && (!msg.HasSourceSession || msg.SourceSessionID == msg.SessionID) {
			flags |= FlagReliable
		}
		s.sendReply(pc, remote, Message{TypeFlags: flags, Sequence: msg.Sequence, Command: CmdResultLogin, SessionID: msg.SessionID, AckNumber: msg.PacketSeq, Payload: payload})

	case CmdChangeAttribute:
		if len(msg.Payload) != 8 || string(msg.Payload[:4]) != "STAT" {
			return
		}
		value := ResultOK
		if err := s.engine.SetRoomStat(bound.CoreID, binary.BigEndian.Uint32(msg.Payload[4:])); err != nil {
			value = ErrorRefused
		}
		s.sendReply(pc, remote, BuildResult(msg, msg.SessionID, value, FlagRoom))
	case CmdSend, CmdSendTarget:
		s.relayNative(pc, remote, msg, actor)
	case CmdSendEcho:
		if actor.InScenario {
			return
		}
		flags := msg.TypeFlags & FlagChannelBits
		if flags == 0 {
			flags = FlagRoom
		}
		s.sendReply(pc, remote, Message{TypeFlags: responseFlags(msg, flags|FlagResponse), Command: msg.Command, Sequence: msg.Sequence, SessionID: msg.SessionID, AckNumber: msg.PacketSeq, Payload: append([]byte(nil), msg.Payload...)})
	case 0x49:
		s.searchRooms(pc, remote, msg)
	case 0x25:
		s.searchUsers(pc, remote, msg)
	case CmdQueryArea:
		s.queryAreas(pc, remote, msg)
	case CmdQueryAttribute:
		s.queryAttribute(pc, remote, msg)
	case CmdChangeUserStatus:
		if len(msg.Payload) != 4 {
			return
		}
		status := binary.BigEndian.Uint32(msg.Payload)
		value := ResultOK
		// Both supported SNAP Outbreak dialects use these measured status words.
		if status == 1 || status == 0xf7e00001 {
			if err := s.engine.ScenarioPresence(bound.CoreID, status == 1); err != nil {
				value = ErrorRefused
			} else if status == 1 && s.started {
				s.watchEngine(pc, bound.CoreID)
			}
		}
		s.sendReply(pc, remote, BuildResult(msg, msg.SessionID, value, FlagRoom))
	case CmdChangeUserProperty:
		if sess, ok := s.bindings.Get(msg.SessionID); ok && len(msg.Payload) == ProfileLength {
			if err := s.engine.SetProfile(sess.CoreID, msg.Payload); err != nil {
				s.sendReply(pc, remote, BuildResult(msg, msg.SessionID, ErrorRefused, FlagRoom))
				return
			}
			s.sendReply(pc, remote, BuildResult(msg, msg.SessionID, ResultOK, FlagRoom))
		}

	case CmdQueryGameRooms:
		if len(msg.Payload) < 4 {
			return
		}
		lobbyID := int(binary.BigEndian.Uint32(msg.Payload[0:4]))
		sharedArea := LobbyToSharedArea(s.game, lobbyID)
		rooms := s.engine.Worlds().ListRooms(s.game)
		var rows [][]byte
		for _, rm := range rooms {
			if rm.Playing || rm.StatWord&0x40000000 != 0 || rm.Area != sharedArea {
				continue
			}
			numID := s.bindings.RoomNumericID(rm.ID)
			rules := rm.StatWord
			if rules == 0 {
				if s.game == world.GameFile1 {
					rules = file1.OpenRoomStatBase
				} else if st, err := file2.EncodeStatWord(rm.Scenario, rm.MaxPlayers, 0); err == nil {
					rules = st
				}
			}
			rows = append(rows, BuildRoomRow(rm.Title, uint32(len(rm.Members)), rules, uint32(rm.MaxPlayers), numID))
		}
		_ = s.sendList(pc, remote, msg, FlagChannelBits|FlagResponse, func(count int) []byte {
			h := make([]byte, 12)
			binary.BigEndian.PutUint32(h[4:], 1)
			binary.BigEndian.PutUint32(h[8:], uint32(count))
			return h
		}, rows)

	case CmdQueryUser:
		s.queryRoster(pc, remote, msg)

	case CmdCreateGameRoom:
		if sess, ok := s.bindings.Get(msg.SessionID); ok && len(msg.Payload) >= 44 {
			title := readCString(msg.Payload[0:16])
			maxP := int(binary.BigEndian.Uint32(msg.Payload[16:20]))
			if maxP < 2 || maxP > 4 {
				s.sendReply(pc, remote, BuildResult(msg, msg.SessionID, ErrorRefused, FlagRoom))
				return
			}
			rules := binary.BigEndian.Uint32(msg.Payload[40:44])
			scenario := 0
			difficulty := 0
			if s.game == world.GameFile1 {
				if scen, _, _, diff, _, err := file1.DecodeStatWord(rules); err == nil {
					scenario = scen
					difficulty = diff
				}
			} else if s.game == world.GameFile2 {
				scenario = int((rules & 0xff) >> 1)
			}
			rm, err := s.engine.CreateRoom(sess.CoreID, core.RoomOptions{Password: []byte(readCString(msg.Payload[20:36])), Area: LobbyToSharedArea(s.game, int(actor.NativeLobbyID)), Title: title, MaxPlayers: maxP, Difficulty: difficulty, StatWord: rules, Scenario: scenario})
			if err != nil {
				s.sendReply(pc, remote, BuildResult(msg, msg.SessionID, ErrorRefused, FlagRoom))
				return
			}

			numID := s.bindings.RoomNumericID(rm.ID)
			s.sendReply(pc, remote, BuildResult(msg, msg.SessionID, numID, FlagRoom))
		}

	case CmdJoin:
		sess, ok := s.bindings.Get(msg.SessionID)
		if !ok || len(msg.Payload) < 4 {
			return
		}
		chType := msg.TypeFlags & FlagChannelBits
		targetID := binary.BigEndian.Uint32(msg.Payload[0:4])
		if chType == FlagChannelBits {
			if err := s.engine.SetArea(sess.CoreID, LobbyToSharedArea(s.game, int(targetID))); err != nil {
				return
			}
			_ = s.engine.SetNativeLobby(sess.CoreID, targetID)

			s.sendReply(pc, remote, BuildResult(msg, msg.SessionID, ResultOK, chType))
			return
		}
		wid, ok := s.bindings.RoomWorldID(targetID)
		if !ok {
			s.sendReply(pc, remote, BuildResult(msg, msg.SessionID, ErrorRefused, FlagRoom))
			return
		}
		var password []byte
		if len(msg.Payload) >= 24 {
			password = []byte(readCString(msg.Payload[8:24]))
		}
		_, err := s.engine.JoinRoomWithPassword(sess.CoreID, wid, password)
		if err != nil {
			s.sendReply(pc, remote, BuildResult(msg, msg.SessionID, ErrorRefused, FlagRoom))
			return
		}
		s.sendReply(pc, remote, BuildResult(msg, msg.SessionID, ResultOK, FlagRoom))

	case CmdLeave:
		if binding, ok := s.bindings.Get(msg.SessionID); ok {
			if err := s.engine.LeaveRoom(binding.CoreID); err == nil {
				channel := uint16(FlagRoom)
				if msg.TypeFlags&FlagChannelBits == FlagChannelBits {
					channel = FlagChannelBits
					_ = s.engine.SetArea(binding.CoreID, 0)
					_ = s.engine.SetNativeLobby(binding.CoreID, 0)
				}
				s.sendReply(pc, remote, BuildResult(msg, msg.SessionID, ResultOK, channel))
			}
		}
	case CmdLogoutClient:
		if binding, ok := s.bindings.Get(msg.SessionID); ok {
			s.engine.Logout(binding.CoreID)
			s.sendReply(pc, remote, BuildResult(msg, msg.SessionID, ResultOK, FlagRoom))
			s.bindings.Delete(msg.SessionID)
		}
	}
}

func (s *Server) sendReply(pc net.PacketConn, remote net.Addr, m Message) error {
	recipient := m.SessionID
	if m.RouteSessionID != 0 {
		recipient = m.RouteSessionID
	}
	send, err := s.reliable.outboundAllocated(remote, &m, time.Now(), func() (uint32, bool) { return s.bindings.NextSequence(recipient, m.TypeFlags&FlagReliable != 0) })
	if err != nil {
		return err
	}
	if !send {
		return nil
	}
	return sendMsg(pc, remote, m)
}

func (t *BindingTable) FindCore(id string) (*Binding, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	for _, b := range t.bindings {
		if b.CoreID == id {
			cp := *b
			return &cp, true
		}
	}
	return nil, false
}

func (t *BindingTable) NextSequence(id uint32, reliable bool) (uint32, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	b := t.bindings[id]
	if b == nil {
		return 0, false
	}
	if reliable {
		seq := b.OutReliable
		b.OutReliable++
		return seq, true
	}
	b.OutSequence++
	return b.OutSequence, true
}
func (t *BindingTable) Rebind(id uint32, remote net.Addr) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if b := t.bindings[id]; b != nil {
		b.Addr = remote
	}
}
func (t *BindingTable) InitialSequence(id uint32, seq uint32) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if b := t.bindings[id]; b != nil && b.OutSequence == 0 {
		b.OutSequence = seq
	}
}
func (t *BindingTable) Snapshot() []Binding {
	t.mu.RLock()
	defer t.mu.RUnlock()
	var out []Binding
	for _, b := range t.bindings {
		out = append(out, *b)
	}
	return out
}

func (t *BindingTable) ClientAlias(id, alias uint32) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if b := t.bindings[id]; b != nil {
		b.ClientID = alias
		b.HasClientID = true
	}
}
func (t *BindingTable) NativeContext(remote net.Addr, id uint32) (*Binding, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	matches := func(b *Binding) bool {
		return b.Addr != nil && remote != nil && b.Addr.Network() == remote.Network() && b.Addr.String() == remote.String()
	}
	if b := t.bindings[id]; b != nil && matches(b) {
		cp := *b
		return &cp, true
	}
	var found *Binding
	for _, b := range t.bindings {
		if b.HasClientID && b.ClientID == id && matches(b) {
			if found != nil {
				return nil, false
			}
			cp := *b
			found = &cp
		}
	}
	return found, found != nil
}
func (t *BindingTable) SupersedeEndpoint(remote net.Addr, keep uint32) []Binding {
	t.mu.Lock()
	defer t.mu.Unlock()
	var old []Binding
	for id, b := range t.bindings {
		if id != keep && b.Addr != nil && b.Addr.Network() == remote.Network() && b.Addr.String() == remote.String() {
			old = append(old, *b)
			delete(t.bindings, id)
		}
	}
	return old
}
