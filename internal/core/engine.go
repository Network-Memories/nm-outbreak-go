// Package core owns the shared authenticated session and room lifecycle.
// SNAP and KDDI are native codecs around this engine, not servers joined by a bridge.
package core

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strconv"
	"sync"
	"time"

	"networkmemories/nm-outbreak-go/internal/identity"
	"networkmemories/nm-outbreak-go/internal/modules"
	"networkmemories/nm-outbreak-go/internal/results"
	"networkmemories/nm-outbreak-go/internal/world"
)

type Protocol string

const (
	SNAP Protocol = "snap"
	KDDI Protocol = "kddi"
)

var ErrSession = errors.New("authenticated session required")

type Session struct {
	presenceRevision uint64 // invalidate queued area text after native navigation

	NativeRoomID  uint16
	NativeHost    string `json:"-"`
	NativeLogin   string
	InScenario    bool
	NativeLobbyID uint32
	ID            string
	WireID        uint32
	Area          int
	Game          world.Game
	Protocol      Protocol
	Region        string
	Account       identity.Account
	Profile       []byte
	RoomID        string
}

type RoomOptions struct {
	NativeMedia                            uint16
	Password                               []byte
	Area, Scenario, MaxPlayers, Difficulty int
	Title, TitleHex                        string
	StatWord                               uint32
	RuleValues                             []byte
}

type Engine struct {
	waiting           map[string]waitingState
	launches          map[string]*nativeLaunch
	roomSubscriptions map[string]*RoomSubscription
	kddiDrafts        map[string]*kddiDraft
	kddiPreviews      map[string]kddiPreview
	kddiLobbyAttached map[string]bool
	kddiSlots         map[kddiBrowseKey]map[string]uint16
	roomSecrets       map[string]roomSecret
	resultLedger      *results.Ledger
	roomRounds        map[string]*scenarioRound
	sessionRounds     map[string]*scenarioRound
	kddiTickets       map[string]kddiTicket
	mu                sync.Mutex
	worlds            *world.Registry
	authority         identity.Authority
	sessions          map[string]*Session
	modules           map[world.Game]modules.Definition
	snapConfig        SNAPConfig
	snapLogins        map[uint32]*snapLogin
	lastAlive         map[string]time.Time
}

func New(registry *world.Registry, authority identity.Authority, definitions ...modules.Definition) *Engine {
	if registry == nil {
		registry = world.NewRegistry()
	}
	definitionsByGame := make(map[world.Game]modules.Definition)
	for _, definition := range definitions {
		if _, exists := definitionsByGame[definition.Game]; exists {
			panic("duplicate game module")
		}
		definitionsByGame[definition.Game] = definition.Clone()
	}
	return &Engine{worlds: registry, authority: authority, sessions: make(map[string]*Session), modules: definitionsByGame, snapLogins: make(map[uint32]*snapLogin)}
}

// Worlds supplies the same read model to both codecs and the state API.
// Authenticated lifecycle mutations belong in the methods below.
func (e *Engine) Worlds() *world.Registry { return e.worlds }
func cloneSession(s Session) Session      { s.Profile = append([]byte(nil), s.Profile...); return s }

// Login is invoked only when the native authentication flow has the user's password.
// Challenge-based SNAP handoffs require a separately measured verifier before this
// transport can be connected; a packet username alone never creates a core session.
func (e *Engine) Login(game world.Game, protocol Protocol, region, username, password string) (Session, error) {
	if !e.modules[game].Supports(string(protocol), region) {
		return Session{}, errors.New("unsupported native dialect")
	}
	if e.authority == nil {
		return Session{}, ErrSession
	}
	account, err := e.authority.Authenticate(username, password)
	if err != nil {
		return Session{}, err
	}
	if account == nil || account.UserID < 1 || account.Username == "" || account.NativeAlias == "" || account.NativeNickname == "" || len(account.Handle) != 6 {
		return Session{}, errors.New("authority returned incomplete native identity")
	}
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return Session{}, err
	}
	session := Session{ID: hex.EncodeToString(raw[:]), Game: game, Protocol: protocol, Region: region, Account: *account}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.sessions[session.ID] = &session
	e.adoptSessionLocked(&session)
	return cloneSession(session), nil
}

func (e *Engine) Session(id string) (Session, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	s, ok := e.sessions[id]
	if !ok {
		return Session{}, false
	}
	return cloneSession(*s), true
}

