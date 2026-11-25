package snap

import (
	"bytes"
	"net"
	"testing"
	"time"
)

func TestReliableOrderingMultiAndDuplicates(t *testing.T) {
	r := newReliableTransport()
	addr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 31001}
	m := func(n uint32) Message {
		return Message{SessionID: 42, TypeFlags: FlagReliable, PacketSeq: n, Payload: []byte{byte(n)}}
	}
	got, ack, _ := r.receive(addr, []Message{m(0)})
	if len(got) != 1 || !ack {
		t.Fatal("first record missing")
	}
	ahead := []Message{m(2), {SessionID: 42, EmbeddedInMulti: true, Payload: []byte{99}}}
	got, ack, _ = r.receive(addr, ahead)
	if len(got) != 0 || !ack {
		t.Fatal("ahead packet not held and ACKed")
	}
	ahead[1].Payload[0] = 1
	got, ack, _ = r.receive(addr, []Message{m(2)})
	if len(got) != 0 || !ack {
		t.Fatal("duplicate held packet dispatched")
	}
	got, ack, _ = r.receive(addr, []Message{m(1)})
	if len(got) != 3 || !ack || got[0].PacketSeq != 1 || got[1].PacketSeq != 2 || got[2].Payload[0] != 99 {
		t.Fatal("gap flush lost ordering, multi child or isolation")
	}
	got, ack, _ = r.receive(addr, []Message{m(0)})
	if len(got) != 0 || !ack {
		t.Fatal("old record dispatched twice")
	}
	// The reliable counter wraps; serial arithmetic must retain ordering.
	r = newReliableTransport()
	r.receive(addr, []Message{m(^uint32(0))})
	got, ack, _ = r.receive(addr, []Message{m(0)})
	if len(got) != 1 || !ack {
		t.Fatal("wrapped sequence rejected")
	}
}
func TestReliableExactACKAndFlightWindow(t *testing.T) {
	r := newReliableTransport()
	addr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 31002}
	other := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 31003}
	for n := uint32(0); n < 18; n++ {
		send, err := r.outbound(addr, Message{SessionID: 42, TypeFlags: FlagReliable, PacketSeq: n}, time.Now())
		if err != nil || send != (n < 16) {
			t.Fatalf("flight slot %d: %v %v", n, send, err)
		}
	}
	// An unrelated endpoint, session, non-response, or child's garbage ACK cannot retire it.
	r.receive(other, []Message{{SessionID: 42, TypeFlags: FlagResponse, AckNumber: 0}})
	r.receive(addr, []Message{{SessionID: 43, TypeFlags: FlagResponse, AckNumber: 0}})
	r.receive(addr, []Message{{SessionID: 42, AckNumber: 0}, {SessionID: 42, EmbeddedInMulti: true, TypeFlags: FlagResponse, AckNumber: 0}})
	_, _, flush := r.receive(addr, []Message{{SessionID: 42, TypeFlags: FlagResponse, AckNumber: 15}})
	if len(flush) != 0 {
		t.Fatal("hole at oldest pending sequence was skipped")
	}
	_, _, flush = r.receive(addr, []Message{{SessionID: 42, TypeFlags: FlagResponse, AckNumber: 0}})
	if len(flush) != 1 || flush[0].PacketSeq != 16 {
		t.Fatal("exact ACK failed to release one flight slot")
	}
	_, _, flush = r.receive(addr, []Message{{SessionID: 42, TypeFlags: FlagResponse, AckNumber: 0}})
	if len(flush) != 0 {
		t.Fatal("replayed ACK released another slot")
	}
}
func TestReliableRetransmitOriginalBytesUntilACK(t *testing.T) {
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer pc.Close()
	client, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	bindings := NewBindingTable()
	bindings.Put(&Binding{ID: 42, Addr: client.LocalAddr()})
	r := newReliableTransport()
	start := time.Now()
	m := Message{SessionID: 42, TypeFlags: FlagReliable | FlagResponse, PacketSeq: 3, AckNumber: 9, Command: 0x28, Payload: []byte{1, 2, 3}}
	send, err := r.outbound(client.LocalAddr(), m, start)
	if err != nil || !send {
		t.Fatal("initial packet deferred")
	}
	m.Payload[0] = 99
	r.resend(pc, bindings, start.Add(time.Second))
	client.SetReadDeadline(time.Now().Add(10 * time.Millisecond))
	buf := make([]byte, 2048)
	if _, _, err := client.ReadFrom(buf); err == nil {
		t.Fatal("resent before native initial round trip estimate")
	}
	r.resend(pc, bindings, start.Add(2*time.Second))
	client.SetReadDeadline(time.Now().Add(time.Second))
	n, _, err := client.ReadFrom(buf)
	if err != nil {
		t.Fatal(err)
	}
	m.Payload[0] = 1
	if !bytes.Equal(buf[:n], EncodeDatagram(m)) {
		t.Fatal("retransmit changed sequence, payload or ACK")
	}
	r.receive(client.LocalAddr(), []Message{{SessionID: 42, TypeFlags: FlagResponse, AckNumber: 3}})
	r.resend(pc, bindings, start.Add(5*time.Second))
	client.SetReadDeadline(time.Now().Add(10 * time.Millisecond))
	if _, _, err := client.ReadFrom(buf); err == nil {
		t.Fatal("ACKed packet still retransmitted")
	}
	bindings.Delete(42)
	r.resend(pc, bindings, start.Add(6*time.Second))
	if len(r.peers) != 0 {
		t.Fatal("dead peer retained")
	}
}

