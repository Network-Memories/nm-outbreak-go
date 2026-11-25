package file2

import (
	"bytes"
	"errors"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/width"
	"strings"
	"unicode"
)

var ErrChat = errors.New("file2: unsupported native text chat")

// Measured western normal text header (historical original capture):
// 04 00 nameLen textLen 00 00 in titled rooms (capture31); capture29
// aftergame uses0c. Historical02 belongs to area text, outside this codec.
// Tokens and control announcements must not be interpreted as text.
func IsWesternChatEnvelope(p []byte) bool {
	return len(p) >= 6 && (p[0] == 4 || p[0] == 12) && p[1] == 0 && p[4] == 0 && p[5] == 0 && p[2] >= 1 && p[2] <= 15 && p[3] >= 1 && p[3] <= 64 && len(p) == 6+int(p[2])+int(p[3])
}

func DecodeWesternChat(p []byte, authenticatedName string) (text string, aftergame bool, err error) {
	if len(p) < 6 || (p[0] != 4 && p[0] != 12) || p[1] != 0 || p[4] != 0 || p[5] != 0 {
		return "", false, ErrChat
	}
	n, m := int(p[2]), int(p[3])
	if n < 1 || n > 15 || m < 1 || m > 64 || len(p) != 6+n+m || string(p[6:6+n]) != authenticatedName {
		return "", false, ErrChat
	}
	text = string(p[6+n:])
	if !asciiText(text) {
		return "", false, ErrChat
	}
	return text, p[0] == 12, nil
}

func EncodeWesternChat(name, text string, aftergame bool) ([]byte, error) {
	// Keep canonical Unicode unchanged. Only the western rendering narrows full
	// width Latin characters from the measured Japanese keyboard. No guessed
	// Japanese translation, question marks, truncation or nickname normalization.
	text = width.Narrow.String(text)
	if len(name) < 1 || len(name) > 15 || !asciiText(name) || len(text) < 1 || len(text) > 64 || !asciiText(text) {
		return nil, ErrChat
	}
	kind := byte(4)
	if aftergame {
		kind = 12
	}
	return append(append([]byte{kind, 0, byte(len(name)), byte(len(text)), 0, 0}, []byte(name)...), []byte(text)...), nil
}
func asciiText(text string) bool {
	for _, r := range text {
		if r < 32 || r > 126 {
			return false
		}
	}
	return true
}

// DecodeJapaneseChatText receives the already unshifted CHATIN field. Capture30
// has five full-width Latin c characters (10 SJIS bytes), then a trailer0c.
func DecodeJapaneseChatText(p []byte) (string, error) {
	if len(p) < 1 || len(p) > 256 {
		return "", ErrChat
	}
	text, err := japanese.ShiftJIS.NewDecoder().String(string(p))
	if err != nil || strings.ContainsRune(text, unicode.ReplacementChar) {
		return "", ErrChat
	}
	for _, r := range text {
		if unicode.IsControl(r) {
			return "", ErrChat
		}
	}
	roundtrip, err := japanese.ShiftJIS.NewEncoder().Bytes([]byte(text))
	if err != nil || !bytes.Equal(roundtrip, p) {
		return "", ErrChat
	}
	return text, nil
}

// CHATOUT envelope ported from BioServer2 PacketHandler.broadcastChatOut;
// identity always comes from the shared authenticated authority.
func EncodeJapaneseChat(handle, name, text string) ([]byte, error) {
	if len(handle) != 6 || !asciiText(handle) {
		return nil, ErrChat
	}
	nick, err := japanese.ShiftJIS.NewEncoder().Bytes([]byte(name))
	if err != nil || len(nick) < 1 || len(nick) > 30 {
		return nil, ErrChat
	}
	msg, err := japanese.ShiftJIS.NewEncoder().Bytes([]byte(text))
	if err != nil || len(msg) < 1 || len(msg) > 256 {
		return nil, ErrChat
	}
	p := []byte{0, 6}
	p = append(p, handle...)
	p = append(p, 0, byte(len(nick)))
	p = append(p, nick...)
	p = append(p, byte(len(msg)>>8), byte(len(msg)))
	p = append(p, msg...)
	p = append(p, 0, 0, 0, 0, 255)
	return p, nil
}

// Area text02 is measured separately from titled-room04 and aftergame0c.
// Preserve opaque APP tokens by requiring the exact text field shape first.
func IsWesternAreaChatEnvelope(p []byte) bool {
	if len(p) < 6 || p[0] != 2 {
		return false
	}
	copy := append([]byte(nil), p...)
	copy[0] = 4
	return IsWesternChatEnvelope(copy)
}
func DecodeWesternAreaChat(p []byte, name string) (string, error) {
	if !IsWesternAreaChatEnvelope(p) {
		return "", ErrChat
	}
	copy := append([]byte(nil), p...)
	copy[0] = 4
	text, _, err := DecodeWesternChat(copy, name)
	return text, err
}
func EncodeWesternAreaChat(name, text string) ([]byte, error) {
	p, err := EncodeWesternChat(name, text, false)
	if err != nil {
		return nil, err
	}
	p[0] = 2
	return p, nil
}
