package core

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"networkmemories/nm-outbreak-go/internal/world"
	"time"
)

// A ticket proves the native web login. Once its session has used it, the
// original client presents it again after every scenario (after-game lobby
// reconnection), possibly long after login: it then stays valid as long as the
// session lives (silent sessions are released by ReapSilent).
type kddiTicket struct {
	coreID  string
	expires time.Time
	used    bool
}

// LoginKDDI is called by the native web password proof. The eight decimal
// digits are the original client's ticket format, not an account identifier.
func (e *Engine) LoginKDDI(game world.Game, username, password string) (string, error) {
	s, err := e.Login(game, KDDI, "ntsc-j", username, password)
	if err != nil {
		return "", err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.kddiTickets == nil {
		e.kddiTickets = make(map[string]kddiTicket)
	}
	now := time.Now()
	for key, v := range e.kddiTickets {
		if e.sessions[v.coreID] == nil || !v.used && now.After(v.expires) {
			delete(e.kddiTickets, key)
		}
	}
	if len(e.kddiTickets) >= 4096 {
		delete(e.sessions, s.ID)
		return "", errors.New("native session capacity reached")
	}
	for {
		n, err := rand.Int(rand.Reader, big.NewInt(90000000))
		if err != nil {
			delete(e.sessions, s.ID)
			return "", err
		}
		ticket := fmt.Sprintf("%08d", n.Int64()+10000000)
		if _, exists := e.kddiTickets[ticket]; !exists {
			e.kddiTickets[ticket] = kddiTicket{coreID: s.ID, expires: now.Add(30 * time.Minute)}
			return ticket, nil
		}
	}
}
func (e *Engine) KDDIHandoff(game world.Game, ticket string) (Session, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	t, ok := e.kddiTickets[ticket]
	s := e.sessions[t.coreID]
	if !ok || !t.used && time.Now().After(t.expires) || s == nil || s.Game != game || s.Protocol != KDDI {
		return Session{}, ErrSession
	}
	t.used = true
	e.kddiTickets[ticket] = t
	return cloneSession(*s), nil
}

// Native suffix is opaque: only the module's measured bytes are accepted.
func (e *Engine) SelectKDDIHandle(id, handle, nickname string) (Session, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	s := e.sessions[id]
	if s == nil || s.Protocol != KDDI || len(handle) != 6 || nickname == "" {
		return Session{}, ErrSession
	}
	if s.Account.Handle == handle && s.Account.NativeNickname == nickname {
		return cloneSession(*s), nil
	}
	if s.RoomID != "" || s.InScenario || e.sessionRounds[id] != nil {
		return Session{}, ErrSession
	}
	for _, pair := range s.Account.HandlePairs {
		if pair.Handle == handle && pair.Nickname == nickname {
			s.Account.Handle = handle
			s.Account.NativeNickname = nickname
			return cloneSession(*s), nil
		}
	}
	return Session{}, ErrSession
}

func (e *Engine) KDDILobbyLoginSuffix(game world.Game) []byte {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]byte(nil), e.modules[game].KDDILobbyLoginSuffix...)
}

func (e *Engine) KDDIVersionReplyAllowed(game world.Game, query, reply uint16, payload []byte) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	variant := e.modules[game].KDDIVersionReply
	if variant != nil {
		return variant(query, reply, payload)
	}
	return query == reply
}

func (e *Engine) KDDIHeartbeatPayload(game world.Game) []byte {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]byte(nil), e.modules[game].KDDIHeartbeatPayload...)
}

// KDDIDownloadManifest returns module-owned catalog metadata, never executable data.
func (e *Engine) KDDIDownloadManifest(game world.Game) []byte {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]byte(nil), e.modules[game].KDDIDownloadManifest...)
}

func (e *Engine) KDDIDownloadBlock(game world.Game, request []byte) []byte {
	read := e.modules[game].KDDIDownloadBlock
	if read == nil {
		return nil
	}
	return read(request)
}

// The measured module variant carries opaque bytes, never a claimed identity.
func (e *Engine) KDDIAreaExitAllowed(game world.Game, payload []byte) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	expected := e.modules[game].KDDIAreaExitPayload
	return len(payload) == 0 || len(expected) > 0 && bytes.Equal(expected, payload)
}

// KDDIEmptyPlayerScore returns only the module's absence envelope. It does not
// project opaque reports or historical profile data into native rankings.
func (e *Engine) KDDIEmptyPlayerScore(game world.Game, number byte) []byte {
	e.mu.Lock()
	defer e.mu.Unlock()
	callback := e.modules[game].KDDIEmptyPlayerScore
	if callback == nil {
		return nil
	}
	return callback(number)
}
