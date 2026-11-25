package stun

import (
	"encoding/binary"
	"net"
	"testing"
	"time"
)

func mappedResponse(transaction []byte) []byte {
	out := make([]byte, 32)
	binary.BigEndian.PutUint16(out, bindingResponse)
	binary.BigEndian.PutUint16(out[2:], 12)
	binary.BigEndian.PutUint32(out[4:], magicCookie)
	copy(out[8:], transaction)
	binary.BigEndian.PutUint16(out[20:], 0x0020)
	binary.BigEndian.PutUint16(out[22:], 8)
	out[25] = 1
	binary.BigEndian.PutUint16(out[26:], 32000^uint16(magicCookie>>16))
	for i, c := range []byte{203, 0, 113, 7} {
		out[28+i] = c ^ out[4+i]
	}
	return out
}
func TestValidateSTUNMapping(t *testing.T) {
	transaction := []byte("0123456789ab")
	raw := mappedResponse(transaction)
	mapped, err := parseBindingResponse(raw, transaction)
	if err != nil || mapped != "203.0.113.7:32000" {
		t.Fatal(mapped, err)
	}
	for _, mutate := range []func([]byte) []byte{
		func(b []byte) []byte { return b[:20] },
		func(b []byte) []byte { b[2] = 1; return b },
		func(b []byte) []byte { b[8] ^= 1; return b },
		func(b []byte) []byte { b[23] = 99; return b },
		func(b []byte) []byte { b[25] = 99; return b },
	} {
		bad := mutate(append([]byte(nil), raw...))
		if _, err := parseBindingResponse(bad, transaction); err == nil {
			t.Fatal("invalid STUN response accepted")
		}
	}
}
func TestProbeActualUDPAndOutage(t *testing.T) {
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 128)
		n, remote, err := pc.ReadFrom(buf)
		if err == nil && n >= 20 {
			pc.WriteTo(mappedResponse(buf[8:20]), remote)
		}
	}()
	link := NewLink(pc.LocalAddr().String(), time.Second, 50*time.Millisecond)
	if err := link.Probe(); err != nil {
		t.Fatal(err)
	}
	status := link.Status()
	if !status.Healthy || status.MappedAddress == "" || status.RelayEnabled || status.GameTransportVerified {
		t.Fatal("probe fabricated transport/relay readiness")
	}
	<-done
	pc.Close()
	if err := link.Probe(); err == nil {
		t.Fatal("outage ignored")
	}
	status = link.Status()
	if status.Healthy || status.LastSuccess.IsZero() || status.RelayState != "not_implemented" {
		t.Fatal("outage did not preserve explicit degraded state")
	}
}
