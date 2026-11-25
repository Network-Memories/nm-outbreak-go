package file2

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestCapturedWesternTextAndControlSeparation(t *testing.T) {
	for _, fixture := range []struct {
		hex, text string
		after     bool
	}{
		{"040005070000616161616161616161616161", "aaaaaaa", false},
		{"0c000505000061616161616161616161", "aaaaa", true},
	} {
		p, _ := hex.DecodeString(fixture.hex)
		text, after, err := DecodeWesternChat(p, "aaaaa")
		if err != nil || text != fixture.text || after != fixture.after {
			t.Fatal("captured original", err, text, after)
		}
		out, err := EncodeWesternChat("aaaaa", text, after)
		if err != nil || !bytes.Equal(out, p) {
			t.Fatal("native envelope changed")
		}
		if _, _, err = DecodeWesternChat(p, "imposter"); err == nil {
			t.Fatal("claimed sender trusted")
		}
		if _, _, err = DecodeWesternChat(append(p, 0), "aaaaa"); err == nil {
			t.Fatal("trailing opaque data treated as text")
		}
	}
	if IsWesternChatEnvelope([]byte{2, 0, 5, 1, 0, 0, 97, 97, 97, 97, 97, 49}) {
		t.Fatal("area text confused with room text")
	}
	if IsWesternChatEnvelope(make([]byte, 20)) {
		t.Fatal("native control classified as text")
	}
}
func TestCapturedJapaneseFullwidthTextAndWesternRendering(t *testing.T) {
	p, _ := hex.DecodeString("82838283828382838283") // original capture30 CHATIN field
	text, err := DecodeJapaneseChatText(p)
	if err != nil || text != "ｃｃｃｃｃ" {
		t.Fatal("captured SJIS", err, text)
	}
	out, err := EncodeWesternChat("ccccc", text, true)
	if err != nil || string(out[11:]) != "ccccc" {
		t.Fatal("Latin full width rendering", err)
	}
	native, err := EncodeJapaneseChat("ABC123", "ccccc", text)
	if err != nil {
		t.Fatal(err)
	}
	want := append([]byte{0, 6, 'A', 'B', 'C', '1', '2', '3', 0, 5, 'c', 'c', 'c', 'c', 'c', 0, 10}, p...)
	want = append(want, 0, 0, 0, 0, 255)
	if !bytes.Equal(native, want) {
		t.Fatal("CHATOUT reference envelope changed")
	}
	for _, bad := range [][]byte{{0x82}, {0x82, 0x20}, {0}, {10}} {
		if _, err := DecodeJapaneseChatText(bad); err == nil {
			t.Fatal("malformed/control text accepted", bad)
		}
	}
	if _, err := EncodeWesternChat("ccccc", "日本語", true); err == nil {
		t.Fatal("unmeasured Western glyphs replaced silently")
	}
	if _, err := EncodeWesternChat("ccccc", string(bytes.Repeat([]byte{'c'}, 65)), true); err == nil {
		t.Fatal("text silently truncated")
	}
}

func TestCapturedAreaTextIsSeparateAndDoesNotMutateControlData(t *testing.T) {
	p, _ := hex.DecodeString("020005010000616161616131") // historical original area probe
	before := append([]byte(nil), p...)
	if !IsWesternAreaChatEnvelope(p) || IsWesternChatEnvelope(p) {
		t.Fatal("area and party scopes overlap")
	}
	text, err := DecodeWesternAreaChat(p, "aaaaa")
	if err != nil || text != "1" || !bytes.Equal(p, before) {
		t.Fatal("original area envelope changed", err)
	}
	out, err := EncodeWesternAreaChat("aaaaa", text)
	if err != nil || !bytes.Equal(out, p) {
		t.Fatal("area native output differs")
	}
	if _, err = DecodeWesternAreaChat(p, "imposter"); err == nil {
		t.Fatal("forged native name admitted")
	}
	if IsWesternAreaChatEnvelope(append(p, 0)) || IsWesternAreaChatEnvelope([]byte{2, 0, 0, 0, 0, 0}) {
		t.Fatal("opaque token classified as text")
	}
	if IsWesternAreaChatEnvelope([]byte{4, 0, 5, 1, 0, 0, 97, 97, 97, 97, 97, 49}) {
		t.Fatal("room04 interpreted as area02")
	}
}
