package core

import (
	"crypto/sha1"
	"errors"
	"networkmemories/nm-outbreak-go/internal/identity"
	"networkmemories/nm-outbreak-go/internal/modules/file1"
	"networkmemories/nm-outbreak-go/internal/modules/file2"
	"networkmemories/nm-outbreak-go/internal/protocol/snapauth"
	"networkmemories/nm-outbreak-go/internal/world"
	"testing"
	"time"
)

type nativeAuthority struct{ testAuthority }

func (nativeAuthority) SNAPCredentials(name string) (identity.SNAPCredential, error) {
	if name != "NativeOne" {
		return identity.SNAPCredential{}, identity.ErrAuthentication
	}
	seed := "0123456789abcdef0123456789abcdef"
	magic := sha1.Sum([]byte("fixturepass" + seed))
	return identity.SNAPCredential{Account: identity.Account{UserID: 1, Username: name, NativeAlias: name, NativeNickname: name, Handle: "ABC123"}, Seed: seed, MagicKey: magic[:]}, nil
}
func nativeEngine(t *testing.T) *Engine {
	t.Helper()
	e := New(nil, nativeAuthority{}, file1.Definition(), file2.Definition())
	if err := e.ConfigureSNAP(SNAPConfig{AdvertiseHost: "127.0.0.1", BootstrapPort: 9090, GamePorts: map[world.Game]uint16{world.GameFile1: 9091, world.GameFile2: 9092}}); err != nil {
		t.Fatal(err)
	}
	return e
}
func nativeAnswer(t *testing.T, challenge []byte) []byte {
	t.Helper()
	raw, err := snapauth.Decrypt(snapauth.BootstrapKey, challenge)
	if err != nil {
		t.Fatal(err)
	}
	size := int(uint32(raw[4])<<24 | uint32(raw[5])<<16 | uint32(raw[6])<<8 | uint32(raw[7]))
	magic := sha1.Sum([]byte("fixturepass" + string(raw[8:8+size])))
	clear, err := snapauth.Decrypt(magic[:], raw[148:276])
	if err != nil {
		t.Fatal(err)
	}
	answer := make([]byte, 136)
	copy(answer[8:], clear)
	encrypted, err := snapauth.Encrypt(snapauth.BootstrapKey, answer)
	if err != nil {
		t.Fatal(err)
	}
	return encrypted
}
func TestNativeProofIsBoundAndCannotReplayAcrossChallenges(t *testing.T) {
	e := nativeEngine(t)
	wire, challenge, err := e.BeginSNAP("127.0.0.1:2000", 0xCAF3, "NativeOne", []byte("request-one"))
	if err != nil {
		t.Fatal(err)
	}
	if len(e.sessions) != 0 {
		t.Fatal("challenge created a player before proof")
	}
	answer := nativeAnswer(t, challenge)
	if _, err := e.FinishSNAP("127.0.0.1:2001", answer); !errors.Is(err, ErrSession) {
		t.Fatal("proof from wrong bootstrap endpoint accepted")
	}
	accepted, err := e.FinishSNAP("127.0.0.1:2000", answer)
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := e.FinishSNAP("127.0.0.1:2000", answer)
	if err != nil || repeated.Session.ID != accepted.Session.ID || len(e.sessions) != 1 {
		t.Fatal("repeated proof created another owner/session")
	}
	if _, err := e.SNAPGameHandoff(world.GameFile2, 0, "127.0.0.1:9999", "NativeOne"); err == nil {
		t.Fatal("public host and alias alone granted another socket's identity")
	}
	if same, err := e.SNAPGameHandoff(world.GameFile2, 0, "127.0.0.1:2000", "NativeOne"); err != nil || same.Session.ID != accepted.Session.ID {
		t.Fatal("exact proved endpoint was not retained", err)
	}
	if _, err := e.SNAPGameHandoff(world.GameFile1, wire, "127.0.0.1:2000", "NativeOne"); err == nil {
		t.Fatal("File2 proof entered File1")
	}
	if _, err := e.SNAPGameHandoff(world.GameFile2, wire, "127.0.0.2:2000", "NativeOne"); err == nil {
		t.Fatal("foreign host hijacked a proved handoff")
	}
	e.Logout(accepted.Session.ID)
	newWire, _, err := e.BeginSNAP("127.0.0.1:2000", 0xCAF3, "NativeOne", []byte("request-two"))
	if err != nil {
		t.Fatal(err)
	}
	if newWire == wire {
		t.Fatal("new native flow reused old wire ID")
	}
	if _, err := e.FinishSNAP("127.0.0.1:2000", answer); err == nil || len(e.sessions) != 0 {
		t.Fatal("old answer proved the new challenge")
	}
}
func TestNativeExpiredAndAmbiguousHandoffsFailClosed(t *testing.T) {
	e := nativeEngine(t)
	wire, challenge, err := e.BeginSNAP("127.0.0.1:2000", 0xCAF3, "NativeOne", []byte("one"))
	if err != nil {
		t.Fatal(err)
	}
	e.snapLogins[wire].expires = time.Now().Add(-time.Second)
	if _, err := e.FinishSNAP("127.0.0.1:2000", nativeAnswer(t, challenge)); err == nil {
		t.Fatal("expired proof accepted")
	}
	var ids []uint32
	for _, endpoint := range []string{"127.0.0.1:2000", "127.0.0.1:2001"} {
		id, ch, err := e.BeginSNAP(endpoint, 0xCAF3, "NativeOne", []byte(endpoint))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := e.FinishSNAP(endpoint, nativeAnswer(t, ch)); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if _, err := e.SNAPGameHandoff(world.GameFile2, 0, "127.0.0.1:9999", "NativeOne"); err == nil {
		t.Fatal("ambiguous same-host handoff guessed an owner")
	}
	if handoff, err := e.SNAPGameHandoff(world.GameFile2, ids[0], "127.0.0.1:2000", "NativeOne"); err != nil || handoff.WireID != ids[0] {
		t.Fatal("exact proved wire ID ignored", err)
	}
}
