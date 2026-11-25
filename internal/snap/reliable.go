// Reliable transport rules ported from openSNAP core/sessions.py and udp_server.py.
// See THIRD_PARTY_NOTICES.md (AGPL-3.0). This is the Outbreak path, not the
// separately paced sender required by some other SNAP titles.
package snap

import (
	"errors"
	"net"
	"sync"
	"time"
)

var ErrReliableBackpressure = errors.New("snap: reliable queue full")

const reliableWindow = 16
const reliableQueueLimit = 1024
const initialRTT = 1600 * time.Millisecond

type pendingPacket struct {
	message  Message
	sent     time.Time
	attempts int
}
type reliablePeer struct {
	lastInbound, keepaliveSent time.Time
	id                         uint32
	addr                       net.Addr
	rtt                        time.Duration
	initialized                bool
	next                       uint32
	held                       map[uint32][]Message
	pending                    map[uint32]pendingPacket
	queued                     []Message
}
type reliableTransport struct {
	clock func() time.Time
	mu    sync.Mutex
	peers map[string]*reliablePeer
}

func newReliableTransport() *reliableTransport {
	return &reliableTransport{clock: time.Now, peers: make(map[string]*reliablePeer)}
}
func peerKey(remote net.Addr, id uint32) string {
	return remote.Network() + ":" + remote.String() + ":" + string([]byte{byte(id >> 24), byte(id >> 16), byte(id >> 8), byte(id)})
}
func (r *reliableTransport) peer(key string) *reliablePeer {
	p := r.peers[key]
	if p == nil {
		p = &reliablePeer{held: make(map[uint32][]Message), pending: make(map[uint32]pendingPacket)}
		r.peers[key] = p
	}
	return p
}
func copyMessages(ms []Message) []Message {
	out := append([]Message(nil), ms...)
	for i := range out {
		out[i].Payload = append([]byte(nil), out[i].Payload...)
	}
	return out
}

// receive treats the outer datagram as one sequence, including all multi children.
// A reverse ACK is meaningful only on the outer response header. A full held
// queue is not acknowledged, allowing the sender to retry instead of losing work.
func (r *reliableTransport) receive(remote net.Addr, ms []Message) (deliver []Message, ack bool, flush []Message) {
	return r.receiveAdmitted(remote, ms, nil)
}
func (r *reliableTransport) receiveAdmitted(remote net.Addr, ms []Message, admit func(net.Addr, []Message) bool) (deliver []Message, ack bool, flush []Message) {
	if len(ms) == 0 {
		return
	}
	m := ms[0]
	r.mu.Lock()
	defer r.mu.Unlock()
	p := r.peer(peerKey(remote, m.SessionID))
	now := r.clock()
	p.lastInbound = now
	p.id = m.SessionID
	p.addr = remote
	if p.rtt == 0 {
		p.rtt = initialRTT
	}
	if m.TypeFlags&FlagResponse != 0 {
		if old, ok := p.pending[m.AckNumber]; ok && old.attempts == 0 {
			sample := now.Sub(old.sent)
			if sample < 17*time.Millisecond {
				sample = 17 * time.Millisecond
			}
			if sample > 32*time.Second {
				sample = 32 * time.Second
			}
			p.rtt = sample
		}
		delete(p.pending, m.AckNumber)
		flush = flushReady(p, now)
	}
	if m.TypeFlags&FlagReliable == 0 {
		candidate := copyMessages(ms)
		if admit != nil && !admit(remote, candidate) {
			return nil, false, flush
		}
		return candidate, false, flush
	}
	if !p.initialized {
		p.initialized = true
		p.next = m.PacketSeq
	}
	delta := int32(m.PacketSeq - p.next)
	if delta < 0 {
		return nil, true, flush
	}
	if delta > 0 {
		if _, ok := p.held[m.PacketSeq]; ok {
			return nil, true, flush
		}
		if len(p.held) >= reliableQueueLimit {
			return nil, false, flush
		}
		p.held[m.PacketSeq] = copyMessages(ms)
		return nil, true, flush
	}
	if admit != nil {
		candidate := copyMessages(ms)
		if held, exists := p.held[p.next]; exists {
			candidate = copyMessages(held)
		}
		if !admit(remote, candidate) {
			if _, exists := p.held[p.next]; !exists {
				if len(p.held) >= reliableQueueLimit {
					return nil, false, flush
				}
				p.held[p.next] = copyMessages(ms)
			}
			return nil, true, flush // the bounded transport queue now owns this packet
		}
		delete(p.held, p.next)
		p.next++
		return candidate, true, flush
	}
	deliver = copyMessages(ms)
	p.next++
	for {
		held, ok := p.held[p.next]
		if !ok {
			break
		}
		delete(p.held, p.next)
		deliver = append(deliver, held...)
		p.next++
	}
	return deliver, true, flush
}

