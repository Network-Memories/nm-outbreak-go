package world

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sort"
	"sync"
	"time"
)

// Game identifies an Outbreak title module.
type Game string

const (
	GameFile1 Game = "outbreak"
	GameFile2 Game = "outbreak2"
)

// Member represents a real, authenticated player in a room.
type Member struct {
	Connection  string    `json:"connection"`
	Adapter     string    `json:"adapter"` // snap-file1, kddi-file1, snap-file2, kddi-file2
	OwnerID     string    `json:"owner_id,omitempty"`
	NativeUser  string    `json:"native_user,omitempty"`
	NMUserID    string    `json:"nm_user_id,omitempty"`
	Name        string    `json:"name"`
	WesternName string    `json:"western_name,omitempty"`
	Handle      string    `json:"handle,omitempty"`
	Region      string    `json:"region"` // ntsc-j, pal, ntsc-u
	Seat        int       `json:"seat"`
	Profile     []byte    `json:"-"`
	JoinedAt    time.Time `json:"joined_at"`
}

// Room represents a live waiting or active scenario room shared directly in memory
// across SNAP and KDDI transports without any HTTP/JSON bridge.
type Room struct {
	CreatedAt   time.Time         `json:"-"`
	NativeMedia uint16            `json:"-"`
	ID          string            `json:"id"`
	Game        Game              `json:"game"`
	HostConn    string            `json:"host_connection"`
	HostSeat    int               `json:"host_seat"`
	Area        int               `json:"area"`
	Scenario    int               `json:"scenario,omitempty"`
	Title       string            `json:"title"`
	TitleHex    string            `json:"title_hex"`
	Protected   bool              `json:"protected"`
	Playing     bool              `json:"playing"`
	MaxPlayers  int               `json:"max_players"`
	RuleValues  []byte            `json:"rule_values,omitempty"`
	Difficulty  int               `json:"difficulty"`
	StatWord    uint32            `json:"stat_word"`
	Settings    map[string]string `json:"settings,omitempty"`
	Members     []Member          `json:"members"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// ChatMessage is an in-memory cross-region chat event (area or room).
type ChatMessage struct {
	Game      Game      `json:"game"`
	Channel   string    `json:"channel"` // "area:free" or "world:<room_id>"
	Sender    string    `json:"sender"`
	Adapter   string    `json:"adapter"`
	Text      string    `json:"text"`
	Timestamp time.Time `json:"timestamp"`
}

// GamePacket is a direct in-memory game frame routed between SNAP and KDDI peers.
type GamePacket struct {
	RoomID   string
	FromConn string
	Payload  []byte
}

// Registry is the unified in-memory state for all Outbreak File #1 and File #2 worlds.
type Registry struct {
	mu       sync.RWMutex
	rooms    map[string]*Room
	chatSubs map[Game][]chan ChatMessage
	gameSubs map[string]map[string]chan GamePacket // roomID -> connID -> ch
}

func NewRegistry() *Registry {
	return &Registry{
		rooms:    make(map[string]*Room),
		chatSubs: make(map[Game][]chan ChatMessage),
		gameSubs: make(map[string]map[string]chan GamePacket),
	}
}

func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// Snapshots own their mutable fields; native transports must not mutate shared state.
func cloneRoom(room Room) Room {
	room.RuleValues = append([]byte(nil), room.RuleValues...)
	if room.Settings != nil {
		settings := make(map[string]string, len(room.Settings))
		for k, v := range room.Settings {
			settings[k] = v
		}
		room.Settings = settings
	}
	room.Members = append([]Member(nil), room.Members...)
	for i := range room.Members {
		room.Members[i].Profile = append([]byte(nil), room.Members[i].Profile...)
	}
	return room
}

func (r *Registry) CreateRoom(room Room) *Room {
	r.mu.Lock()
	defer r.mu.Unlock()
	if room.ID == "" {
		room.ID = newID()
	}
	room.CreatedAt = time.Now().UTC()
	room.UpdatedAt = room.CreatedAt
	cp := cloneRoom(room)
	r.rooms[cp.ID] = &cp
	snapshot := cloneRoom(cp)
	return &snapshot
}

func (r *Registry) GetRoom(id string) (*Room, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	rm, ok := r.rooms[id]
	if !ok {
		return nil, false
	}
	cp := cloneRoom(*rm)
	return &cp, true
}

// SetPlaying records a native scenario transition already proved by the core.
// A returned snapshot cannot mutate the authoritative room lifecycle.
func (r *Registry) SetPlaying(id string, playing bool) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	room := r.rooms[id]
	if room == nil {
		return false
	}
	room.Playing = playing
	room.UpdatedAt = time.Now().UTC()
	return true
}

func (r *Registry) ListRooms(game Game) []Room {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Room, 0, len(r.rooms))
	for _, rm := range r.rooms {
		if rm.Game != game {
			continue
		}
		cp := cloneRoom(*rm)
		out = append(out, cp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (r *Registry) JoinRoom(roomID string, member Member) (*Room, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rm, ok := r.rooms[roomID]
	if !ok {
		return nil, errors.New("room not found")
	}
	if rm.Playing {
		return nil, errors.New("room already in scenario")
	}
	if len(rm.Members) >= rm.MaxPlayers {
		return nil, errors.New("room is full")
	}
	used := make(map[int]bool, len(rm.Members))
	for _, m := range rm.Members {
		if m.Connection == member.Connection {
			return nil, errors.New("already in room")
		}
		used[m.Seat] = true
	}
	seat := -1
	for s := 0; s < rm.MaxPlayers; s++ {
		if !used[s] {
			seat = s
			break
		}
	}
	if seat < 0 {
		return nil, errors.New("no free seat")
	}
	member.Profile = append([]byte(nil), member.Profile...)
	member.Seat = seat
	member.JoinedAt = time.Now().UTC()
	rm.Members = append(rm.Members, member)
	rm.UpdatedAt = time.Now().UTC()
	cp := cloneRoom(*rm)
	return &cp, nil
}

// UpdateProfile records a newly received native profile without changing its owner or seat.
func (r *Registry) UpdateProfile(roomID, conn string, profile []byte) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	room := r.rooms[roomID]
	if room == nil {
		return false
	}
	for i := range room.Members {
		if room.Members[i].Connection == conn {
			room.Members[i].Profile = append([]byte(nil), profile...)
			room.UpdatedAt = time.Now().UTC()
			return true
		}
	}
	return false
}

func (r *Registry) LeaveRoom(roomID, conn string) (removed bool, disbanded bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rm, ok := r.rooms[roomID]
	if !ok {
		return false, false
	}
	kept := rm.Members[:0]
	for _, m := range rm.Members {
		if m.Connection == conn {
			removed = true
			continue
		}
		kept = append(kept, m)
	}
	if !removed {
		return false, false
	}
	if len(kept) == 0 || conn == rm.HostConn {
		delete(r.rooms, roomID)
		for _, ch := range r.gameSubs[roomID] {
			close(ch)
		}
		delete(r.gameSubs, roomID)
		return true, true
	}
	if subs := r.gameSubs[roomID]; subs != nil {
		if ch, ok := subs[conn]; ok {
			close(ch)
			delete(subs, conn)
		}
	}
	rm.Members = kept
	rm.UpdatedAt = time.Now().UTC()
	return true, false
}

func (r *Registry) SubscribeChat(game Game) chan ChatMessage {
	r.mu.Lock()
	defer r.mu.Unlock()
	ch := make(chan ChatMessage, 64)
	r.chatSubs[game] = append(r.chatSubs[game], ch)
	return ch
}

// UnsubscribeChat releases the bounded subscription when a transport stops.
func (r *Registry) UnsubscribeChat(game Game, target chan ChatMessage) {
	r.mu.Lock()
	defer r.mu.Unlock()
	subs := r.chatSubs[game]
	for i, ch := range subs {
		if ch == target {
			r.chatSubs[game] = append(subs[:i], subs[i+1:]...)
			close(ch)
			if len(r.chatSubs[game]) == 0 {
				delete(r.chatSubs, game)
			}
			return
		}
	}
}

func (r *Registry) PublishChat(msg ChatMessage) {
	msg.Timestamp = time.Now().UTC()
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, ch := range r.chatSubs[msg.Game] {
		select {
		case ch <- msg:
		default:
		}
	}
}

func hasMember(room *Room, conn string) bool {
	if room == nil {
		return false
	}
	for _, member := range room.Members {
		if member.Connection == conn {
			return true
		}
	}
	return false
}

func (r *Registry) AttachGameRelay(roomID, conn string) chan GamePacket {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !hasMember(r.rooms[roomID], conn) {
		ch := make(chan GamePacket)
		close(ch)
		return ch
	}
	if r.gameSubs[roomID] == nil {
		r.gameSubs[roomID] = make(map[string]chan GamePacket)
	}
	if ch := r.gameSubs[roomID][conn]; ch != nil {
		return ch
	}
	ch := make(chan GamePacket, 256)
	r.gameSubs[roomID][conn] = ch
	return ch
}

func (r *Registry) DetachGameRelay(roomID, conn string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if subs := r.gameSubs[roomID]; subs != nil {
		if ch, ok := subs[conn]; ok {
			close(ch)
			delete(subs, conn)
		}
		if len(subs) == 0 {
			delete(r.gameSubs, roomID)
		}
	}
}

// Nonblocking sends remain under the read lock: departure cannot close a target mid-send.
// Each recipient owns its payload and cannot alter another peer's frame.
func (r *Registry) FanOutGamePacket(pkt GamePacket) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if !hasMember(r.rooms[pkt.RoomID], pkt.FromConn) {
		return
	}
	for conn, ch := range r.gameSubs[pkt.RoomID] {
		if conn == pkt.FromConn {
			continue
		}
		copyPacket := pkt
		copyPacket.Payload = append([]byte(nil), pkt.Payload...)
		select {
		case ch <- copyPacket:
		default:
		}
	}
}

func (r *Registry) SetStatWord(roomID, host string, stat uint32) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	room := r.rooms[roomID]
	if room == nil || room.HostConn != host {
		return errors.New("room host required")
	}
	room.StatWord = stat
	if room.Game == GameFile1 && stat&0x40000000 == 0 && (stat&uint32(0xfffe01e1)) == 0x00020000 {
		scenIdx := int((stat >> 1) & 0x0f)
		if scenIdx >= 0 && scenIdx < 5 {
			room.Scenario = scenIdx + 1
		}
	}
	room.UpdatedAt = time.Now().UTC()
	return nil
}
