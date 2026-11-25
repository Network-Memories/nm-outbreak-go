package snap

import (
	"bytes"
	"testing"
)

func TestEncodeDecodeRoundTrip(t *testing.T) {
	in := Message{
		TypeFlags: 0xb000,
		Sequence:  0,
		Command:   0x04,
		SessionID: 0x56d8b9e9,
		PacketSeq: 10,
		AckNumber: 0,
		Payload:   []byte("111\x00"),
	}
	raw := EncodeDatagram(in)
	out, err := DecodeDatagram(raw)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if out.Command != in.Command || out.SessionID != in.SessionID || string(out.Payload) != string(in.Payload) {
		t.Fatalf("mismatch: %+v vs %+v", out, in)
	}
	raw[len(raw)-1] ^= 0xff
	if _, err := DecodeDatagram(raw); err == nil {
		t.Fatalf("expected footer error on corrupted datagram")
	}
}

func TestNativeLiteralEnvelope(t *testing.T) {
	// Header length 20, not a checksum footer; fields follow the native reference.
	raw := []byte{0xb0, 0x14, 0, 4, 0x56, 0xd8, 0xb9, 0xe9, 0, 0, 0, 10, 0, 0, 0, 0, '1', '1', '1', 0, 0xba, 0x47, 0x66, 0x11}
	m, err := DecodeDatagram(raw)
	if err != nil {
		t.Fatal(err)
	}
	if m.TypeFlags != 0xb000 || m.Command != 4 || m.PacketSeq != 10 || string(m.Payload) != "111\x00" {
		t.Fatal("native fields decoded incorrectly")
	}
	encoded, err := EncodeMessages([]Message{m})
	if err != nil || !bytes.Equal(encoded, raw) {
		t.Fatal("native envelope not reproduced", err)
	}
	for _, bad := range [][]byte{raw[:15], append([]byte{0xb0, 0x0f}, raw[2:]...), append([]byte{0xb0, 0x20}, raw[2:]...)} {
		if _, err := DecodeMessages(bad); err == nil {
			t.Fatal("invalid native size accepted")
		}
	}
	if _, err := EncodeMessages([]Message{{Payload: make([]byte, 1008)}}); err == nil {
		t.Fatal("oversized message silently changes type bits")
	}
}

func TestMultiMessageSequences(t *testing.T) {
	raw, err := EncodeMessages([]Message{{TypeFlags: FlagMulti, PacketSeq: 12}, {TypeFlags: FlagRoom, Command: 0x0c, PacketSeq: 99, Payload: []byte{1}}})
	if err != nil {
		t.Fatal(err)
	}
	messages, err := DecodeMessages(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 2 || messages[0].PacketSeq != 12 || messages[1].PacketSeq != 0 || !messages[1].EmbeddedInMulti {
		t.Fatal("multi child used a stale transport sequence")
	}
	if _, err := DecodeDatagram(raw); err == nil {
		t.Fatal("single decoder silently drops embedded messages")
	}
}

func FuzzDecodeNativeDatagram(f *testing.F) {
	f.Add([]byte{0x20, 0x10, 0, 1, 0, 0, 0, 1, 0, 0, 0, 1, 0, 0, 0, 0, 0xba, 0x47, 0x66, 0x11})
	f.Add([]byte{0xba, 0x47, 0x66, 0x11})
	f.Fuzz(func(t *testing.T, raw []byte) {
		messages, err := DecodeMessages(raw)
		if err != nil {
			return
		}
		for _, m := range messages {
			if len(m.Payload) > int(LengthMask)-HeaderSize || m.TypeFlags&LengthMask != 0 {
				t.Fatal("native message bounds violated")
			}
		}
	})
}