func TestRelayACKIsOwnedByRecipientNotHeaderSender(t *testing.T) {
	r := newReliableTransport()
	remote := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 31004}
	m := Message{SessionID: 42, RouteSessionID: 88, TypeFlags: FlagReliable, PacketSeq: 3}
	if send, err := r.outbound(remote, m, time.Now()); err != nil || !send {
		t.Fatal(err)
	}
	r.receive(remote, []Message{{SessionID: 42, TypeFlags: FlagResponse, AckNumber: 3}})
	if len(r.peers[peerKey(remote, 88)].pending) != 1 {
		t.Fatal("source header retired receiver's reliable work")
	}
	r.receive(remote, []Message{{SessionID: 88, TypeFlags: FlagResponse, AckNumber: 3}})
	if len(r.peers[peerKey(remote, 88)].pending) != 0 {
		t.Fatal("recipient ACK did not retire relayed work")
	}
}

func TestReliableAdmissionRetainsCongestedCurrentAndOrderedHeldGroups(t *testing.T) {
	r := newReliableTransport()
	addr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 32001}
	msg := func(seq uint32) Message {
		return Message{SessionID: 42, PacketSeq: seq, TypeFlags: FlagReliable, Payload: []byte{byte(seq)}}
	}
	allowed := false
	committed := []byte{}
	admit := func(_ net.Addr, ms []Message) bool {
		if !allowed {
			return false
		}
		committed = append(committed, ms[0].Payload[0])
		ms[0].EngineHandled = true
		return true
	}
	if got, ack, _ := r.receiveAdmitted(addr, []Message{msg(1)}, admit); len(got) != 0 || !ack {
		t.Fatal("bounded transport did not retain blocked group")
	}
	if got, ack, _ := r.receiveAdmitted(addr, []Message{msg(3)}, admit); len(got) != 0 || !ack {
		t.Fatal("out-of-order group not held")
	}
	if _, got := r.takeReady(admit); len(got) != 0 {
		t.Fatal("congestion bypassed")
	}
	allowed = true
	_, got := r.takeReady(admit)
	if len(got) != 1 || !got[0].EngineHandled || got[0].PacketSeq != 1 {
		t.Fatal(got)
	}
	if _, got = r.takeReady(admit); len(got) != 0 {
		t.Fatal("sequence hole bypassed")
	}
	got, ack, _ := r.receiveAdmitted(addr, []Message{msg(2)}, admit)
	if len(got) != 1 || !ack {
		t.Fatal(got)
	}
	_, got = r.takeReady(admit)
	if len(got) != 1 || got[0].PacketSeq != 3 {
		t.Fatal(got)
	}
	if got, ack, _ = r.receiveAdmitted(addr, []Message{msg(1)}, admit); len(got) != 0 || !ack {
		t.Fatal("duplicate relayed again")
	}
	if !bytes.Equal(committed, []byte{1, 2, 3}) {
		t.Fatal("ordered engine batch duplicated or lost", committed)
	}
}
func TestReliableOutputCongestionDoesNotConsumeNativeSequence(t *testing.T) {
	r := newReliableTransport()
	addr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 32002}
	p := r.peer(peerKey(addr, 42))
	p.queued = make([]Message, reliableQueueLimit)
	next := uint32(99)
	allocate := func() (uint32, bool) { v := next; next++; return v, true }
	m := Message{SessionID: 42, TypeFlags: FlagReliable}
	if _, err := r.outboundAllocated(addr, &m, time.Now(), allocate); err != ErrReliableBackpressure || next != 99 {
		t.Fatal("failed output consumed sequence", next, err)
	}
	p.queued = p.queued[:len(p.queued)-1]
	if send, err := r.outboundAllocated(addr, &m, time.Now(), allocate); err != nil || send || next != 100 || m.PacketSeq != 99 {
		t.Fatal(send, err, next, m.PacketSeq)
	}
}

func TestReliableCongestedTransportDoesNotAckBeyondOwnedCapacity(t *testing.T) {
	r := newReliableTransport()
	addr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 32003}
	refuse := func(_ net.Addr, _ []Message) bool { return false }
	for i := 0; i < reliableQueueLimit; i++ {
		_, ack, _ := r.receiveAdmitted(addr, []Message{{SessionID: 42, PacketSeq: uint32(i), TypeFlags: FlagReliable}}, refuse)
		if !ack {
			t.Fatal("owned bounded slot not acknowledged", i)
		}
	}
	if _, ack, _ := r.receiveAdmitted(addr, []Message{{SessionID: 42, PacketSeq: reliableQueueLimit, TypeFlags: FlagReliable}}, refuse); ack {
		t.Fatal("unretained packet acknowledged")
	}
	if len(r.peer(peerKey(addr, 42)).held) != reliableQueueLimit {
		t.Fatal("transport exceeded memory limit")
	}
}
