package snaplaunch

import (
	"encoding/binary"
	"testing"
)

func TestDecoderNativeSelectorLengthsAndOwnedBuffer(t *testing.T) {
	p := make([]byte, 14)
	binary.BigEndian.PutUint32(p, 1)
	binary.BigEndian.PutUint32(p[4:], 42)
	binary.LittleEndian.PutUint32(p[8:], 5)
	copy(p[12:], []byte{1, 2})
	m, known, err := Decode(p)
	if err != nil || !known || m.Target != 42 || m.Tag != 5 {
		t.Fatal(m, known, err)
	}
	p[12] = 9
	if m.Body[0] != 1 {
		t.Fatal("decoder aliases packet")
	}
	if _, known, err = Decode(p[:13]); !known || err == nil {
		t.Fatal("truncated05 accepted")
	}
	binary.LittleEndian.PutUint32(p[8:], 14)
	if _, known, err = Decode(p); known || err != nil {
		t.Fatal("unrelated application message consumed")
	}
}
func TestHandshakeRefusesMissingAndDuplicatedChunks(t *testing.T) {
	g := New(42)
	feed := func(tag uint32, b []byte) error { return g.Feed(Message{42, tag, b}) }
	if feed(5, []byte{1, 2}) == nil {
		t.Fatal("05 before readiness")
	}
	if feed(3, nil) != nil || g.Acknowledge() != nil || feed(5, []byte{1, 2}) != nil {
		t.Fatal("initial handshake")
	}
	for i := 0; i < 2; i++ {
		b := make([]byte, 34)
		b[0] = byte(i)
		if feed(6, b) != nil {
			t.Fatal("name")
		}
	}
	b := make([]byte, 54)
	b[1] = 50
	if feed(7, b) != nil {
		t.Fatal("first chunk")
	}
	if feed(7, b) == nil || feed(8, nil) == nil {
		t.Fatal("duplicate chunk or early finish accepted")
	}
	b[2] = 50
	if feed(7, b) != nil {
		t.Fatal("rejection corrupted expected offset")
	}
	b[2] = 200
	b[1] = 40
	b[53] = 1
	if feed(7, b) == nil {
		t.Fatal("bad padding accepted")
	}
}
func FuzzNativeLaunchDecoder(f *testing.F) {
	f.Add([]byte{0, 0, 0, 1, 0, 0, 0, 42, 3, 0, 0, 0})
	f.Fuzz(func(t *testing.T, p []byte) {
		m, known, err := Decode(p)
		if err == nil && known {
			g := New(m.Target)
			_ = g.Feed(m)
		}
	})
}
