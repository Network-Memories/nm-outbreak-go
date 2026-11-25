// KDDI handshake and field transform ported from BioServer Packet{,Handler}.java.
// Copyright 2013-2019 obsrv.org / no23. AGPL-3.0-or-later; see notices.
package kddi

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/text/encoding/japanese"
	"networkmemories/nm-outbreak-go/internal/core"
	"networkmemories/nm-outbreak-go/internal/reports"
	"networkmemories/nm-outbreak-go/internal/world"
)

const (
	cmdLogin    uint16 = 0x6101
	cmdVersion  uint16 = 0x6103
	cmdHNList   uint16 = 0x6131
	cmdHNSelect uint16 = 0x6132
)

func readFrame(r io.Reader) (LobbyFrame, error) {
	var h [12]byte
	if _, err := io.ReadFull(r, h[:]); err != nil {
		return LobbyFrame{}, err
	}
	p := make([]byte, int(binary.BigEndian.Uint16(h[4:6])))
	if _, err := io.ReadFull(r, p); err != nil {
		return LobbyFrame{}, err
	}
	var stream LobbyStream
	f := stream.Feed(append(h[:], p...))
	frame := f[0]
	frame.raw = append(append([]byte(nil), h[:]...), p...)
	return frame, nil
}
func writeFrame(w io.Writer, f LobbyFrame) error {
	if framed, ok := w.(interface{ WriteLobbyFrame(LobbyFrame) error }); ok {
		return framed.WriteLobbyFrame(f)
	}
	return writeFrameRaw(w, f)
}
func writeFrameRaw(w io.Writer, f LobbyFrame) error {
	b, err := f.Encode()
	if err != nil {
		return err
	}
	for len(b) > 0 {
		n, err := w.Write(b)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		b = b[n:]
	}
	return nil
}
func ticketAnswer(p []byte, pid uint16, lobby bool, suffix ...[]byte) (string, error) {
	if lobby && len(p) > 12 && len(suffix) == 1 && len(suffix[0]) == len(p)-12 && bytes.Equal(p[12:], suffix[0]) {
		p = p[:12]
	}

	if lobby {
		if len(p) != 12 || binary.BigEndian.Uint16(p[:2]) != 10 {
			return "", core.ErrSession
		}
		p = p[2:]
	}
	if len(p) != 10 {
		return "", core.ErrSession
	}
	for _, c := range p {
		if c < '0' || c > '9' {
			return "", core.ErrSession
		}
	}
	a, _ := strconv.Atoi(string(p[:5]))
	b, _ := strconv.Atoi(string(p[5:]))
	a -= int(pid)
	b -= int(pid)
	if a < 0 || a > 9999 || b < 0 || b > 9999 {
		return "", core.ErrSession
	}
	return fmt.Sprintf("%04d%04d", a, b), nil
}
func decodeNativeField(p []byte, pid uint16) ([]byte, int, error) {
	if len(p) < 4 {
		return nil, 0, core.ErrSession
	}
	n := int(binary.BigEndian.Uint16(p)) + 2
	if n < 4 || n > len(p) {
		return nil, 0, core.ErrSession
	}
	out := append([]byte(nil), p[4:n]...)
	fix := [8]int{21, 23, 10, 17, 23, 19, 6, 13}
	masks := [8]int{0x33, 0x30, 0x3c, 0x34, 0x2d, 0x30, 0x3c, 0x34}
	for i := range out {
		shift := fix[i&7] - (i & 0xf8) - int(byte(pid)) + ((int(byte(pid))-9+i)&masks[i&7])*2
		out[i] ^= byte(shift)
	}
	return out, n, nil
}
func nativeHandlePair(handle, nickname string) ([]byte, error) {
	nick, err := japanese.ShiftJIS.NewEncoder().String(nickname)
	if err != nil || len(handle) != 6 || len(nick) == 0 || len(nick) > 16 {
		return nil, core.ErrSession
	}
	for _, c := range []byte(handle) {
		if c < 32 || c > 126 {
			return nil, core.ErrSession
		}
	}
	for _, c := range []byte(nick) {
		if c < 32 || c == 127 {
			return nil, core.ErrSession
		}
	}
	p := append(word(6), handle...)
	p = append(p, word(uint16(len(nick)))...)
	return append(p, nick...), nil
}

