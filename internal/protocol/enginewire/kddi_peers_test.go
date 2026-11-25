package enginewire

import (
	"bytes"
	"testing"
)

// Local Japanese seat 0, simulated Western seat 1, actual Japanese seat 2
// (record forms from java-f1-jp3-game.pcap).
func TestKDDIPeersSimulateOnlyWesternSeats(t *testing.T) {
	m, err := NewKDDIPeers(0, []int{1})
	if err != nil {
		t.Fatal(err)
	}
	if start := m.Start(); len(start) != 1 || !bytes.Equal(start[0], []byte{3, 0x91, 0}) {
		t.Fatalf("% x", start)
	}
	out, frame, err := m.Receive([]byte{3, 0x90, 0}, 1.5)
	if err != nil || frame != nil || len(out) != 9 || !bytes.Equal(out[0], []byte{3, 0x91, 1}) || out[1][1] != 0x21 {
		t.Fatalf("%v % x", err, out)
	}
	probes := out[1:]
	// Own probe: echoed by the simulated seat with target = local seat.
	out, _, err = m.Receive([]byte{7, 0x20, 3, 1, 2, 3, 4}, 2)
	if err != nil || len(out) != 1 || !bytes.Equal(out[0], []byte{8, 0x31, 0, 3, 1, 2, 3, 4}) {
		t.Fatalf("%v % x", err, out)
	}
	// Echo aimed at the actual Japanese seat is not answered here.
	out, frame, err = m.Receive([]byte{8, 0x30, 2, 0, 9, 9, 9, 9}, 2)
	if err != nil || out != nil || frame != nil {
		t.Fatal("echo for an actual Japanese seat must be left to the raw relay")
	}
	var last [][]byte
	for _, p := range probes {
		echo := append([]byte{8, 0x30, 1}, p[2:]...)
		if last, _, err = m.Receive(echo, 3); err != nil {
			t.Fatal(err)
		}
	}
	if len(last) != 1 || !bytes.Equal(last[0], []byte{3, 0x91, 2}) {
		t.Fatalf("simulated seat did not finish: % x", last)
	}
	if _, _, err = m.Receive([]byte{3, 0x91, 0}, 3); err == nil {
		t.Fatal("record with another sender place accepted")
	}
	if _, err := NewKDDIPeers(0, []int{1, 1}); err == nil {
		t.Fatal("duplicate seat accepted")
	}
}
