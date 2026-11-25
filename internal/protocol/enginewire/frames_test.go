package enginewire

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestNativeEngineBodiesPreservedAndPaddingEscape(t *testing.T) {
	for _, peer := range []int{0, 1} {
		for _, size := range []int{0, 1, 5, 35, 36, 37, JapaneseSplit} { // larger bodies: measured split (split_test.go)
			body := bytes.Repeat([]byte{0xaa}, size)
			source := Frame{peer, body}
			jp, err := JapaneseEncode(source)
			if err != nil {
				t.Fatal(err)
			}
			if size == 36 && (len(jp) != 41 || jp[1] != byte(0x70|peer)) {
				t.Fatal("missing native padding escape")
			}
			if size == 36 {
				jp[40] = 0xff
			} // original receiver discards this opaque transport byte
			decoded, known, err := JapaneseData(jp)
			if err != nil || !known || decoded.Peer != peer || !bytes.Equal(decoded.Body, body) {
				t.Fatal(size, decoded, err)
			}
			west, err := WesternEncode(decoded)
			if err != nil {
				t.Fatal(err)
			}
			frames, err := WesternDecode(west, 2)
			if err != nil || len(frames) != 1 || !bytes.Equal(frames[0].Body, body) {
				t.Fatal(frames, err)
			}
		}
	}
	if _, err := JapaneseEncode(Frame{0, make([]byte, 252)}); err == nil {
		t.Fatal("one-byte native length overflow")
	}
	if _, known, err := JapaneseData([]byte{5, 0x41, 1, 0, 99}); !known || err == nil {
		t.Fatal("mismatched sender word accepted")
	}
	if _, err := WesternDecode([]byte{3, 0, 1}, 2); err == nil {
		t.Fatal("truncated Western body")
	}
}
func TestTransportPhaseNeedsEightDistinctAuthenticProbeEchoes(t *testing.T) {
	s, _ := NewKDDIPeer(1, 0)
	if got := s.Start(); len(got) != 1 || !bytes.Equal(got[0], []byte{3, 0x90, 0}) {
		t.Fatal(got)
	}
	responses, frame, err := s.Receive([]byte{3, 0x91, 0}, 1.25)
	if err != nil || frame != nil || len(responses) != 9 {
		t.Fatal(responses, err)
	}
	var echoes [][]byte
	for _, probe := range responses[1:] {
		echoes = append(echoes, append([]byte{8, 0x31, 0}, probe[2:]...))
	}
	for i := 0; i < 16; i++ {
		out, _, err := s.Receive(echoes[0], 2)
		if err != nil || len(out) != 0 {
			t.Fatal("duplicate echo invented phase2", out, err)
		}
	}
	bad := append([]byte(nil), echoes[1]...)
	bad[7] ^= 1
	if _, _, err := s.Receive(bad, 2); err == nil {
		t.Fatal("echo clock not checked")
	}
	for i := 1; i < 8; i++ {
		out, _, err := s.Receive(echoes[i], 2)
		if err != nil {
			t.Fatal(err)
		}
		if i < 7 && len(out) != 0 {
			t.Fatal("premature phase2")
		}
		if i == 7 && (len(out) != 1 || !bytes.Equal(out[0], []byte{3, 0x90, 2})) {
			t.Fatal(out)
		}
	}
	if out, _, err := s.Receive(echoes[7], 2); err != nil || len(out) != 0 {
		t.Fatal("phase2 repeated")
	}
	p := []byte{7, 0x21, 3, 0, 0, 0, 0}
	binary.LittleEndian.PutUint32(p[3:], 1234)
	out, _, err := s.Receive(p, 3)
	if err != nil || len(out) != 1 || !bytes.Equal(out[0], append([]byte{8, 0x30, 1}, p[2:]...)) {
		t.Fatal(out, err)
	}
	if _, _, err := s.Receive([]byte{2, 0xf0}, 3); err == nil {
		t.Fatal("JP guest impersonated western place")
	}
}