func hnList(actor core.Session) ([]byte, error) {
	b := []byte{0}
	add := func(handle, nickname string) error {
		pair, err := nativeHandlePair(handle, nickname)
		if err != nil {
			return err
		}
		b = append(append(b, pair...), 0, 0)
		b[0]++
		return nil
	}
	account := actor.Account
	if err := add(account.Handle, account.NativeNickname); err != nil {
		return nil, err
	}
	for _, pair := range account.HandlePairs {
		if pair.Handle == "" || pair.Handle == account.Handle || b[0] == 3 {
			continue
		}
		if err := add(pair.Handle, pair.Nickname); err != nil {
			return nil, err
		}
	}
	return b, nil
}

func (s *Server) serveNative(ctx context.Context, c net.Conn) {
	c = &lobbyFrameConn{Conn: c}
	stop := context.AfterFunc(ctx, func() { c.Close() })
	defer stop()
	defer c.Close()
	gameSocket := strings.HasSuffix(s.name, "-game")
	var seed [2]byte
	if _, err := rand.Read(seed[:]); err != nil {
		return
	}
	pid := binary.BigEndian.Uint16(seed[:])
	if pid == 0 {
		pid = 1
	}
	sender := SenderServer
	cmd := cmdLogin
	payload := []byte{0x28, 0x37}
	if gameSocket {
		sender = 0x28
		cmd = CmdGSLogin
		payload = nil
	}
	c.SetDeadline(time.Now().Add(30 * time.Second))
	if writeFrame(c, LobbyFrame{Sender: sender, Kind: 1, Command: cmd, PacketID: pid, Payload: payload}) != nil {
		return
	}
	answer, err := readFrame(c)
	who := byte(0x81)
	if gameSocket {
		who = 0x82
	}
	if err != nil || answer.Sender != who || answer.Kind != 2 || answer.Command != cmd || answer.PacketID != pid || answer.Error != 0 {
		return
	}
	ticket, err := ticketAnswer(answer.Payload, pid, !gameSocket, s.engine.KDDILobbyLoginSuffix(s.game))
	if err != nil {
		return
	}
	actor, err := s.engine.KDDIHandoff(s.game, ticket)
	if err != nil {
		return
	}
	if gameSocket {
		if !actor.InScenario {
			return
		}
		c.SetDeadline(time.Time{})
		s.RelayGameConnection(c, actor.ID, &GameStream{})
		return
	}
	if err := s.engine.AttachKDDILobby(actor.ID); err != nil {
		return
	}
	defer s.engine.DetachKDDILobby(actor.ID)
	notifyCtx, notifyCancel := context.WithCancel(ctx)
	defer notifyCancel()
	var packetCounter atomic.Uint32
	packetCounter.Store(uint32(pid))
	nextID := func() uint16 {
		for {
			n := uint16(packetCounter.Add(1))
			if n != 0 {
				return n
			}
		}
	}
	versionID := nextID()
	if writeFrame(c, LobbyFrame{Sender: SenderServer, Kind: 1, Command: cmdVersion, PacketID: versionID}) != nil {
		return
	}
	heartbeat := &nativeHeartbeat{}
	go heartbeat.run(notifyCtx, c, nextID, s.engine.KDDIHeartbeatPayload(s.game), 30*time.Second)
	selected := false
	versionChecked := false
	for {
		c.SetReadDeadline(time.Now().Add(90 * time.Second))
		f, err := readFrame(c)
		if err != nil || f.Sender != 0x81 || f.Error != 0 {
			return
		}
		reply := LobbyFrame{Sender: SenderServer, Kind: KindTell, Command: f.Command, PacketID: f.PacketID}
		switch f.Command {
		case 0x6006:
			if f.Kind != 1 || len(f.Payload) != 0 {
				return
			}
			// Explicit logout differs from a lobby socket closing during game
			// handoff: revoke this session and leave its party, preserving accounts.
			if writeFrame(c, reply) != nil {
				return
			}
			s.engine.Logout(actor.ID)
			return
		case CmdKeepAlive:
			// The authenticated TCP response proves liveness; legacy BioServer does
			// not interpret its optional body or correlate the client's own counter.
			if f.Kind == 2 {
				heartbeat.awaiting.Store(false)
				continue
			}
			if len(f.Payload) != 0 {
				return
			}
			if f.Kind != 1 {
				return
			}
		case cmdVersion:
			if versionChecked || f.Kind != 2 || !s.engine.KDDIVersionReplyAllowed(s.game, versionID, f.PacketID, f.Payload) {
				return
			}
			versionChecked = true
			b, err := hnList(actor)
			if err != nil {
				return
			}
			reply.Kind = 0x10
			reply.Command = cmdHNList
			reply.PacketID = nextID()
			reply.Payload = b
		case 0x61a0:
			reply.Payload = []byte{0, 9, 1, 0x2c, 0, 0, 2, 0x58}
		case 0x61a1:
			reply.Payload = []byte{0, 0, 3, 0x84, 0, 0, 7, 8, 0, 0}
		case 0x600e:
			b, _, err := decodeNativeField(f.Payload, f.PacketID)
			if err != nil || len(b) == 0 {
				return
			}
			reply.Payload = []byte{0, 1, b[0]}
		case cmdHNSelect:
			if !versionChecked || selected || f.Kind != 1 {
				return
			}
			handle, n, err := decodeNativeField(f.Payload, f.PacketID)
			if err != nil {
				return
			}
			nick, used, err := decodeNativeField(f.Payload[n:], f.PacketID)
			if err != nil || n+used != len(f.Payload) || len(nick) > 16 {
				return
			}
			nickname, err := japanese.ShiftJIS.NewDecoder().String(string(nick))
			if err != nil {
				return
			}
			canonical, err := japanese.ShiftJIS.NewEncoder().String(nickname)
			if err != nil || canonical != string(nick) {
				return
			}
			actor, err = s.engine.SelectKDDIHandle(actor.ID, string(handle), nickname)
			if err != nil {
				return
			}
			reply.Payload = append([]byte{0, 6}, handle...)
			if writeFrame(c, reply) != nil {
				return
			}
			if s.engine.HasScenarioRound(actor.ID) {
				if writeFrame(c, LobbyFrame{Sender: SenderServer, Kind: 1, Command: 0x6138, PacketID: nextID()}) != nil {
					return
				}
			}
			if writeFrame(c, LobbyFrame{Sender: SenderServer, Kind: 0x10, Command: 0x6104, PacketID: nextID()}) != nil {
				return
			}
			selected = true
			s.watchNativeRoom(notifyCtx, c, actor.ID, nextID)
			continue
		case 0x6138:
			if !selected || s.reports == nil {
				return
			}
			report := reports.Envelope{Game: s.game, Protocol: "kddi", Command: f.Command, SessionID: actor.ID, OwnerID: actor.Account.UserID, NativeHandle: actor.Account.Handle, PacketID: f.PacketID, Payload: append([]byte(nil), f.Payload...), NativeFrame: append([]byte(nil), f.raw...), ReceivedAt: time.Now().UTC()}
			if s.game == world.GameFile2 {
				if fields, err := DecodeFile2Postgame(f); err == nil {
					report.JapaneseFile2 = &fields
				}
			}
			if err := s.reports.Archive(report); err != nil {
				return
			}
			// Native POSTGAMEINFO is a TELL to the server query, not a score ACK.
			continue
		case 0x6190:
			if !selected {
				return
			}
			profile, _, err := decodeNativeField(f.Payload, f.PacketID)
			if err != nil || s.engine.SetProfile(actor.ID, profile) != nil {
				return
			}
		default:
			if !selected {
				return
			}
			s.handleLobbyFrame(c, f, actor.ID)
			continue
		}
		if writeFrame(c, reply) != nil {
			return
		}
	}
}
