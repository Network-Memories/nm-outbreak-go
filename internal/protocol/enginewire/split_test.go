package enginewire

import (
	"bytes"
	"testing"
)

func TestJapaneseSplitRoundTrip(t *testing.T) {
	body := make([]byte, 244)
	for i := range body {
		body[i] = byte(i)
	}
	p, err := JapaneseEncode(Frame{Peer: 1, Body: body})
	if err != nil || len(p) != 200+52 || p[0] != 200 || p[1] != 0x51 || p[2] != 0xf4 || p[3] != 0x10 || p[201] != 0x41 || p[202] != 0x30 || p[203] != 0xf0 {
		t.Fatalf("measured split shape: %x", p[:4])
	}
	var r Reassembler
	if out, err := r.Feed(p[:200]); out != nil || err != nil {
		t.Fatal("head must be held")
	}
	out, err := r.Feed(p[200:])
	if err != nil {
		t.Fatal(err)
	}
	f, ok, err := JapaneseData(out)
	if !ok || err != nil || f.Peer != 1 || !bytes.Equal(f.Body, body) {
		t.Fatal("reassembled body differs")
	}
}
