package snap

import (
	"context"
	"log"
	"net"
	"networkmemories/nm-outbreak-go/internal/stun"
	"os"
	"strconv"
	"sync"
	"time"

	"networkmemories/nm-outbreak-go/internal/core"
	"networkmemories/nm-outbreak-go/internal/world"
)

// Server handles SNAP UDP datagrams for bootstrap (9090), File #1 (9091), or File #2 (9092).
type Server struct {
	dispatchMu          sync.Mutex
	engineWatchers      map[string]string
	started             bool
	notifyDone          <-chan struct{}
	notifyMu            sync.Mutex
	notifications       map[string]*core.RoomSubscription
	trace               bool
	nativeControl       string
	nativeBackupControl string
	native              *stun.NativeConn
	name                string
	game                world.Game
	addr                string
	engine              *core.Engine
	bindings            *BindingTable
	reliable            *reliableTransport
}

func NewTransport(name string, game world.Game, host string, port int, engine *core.Engine) *Server {
	return &Server{trace: os.Getenv("NM_OUTBREAK_TRACE_COMMANDS") == "1", name: name, game: game, addr: net.JoinHostPort(host, strconv.Itoa(port)), engine: engine,
		bindings: NewBindingTable(), reliable: newReliableTransport()}
}

func (s *Server) Start(ctx context.Context) error {
	s.started = true
	s.notifyDone = ctx.Done()
	var pc net.PacketConn
	var err error
	if s.nativeControl != "" {
		host, port, _ := net.SplitHostPort(s.addr)
		number, _ := strconv.Atoi(port)
		s.native, err = stun.ListenNativeWithBackup(ctx, s.nativeControl, s.nativeBackupControl, host, number)
		if err == nil {
			pc = s.native
		}
	} else {
		pc, err = net.ListenPacket("udp", s.addr)
	}
	if err != nil {
		return err
	}
	log.Printf("[nm.snap.%s] UDP socket bound at %s", s.name, s.addr)

	go func() {
		<-ctx.Done()
		_ = pc.Close()
	}()

	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				s.flushAccepted(pc)
				s.reliable.resend(pc, s.bindings, now)
				s.keepAlive(pc, now)
			}
		}
	}()
	go func() {
		buf := make([]byte, 2048)
		for {
			n, remote, err := pc.ReadFrom(buf)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				continue
			}
			messages, err := DecodeMessages(buf[:n])
			if err != nil {
				continue
			}
			s.handleMessages(pc, remote, messages)
		}
	}()
	return nil
}

func (s *Server) handleMessage(pc net.PacketConn, remote net.Addr, msg Message) {
	s.handleMessages(pc, remote, []Message{msg})
}
func (s *Server) handleMessages(pc net.PacketConn, remote net.Addr, messages []Message) {
	// Multiple independent top-level messages have separate transport sequences.
	for i := 0; i < len(messages); {
		j := i + 1
		for j < len(messages) && messages[j].EmbeddedInMulti {
			j++
		}
		s.handleGroup(pc, remote, messages[i:j])
		i = j
	}
}
func (s *Server) handleGroup(pc net.PacketConn, remote net.Addr, messages []Message) {
	s.dispatchMu.Lock()
	defer s.dispatchMu.Unlock()
	if len(messages) == 0 {
		return
	}
	msg := messages[0]
	originalID := msg.SessionID
	for _, m := range messages[1:] {
		if !m.EmbeddedInMulti || m.SessionID != originalID {
			return
		}
	}
	if remote == nil {
		return
	}
	if s.name == "bootstrap" {
		for _, m := range messages {
			s.handleBootstrap(pc, remote, m)
		}
		return
	}
	if msg.Command == CmdLoginToKics {
		binding, exists := s.bindings.Get(msg.SessionID)
		if !exists || binding.Addr == nil || binding.Addr.String() != remote.String() {
			var ok bool
			msg, ok = s.adoptNativeHandoff(remote, msg)
			if !ok {
				return
			}
		}
	}
	if s.trace {
		log.Printf("[nm.snap.%s] cmd=%02x flags=%04x seq=%d ack=%d bytes=%d", s.name, msg.Command, msg.TypeFlags, msg.PacketSeq, msg.AckNumber, len(msg.Payload))
	}
	b, ok := s.bindings.NativeContext(remote, msg.SessionID)
	if !ok {
		return
	}
	if _, ok := s.engine.Session(b.CoreID); !ok {
		return
	}
	s.engine.MarkAlive(b.CoreID)
	if s.started {
		s.watchRoom(pc, b.CoreID)
	}
	msg.SessionID = b.ID
	messages = append([]Message(nil), messages...)
	messages[0] = msg
	for i := 1; i < len(messages); i++ {
		messages[i].SessionID = b.ID
	}
	deliver, ack, flush := s.reliable.receiveAdmitted(remote, messages, s.admitEngineGroup)
	for _, m := range flush {
		_ = sendMsg(pc, remote, m)
	}
	for _, m := range deliver {
		s.dispatchCommand(pc, remote, m)
	}
	s.flushAcceptedLocked(pc)
	if ack {
		_ = sendMsg(pc, remote, Message{TypeFlags: msg.TypeFlags&FlagChannelBits | FlagResponse, Command: CmdAck, SessionID: msg.SessionID, AckNumber: msg.PacketSeq})
	}
}

// WithNativeControl selects the shared NM transport before Start; there is no
// silent direct-socket fallback if registration or the service fails.
func (s *Server) WithNativeControl(path string) *Server { s.nativeControl = path; return s }
func (s *Server) WithNativeBackupControl(path string) *Server {
	s.nativeBackupControl = path
	return s
}
func (s *Server) NativeStatus() stun.NativeStatus {
	if s.native == nil {
		return stun.NativeStatus{LastError: "native-control-not-configured"}
	}
	return s.native.Status()
}

func (s *Server) Role() string { return s.name }

func (s *Server) keepAlive(pc net.PacketConn, now time.Time) {
	if s.name == "bootstrap" {
		return
	}
	for _, b := range s.bindings.Snapshot() {
		if b.Addr == nil {
			continue
		}
		if _, ok := s.engine.Session(b.CoreID); !ok {
			continue
		}
		if s.reliable.keepAliveDue(b.Addr, b.ID, now) {
			_ = s.sendReply(pc, b.Addr, Message{TypeFlags: FlagReliable, Command: CmdAck, SessionID: b.ID})
		}
	}
}

func (s *Server) flushAccepted(pc net.PacketConn) {
	s.dispatchMu.Lock()
	defer s.dispatchMu.Unlock()
	s.flushAcceptedLocked(pc)
}
func (s *Server) flushAcceptedLocked(pc net.PacketConn) {
	// Bound work per tick; a saturated peer cannot monopolize the listener.
	for i := 0; i < 64; i++ {
		remote, group := s.reliable.takeReady(s.admitEngineGroup)
		if len(group) == 0 {
			return
		}
		for _, m := range group {
			s.dispatchCommand(pc, remote, m)
		}
	}
}