// takeReady retries transport-owned ordered batches after destination backpressure.
// Caller dispatches each group before asking again, preserving status transitions.
func (r *reliableTransport) takeReady(admit func(net.Addr, []Message) bool) (net.Addr, []Message) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range r.peers {
		held, ok := p.held[p.next]
		if !ok || p.addr == nil {
			continue
		}
		candidate := copyMessages(held)
		if !admit(p.addr, candidate) {
			continue
		}
		delete(p.held, p.next)
		p.next++
		return p.addr, candidate
	}
	return nil, nil
}

func flushReady(p *reliablePeer, now time.Time) []Message {
	var out []Message
	for len(p.queued) > 0 {
		m := p.queued[0]
		if !windowAllows(p, m.PacketSeq) {
			break
		}
		p.queued = p.queued[1:]
		p.pending[m.PacketSeq] = pendingPacket{message: m, sent: now}
		out = append(out, m)
	}
	return out
}
func windowAllows(p *reliablePeer, seq uint32) bool {
	if len(p.pending) >= reliableWindow {
		return false
	}
	for n := range p.pending {
		if int32(seq-n) >= reliableWindow {
			return false
		}
	}
	return true
}
func (r *reliableTransport) outbound(remote net.Addr, m Message, now time.Time) (bool, error) {
	if m.TypeFlags&FlagReliable == 0 {
		return true, nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.outboundLocked(remote, m, now)
}
func (r *reliableTransport) outboundAllocated(remote net.Addr, m *Message, now time.Time, allocate func() (uint32, bool)) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	recipient := m.SessionID
	if m.RouteSessionID != 0 {
		recipient = m.RouteSessionID
	}
	if m.TypeFlags&FlagReliable != 0 {
		p := r.peer(peerKey(remote, recipient))
		if len(p.queued) >= reliableQueueLimit {
			return false, ErrReliableBackpressure
		}
	}
	seq, ok := allocate()
	if !ok {
		return false, errors.New("snap: native binding unavailable")
	}
	m.PacketSeq = seq
	if m.TypeFlags&FlagReliable == 0 {
		return true, nil
	}
	return r.outboundLocked(remote, *m, now)
}
func (r *reliableTransport) outboundLocked(remote net.Addr, m Message, now time.Time) (bool, error) {
	recipient := m.SessionID
	if m.RouteSessionID != 0 {
		recipient = m.RouteSessionID
	}
	p := r.peer(peerKey(remote, recipient))
	p.id = recipient
	p.addr = remote
	if p.rtt == 0 {
		p.rtt = initialRTT
	}
	m = copyMessages([]Message{m})[0]
	if len(p.queued) > 0 || !windowAllows(p, m.PacketSeq) {
		if len(p.queued) >= reliableQueueLimit {
			return false, ErrReliableBackpressure
		}
		p.queued = append(p.queued, m)
		return false, nil
	}
	p.pending[m.PacketSeq] = pendingPacket{message: m, sent: now}
	return true, nil
}
func (r *reliableTransport) resend(pc net.PacketConn, bindings *BindingTable, now time.Time) {
	type addressed struct {
		addr net.Addr
		m    Message
	}
	var work []addressed
	r.mu.Lock()
	for key, p := range r.peers {
		b, ok := bindings.Get(p.id)
		if !ok || b.Addr == nil || peerKey(b.Addr, b.ID) != key {
			delete(r.peers, key)
			continue
		}
		remote := p.addr
		for seq, v := range p.pending {
			if now.Sub(v.sent) >= p.rtt+17*time.Millisecond+time.Duration(v.attempts)*200*time.Millisecond+time.Duration((HeaderSize+len(v.message.Payload)+131)/100)*time.Millisecond {
				v.sent = now
				v.attempts++
				p.pending[seq] = v
				work = append(work, addressed{remote, v.message})
			}
		}
	}
	r.mu.Unlock()
	for _, v := range work {
		_ = sendMsg(pc, v.addr, v.m)
	}
}

func (r *reliableTransport) lastACK(remote net.Addr, id uint32) uint32 {
	r.mu.Lock()
	defer r.mu.Unlock()
	p := r.peers[peerKey(remote, id)]
	if p == nil || !p.initialized {
		return 0
	}
	return p.next - 1
}

// One reliable empty keepalive per silence, as in the native SDK reference.
// Its absence of an ACK never expires a session by itself.
func (r *reliableTransport) keepAliveDue(remote net.Addr, id uint32, now time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	p := r.peers[peerKey(remote, id)]
	if p == nil || p.lastInbound.IsZero() || now.Sub(p.lastInbound) < 20*time.Second || p.keepaliveSent.After(p.lastInbound) {
		return false
	}
	p.keepaliveSent = now
	return true
}

// KICS is handled before the established receive window in the reference.
// The first application packet may therefore restart its reliable counter.
func (r *reliableTransport) restartIncoming(remote net.Addr, id uint32) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p := r.peer(peerKey(remote, id))
	p.initialized = false
	p.next = 0
	p.held = make(map[uint32][]Message)
}