func (e *Engine) SetProfile(id string, profile []byte) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	s := e.sessions[id]
	if s == nil {
		return ErrSession
	}
	size := e.modules[s.Game].ProfileBytes[string(s.Protocol)]
	if size < 1 || len(profile) != size {
		return errors.New("profile does not match measured native dialect")
	}
	room, _ := e.worlds.GetRoom(s.RoomID)
	if !s.InScenario {
		if err := e.roomNotificationsReadyLocked(room, id); err != nil {
			return err
		}
	}
	s.Profile = append([]byte(nil), profile...)
	if s.RoomID != "" {
		e.worlds.UpdateProfile(s.RoomID, s.ID, profile)
		if !s.InScenario {
			updated, _ := e.worlds.GetRoom(s.RoomID)
			if updated != nil {
				e.emitRoomEventLocked(RoomProfileChanged, s, updated, len(updated.Members), false)
			}
		}
	}
	return nil
}

func member(s *Session) world.Member {
	name := s.Account.NativeAlias
	if s.Protocol == KDDI {
		name = s.Account.NativeNickname
	}
	suffix := "file2"
	if s.Game == world.GameFile1 {
		suffix = "file1"
	}
	return world.Member{Connection: s.ID, Adapter: string(s.Protocol) + "-" + suffix, OwnerID: strconv.FormatInt(s.Account.UserID, 10), Name: name, WesternName: s.Account.NativeAlias, Handle: s.Account.Handle, Region: s.Region, Profile: append([]byte(nil), s.Profile...)}
}

func (e *Engine) CreateRoom(id string, o RoomOptions) (*world.Room, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.kddiDrafts[id] != nil {
		return nil, ErrSession
	}
	return e.createRoomLocked(id, o)
}
func (e *Engine) createRoomLocked(id string, o RoomOptions) (*world.Room, error) {
	var hallBefore *Session
	if h := e.sessions[id]; h != nil {
		snapshot := cloneSession(*h)
		hallBefore = &snapshot
	}
	defer func() { e.emitHallChangeLocked(hallBefore, e.sessions[id]) }()
	s := e.sessions[id]
	if s == nil {
		return nil, ErrSession
	}
	if s.RoomID != "" {
		return nil, errors.New("already in a room")
	}
	if len(s.Profile) == 0 || !e.modules[s.Game].ValidRoom(o.Area, o.Scenario, o.MaxPlayers, o.Difficulty) {
		return nil, errors.New("missing native profile or unmeasured room options")
	}
	if s.Protocol == KDDI {
		if _, err := e.snapIDLocked(id); err != nil {
			return nil, err
		}
	}
	secret, err := newRoomSecret(o.Password)
	if err != nil {
		return nil, err
	}
	m := member(s)
	m.Seat = 0
	room := e.worlds.CreateRoom(world.Room{NativeMedia: o.NativeMedia, Game: s.Game, HostConn: s.ID, HostSeat: 0, Area: o.Area, Scenario: o.Scenario, Title: o.Title, TitleHex: o.TitleHex, MaxPlayers: o.MaxPlayers, Difficulty: o.Difficulty, StatWord: o.StatWord, RuleValues: o.RuleValues, Members: []world.Member{m}, Protected: len(o.Password) > 0})
	if len(o.Password) > 0 {
		if e.roomSecrets == nil {
			e.roomSecrets = make(map[string]roomSecret)
		}
		e.roomSecrets[room.ID] = secret
	}
	s.presenceRevision++
	s.RoomID = room.ID
	return room, nil
}

func (e *Engine) JoinRoom(id, roomID string) (*world.Room, error) {
	return e.JoinRoomWithPassword(id, roomID, nil)
}
func (e *Engine) JoinRoomWithPassword(id, roomID string, password []byte) (*world.Room, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.joinRoomLocked(id, roomID, password)
}
func (e *Engine) joinRoomLocked(id, roomID string, password []byte) (*world.Room, error) {
	var hallBefore *Session
	if h := e.sessions[id]; h != nil {
		snapshot := cloneSession(*h)
		hallBefore = &snapshot
	}
	defer func() { e.emitHallChangeLocked(hallBefore, e.sessions[id]) }()
	if e.kddiDrafts[id] != nil {
		return nil, ErrSession
	}
	s := e.sessions[id]
	if s == nil {
		return nil, ErrSession
	}
	if s.RoomID == roomID {
		if room, ok := e.worlds.GetRoom(roomID); ok {
			return room, nil
		}
	}
	if s.RoomID != "" || len(s.Profile) == 0 {
		return nil, errors.New("already in a room or native profile missing")
	}
	target, ok := e.worlds.GetRoom(roomID)
	if !ok || target.Game != s.Game {
		return nil, errors.New("room unavailable for this authenticated session")
	}
	if target.StatWord&0x40000000 != 0 || e.roomRounds[roomID] != nil {
		return nil, errors.New("room admission closed")
	}
	if target.Protected {
		secret, exists := e.roomSecrets[roomID]
		if !exists || !secret.matches(password) {
			return nil, errors.New("room password rejected")
		}
	}
	if err := e.roomNotificationsReadyLocked(target, id); err != nil {
		return nil, err
	}
	if s.Protocol == KDDI {
		if _, err := e.snapIDLocked(id); err != nil {
			return nil, err
		}
	}
	room, err := e.worlds.JoinRoom(roomID, member(s))
	if err != nil {
		return nil, err
	}
	s.presenceRevision++
	s.RoomID = roomID
	e.emitRoomEventLocked(RoomJoined, s, room, len(room.Members), false)
	return room, nil
}

