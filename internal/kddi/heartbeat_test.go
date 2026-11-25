package kddi

import (
	"bytes"
	"context"
	"net"
	"testing"
	"time"
)

func TestNativeHeartbeatAndConnectionCheckUseFramedWriter(t *testing.T) {
	a, b := net.Pipe()
	defer b.Close()
	defer a.Close()
	b.SetDeadline(time.Now().Add(time.Second))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h := &nativeHeartbeat{}
	done := make(chan struct{})
	var pid uint16
	payload := []byte{0, 2, 0, 1, 3, 0xe7, 0, 1}
	go func() {
		h.run(ctx, &lobbyFrameConn{Conn: a}, func() uint16 { pid++; return pid }, payload, 5*time.Millisecond)
		close(done)
	}()
	first, err := readFrame(b)
	if err != nil || first.Command != 0x6202 || first.Kind != 16 || !bytes.Equal(first.Payload, payload) {
		t.Fatal(first, err)
	}
	second, err := readFrame(b)
	if err != nil || second.Command != 0x6202 {
		t.Fatal(second, err)
	}
	check, err := readFrame(b)
	if err != nil || check.Command != 0x6001 || check.Kind != 1 || len(check.Payload) != 0 || !h.awaiting.Load() {
		t.Fatal(check, err)
	}
	h.awaiting.Store(false)
	for i := 0; i < 2; i++ {
		if f, err := readFrame(b); err != nil || f.Command != 0x6202 {
			t.Fatal(f, err)
		}
	}
	if f, err := readFrame(b); err != nil || f.Command != 0x6001 {
		t.Fatal(f, err)
	}
	// An unanswered check closes the transport at the following check cycle.
	for i := 0; i < 2; i++ {
		if _, err := readFrame(b); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := readFrame(b); err == nil {
		t.Fatal("missing native connection proof ignored")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("heartbeat worker retained after closure")
	}
}
