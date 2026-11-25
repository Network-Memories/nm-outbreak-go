package core

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"net"
	"networkmemories/nm-outbreak-go/internal/identity"
	"networkmemories/nm-outbreak-go/internal/protocol/snapauth"
	"networkmemories/nm-outbreak-go/internal/world"
	"strings"
	"time"
)

type SNAPConfig struct {
	AdvertiseHost string
	BootstrapPort uint16
	GamePorts     map[world.Game]uint16
}
type NativeHandoff struct {
	WireID      uint32
	Session     Session
	Payload     []byte
	OutSequence uint32
}
type snapLogin struct {
	wireID                        uint32
	endpoint, host, login, secret string
	fingerprint                   [32]byte
	game                          world.Game
	region                        string
	account                       identity.Account
	expires                       time.Time
	challenge                     []byte
	coreID                        string
	adopted                       bool
	outSequence                   uint32
}

func (e *Engine) ConfigureSNAP(config SNAPConfig) error {
	ip := net.ParseIP(config.AdvertiseHost)
	if ip == nil || ip.To4() == nil || ip.IsUnspecified() || config.BootstrapPort == 0 {
		return errors.New("SNAP advertised IPv4 address and bootstrap port required")
	}
	ports := make(map[world.Game]uint16)
	for game, port := range config.GamePorts {
		if port == 0 {
			return errors.New("invalid SNAP game port")
		}
		ports[game] = port
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.snapConfig = SNAPConfig{config.AdvertiseHost, config.BootstrapPort, ports}
	return nil
}
func (e *Engine) pruneSNAP(now time.Time) {
	for id, login := range e.snapLogins {
		if now.Before(login.expires) {
			continue
		}
		if !login.adopted && login.coreID != "" {
			delete(e.sessions, login.coreID)
		}
		delete(e.snapLogins, id)
	}
}

// Native lookups expose only protocol-derived credentials to the core, never a clear password.
func (e *Engine) BeginSNAP(endpoint string, title uint32, login string, request []byte) (uint32, []byte, error) {
	host, _, err := net.SplitHostPort(endpoint)
	if err != nil {
		return 0, nil, err
	}
	fingerprint := sha256.Sum256(request)
	e.mu.Lock()
	e.pruneSNAP(time.Now())
	for _, pending := range e.snapLogins {
		if pending.endpoint == endpoint && pending.fingerprint == fingerprint && pending.login == login {
			if pending.coreID == "" || e.sessions[pending.coreID] != nil {
				wire, payload := pending.wireID, append([]byte(nil), pending.challenge...)
				e.mu.Unlock()
				return wire, payload, nil
			}
		}
	}
	var game world.Game
	region := ""
	for candidate, definition := range e.modules {
		if r, ok := definition.SNAPTitles[title]; ok {
			if region != "" {
				e.mu.Unlock()
				return 0, nil, errors.New("ambiguous native title")
			}
			game, region = candidate, r
		}
	}
	config := e.snapConfig
	e.mu.Unlock()
	if region == "" || config.GamePorts[game] == 0 || config.AdvertiseHost == "" {
		return 0, nil, errors.New("unsupported native bootstrap target")
	}
	provider, ok := e.authority.(identity.SNAPAuthority)
	if !ok {
		return 0, nil, errors.New("authority does not provide native SNAP credentials")
	}
	credential, err := provider.SNAPCredentials(login)
	if err != nil {
		return 0, nil, err
	}
	account := credential.Account
	if account.UserID < 1 || account.NativeAlias == "" || account.NativeNickname == "" || len(account.Handle) != 6 || !strings.EqualFold(login, account.NativeAlias) {
		return 0, nil, errors.New("native account proof unavailable")
	}
	// 24 hex characters: the original client proves a secret no longer than the 29-character
	// openSNAP reference (a 48-character secret is refused with ERR:812, lab 23/11/2026).
	var secret [12]byte
	if _, err := rand.Read(secret[:]); err != nil {
		return 0, nil, err
	}
	nonce := hex.EncodeToString(secret[:])
	challenge, err := snapauth.Challenge(credential.MagicKey, credential.Seed, nonce, config.BootstrapPort)
	if err != nil {
		return 0, nil, err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.pruneSNAP(time.Now())
	for _, pending := range e.snapLogins {
		if pending.endpoint == endpoint && pending.fingerprint == fingerprint && pending.login == login && (pending.coreID == "" || e.sessions[pending.coreID] != nil) {
			return pending.wireID, append([]byte(nil), pending.challenge...), nil
		}
	}
	if len(e.snapLogins) >= 1024 {
		return 0, nil, errors.New("native bootstrap busy")
	}
	var wire uint32
	for i := 0; i < 32; i++ {
		var raw [4]byte
		if _, err := rand.Read(raw[:]); err != nil {
			return 0, nil, err
		}
		candidate := binary.BigEndian.Uint32(raw[:])
		if candidate == 0 || e.snapLogins[candidate] != nil {
			continue
		}
		free := true
		for _, session := range e.sessions {
			if session.WireID == candidate {
				free = false
				break
			}
		}
		if free {
			wire = candidate
			break
		}
	}

	if wire == 0 {
		return 0, nil, errors.New("native session ID allocation unavailable")
	}
	for id, old := range e.snapLogins {
		if old.endpoint == endpoint {
			if !old.adopted && old.coreID != "" {
				delete(e.sessions, old.coreID)
			}
			delete(e.snapLogins, id)
		}
	}
	e.snapLogins[wire] = &snapLogin{wireID: wire, endpoint: endpoint, host: host, login: login, secret: nonce, fingerprint: fingerprint, game: game, region: region, account: credential.Account, expires: time.Now().Add(2 * time.Minute), challenge: challenge}
	return wire, append([]byte(nil), challenge...), nil
}
func (e *Engine) FinishSNAP(endpoint string, answer []byte) (NativeHandoff, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.pruneSNAP(time.Now())
	var pending *snapLogin
	for _, candidate := range e.snapLogins {
		if candidate.endpoint == endpoint {
			pending = candidate
			break
		}
	}
	if pending == nil {
		return NativeHandoff{}, ErrSession
	}
	if !snapauth.VerifyAnswer(answer, pending.secret) {
		delete(e.snapLogins, pending.wireID)
		return NativeHandoff{}, identity.ErrAuthentication
	}
	if pending.coreID == "" {
		var raw [16]byte
		if _, err := rand.Read(raw[:]); err != nil {
			return NativeHandoff{}, err
		}
		session := &Session{NativeHost: pending.host, NativeLogin: pending.login, ID: hex.EncodeToString(raw[:]), WireID: pending.wireID, Game: pending.game, Protocol: SNAP, Region: pending.region, Account: pending.account}
		e.sessions[session.ID] = session
		e.adoptSessionLocked(session)
		pending.coreID = session.ID
	}
	session := e.sessions[pending.coreID]
	if session == nil {
		return NativeHandoff{}, ErrSession
	}
	payload, err := snapauth.Success(pending.login, e.snapConfig.AdvertiseHost, e.snapConfig.GamePorts[pending.game])
	if err != nil {
		return NativeHandoff{}, err
	}
	return NativeHandoff{WireID: pending.wireID, Session: cloneSession(*session), Payload: payload, OutSequence: pending.outSequence}, nil
}

// A game socket must know its proved wire ID, or retain the exact bootstrap endpoint.
// A public host/login pair alone must never grant another player's identity.
func (e *Engine) SNAPGameHandoff(game world.Game, wire uint32, endpoint, login string) (NativeHandoff, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.pruneSNAP(time.Now())
	host, _, err := net.SplitHostPort(endpoint)
	if err != nil {
		return NativeHandoff{}, ErrSession
	}
	valid := func(candidate *snapLogin) bool {
		return candidate != nil && candidate.game == game && candidate.host == host && candidate.login == login && candidate.coreID != "" && e.sessions[candidate.coreID] != nil
	}
	var match *snapLogin
	if exact := e.snapLogins[wire]; valid(exact) {
		match = exact
	} else {
		for _, candidate := range e.snapLogins {
			if !valid(candidate) || candidate.endpoint != endpoint {
				continue
			}
			if match != nil {
				return NativeHandoff{}, errors.New("ambiguous native handoff")
			}
			match = candidate
		}
	}

	if match == nil {
		return NativeHandoff{}, ErrSession
	}
	match.adopted = true
	return NativeHandoff{WireID: match.wireID, Session: cloneSession(*e.sessions[match.coreID]), OutSequence: match.outSequence}, nil
}

func (e *Engine) SetSNAPSequence(wire, sequence uint32) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if login := e.snapLogins[wire]; login != nil {
		login.outSequence = sequence
	}
}