func (e *Engine) leave(s *Session) {
	hallBefore := cloneSession(*s)
	defer func() { e.emitHallChangeLocked(&hallBefore, s) }()
	e.discardRoomEventsLocked(s.ID)
	delete(e.waiting, s.ID)
	e.cancelKDDIDraftLocked(s.ID)
	delete(e.kddiPreviews, s.ID)
	if s.RoomID == "" {
		return
	}
	roomID := s.RoomID
	if round := e.roomRounds[roomID]; round != nil {
		e.closeEngineRoundLocked(round)
	}
	e.cancelLaunchesLocked(roomID)
	if round := e.roomRounds[roomID]; round != nil && len(round.entered) == 0 {
		delete(e.roomRounds, roomID)
	}
	before, _ := e.worlds.GetRoom(roomID)
	removed, disbanded := e.worlds.LeaveRoom(roomID, s.ID)
	s.presenceRevision++
	s.RoomID = ""
	if disbanded {
		e.clearWaitingLocked(roomID)
		delete(e.roomRounds, roomID)
		delete(e.roomSecrets, roomID)
		for _, other := range e.sessions {
			if other.RoomID == roomID {
				hallOther := cloneSession(*other)
				other.presenceRevision++
				other.RoomID = ""
				e.emitHallChangeLocked(&hallOther, other)
			}
		}
	}
	if removed && before != nil {
		count := len(before.Members) - 1
		if disbanded {
			count = 0
		}
		e.emitRoomEventLocked(RoomLeft, s, before, count, disbanded)
	}
}
func (e *Engine) LeaveRoom(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	s := e.sessions[id]
	if s == nil {
		return ErrSession
	}
	e.leave(s)
	return nil
}

// Logout is an explicit account/lobby departure. Closing only a game relay socket
// must use DetachRelay instead and must not remove the player's logical place.
func (e *Engine) Logout(id string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.logoutLocked(id)
}
func (e *Engine) logoutLocked(id string) {
	delete(e.lastAlive, id)
	if s := e.sessions[id]; s != nil {
		e.leave(s)
		hallBefore := cloneSession(*s)
		delete(e.sessions, id)
		e.emitHallChangeLocked(&hallBefore, nil)
		if sub := e.roomSubscriptions[id]; sub != nil {
			delete(e.roomSubscriptions, id)
			close(sub.events)
		}
	}
}

func (e *Engine) AttachRelay(id string) (chan world.GamePacket, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	s := e.sessions[id]
	if s == nil || s.RoomID == "" {
		return nil, ErrSession
	}
	return e.worlds.AttachGameRelay(s.RoomID, id), nil
}
func (e *Engine) DetachRelay(id string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if s := e.sessions[id]; s != nil && s.RoomID != "" {
		e.worlds.DetachGameRelay(s.RoomID, id)
	}
}
func (e *Engine) Relay(id string, payload []byte) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	s := e.sessions[id]
	if s == nil || s.RoomID == "" {
		return ErrSession
	}
	e.worlds.FanOutGamePacket(world.GamePacket{RoomID: s.RoomID, FromConn: id, Payload: payload})
	return nil
}

// Sessions returns a private snapshot for native control-plane routing.
func (e *Engine) Sessions() []Session {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]Session, 0, len(e.sessions))
	for _, s := range e.sessions {
		out = append(out, cloneSession(*s))
	}
	return out
}

// Presence returns a consistent snapshot for the public presence projection.
func (e *Engine) Presence(game world.Game) ([]world.Room, []world.Member) {
	e.mu.Lock()
	defer e.mu.Unlock()
	waiting := make([]world.Member, 0)
	for _, s := range e.sessions {
		if s.Game == game && s.Area > 0 && s.RoomID == "" && !s.InScenario {
			waiting = append(waiting, member(s))
		}
	}
	return e.worlds.ListRooms(game), waiting
}

func (e *Engine) SetRoomStat(id string, stat uint32) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	s := e.sessions[id]
	if s == nil {
		return ErrSession
	}
	return e.worlds.SetStatWord(s.RoomID, id, stat)
}
