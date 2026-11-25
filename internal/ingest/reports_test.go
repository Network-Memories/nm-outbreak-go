package ingest

import (
	"crypto/sha1"
	"errors"
	"networkmemories/nm-outbreak-go/internal/core"
	"networkmemories/nm-outbreak-go/internal/identity"
	"networkmemories/nm-outbreak-go/internal/modules/file1"
	"networkmemories/nm-outbreak-go/internal/protocol/snapauth"
	"networkmemories/nm-outbreak-go/internal/reports"
	"networkmemories/nm-outbreak-go/internal/results"
	"networkmemories/nm-outbreak-go/internal/world"
	"testing"
)

type authFixture struct{}

func (authFixture) Authenticate(string, string) (*identity.Account, error) {
	return nil, identity.ErrAuthentication
}
func (authFixture) SNAPCredentials(name string) (identity.SNAPCredential, error) {
	seed := "0123456789abcdef0123456789abcdef"
	key := sha1.Sum([]byte("nativeproof" + seed))
	return identity.SNAPCredential{Account: identity.Account{UserID: 1, Username: name, NativeAlias: name, NativeNickname: name, Handle: "ABC123"}, Seed: seed, MagicKey: key[:]}, nil
}

type archiveFixture struct {
	reports []reports.Envelope
	fail    bool
}

func (a *archiveFixture) Archive(r reports.Envelope) error {
	if a.fail {
		return errors.New("archive unavailable")
	}
	a.reports = append(a.reports, r)
	return nil
}
func TestProvedNativeReportRoundAndReplayOnce(t *testing.T) {
	e := core.New(nil, authFixture{}, file1.Definition())
	ledger, _ := results.Open("")
	e.ConfigureResults(ledger)
	err := e.ConfigureSNAP(core.SNAPConfig{AdvertiseHost: "127.0.0.1", BootstrapPort: 9090, GamePorts: map[world.Game]uint16{world.GameFile1: 9091}})
	if err != nil {
		t.Fatal(err)
	}
	endpoint := "127.0.0.1:32123"
	wire, challenge, err := e.BeginSNAP(endpoint, 0xcae0, "OwnerOne", []byte("fixture-native-title"))
	if err != nil {
		t.Fatal(err)
	}
	clear, err := snapauth.Decrypt(snapauth.BootstrapKey, challenge)
	if err != nil {
		t.Fatal(err)
	}
	key := sha1.Sum(append([]byte("nativeproof"), clear[8:40]...))
	secret, err := snapauth.Decrypt(key[:], clear[148:276])
	if err != nil {
		t.Fatal(err)
	}
	answer := make([]byte, 136)
	copy(answer[8:], secret)
	answer, err = snapauth.Encrypt(snapauth.BootstrapKey, answer)
	if err != nil {
		t.Fatal(err)
	}
	handoff, err := e.FinishSNAP(endpoint, answer)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = e.SNAPGameHandoff(world.GameFile1, wire, endpoint, "OwnerOne"); err != nil {
		t.Fatal(err)
	}
	e.SetProfile(handoff.Session.ID, make([]byte, 240))
	if _, err = e.CreateRoom(handoff.Session.ID, core.RoomOptions{Area: 1, Scenario: 1, MaxPlayers: 4}); err != nil {
		t.Fatal(err)
	}
	archive := &archiveFixture{}
	sink := &Sink{Engine: e, ArchiveSink: archive}
	points := uint32(123)
	report := reports.Envelope{Game: world.GameFile1, Protocol: "snap", Command: 0x6301, ClaimedLogin: "OwnerOne", PayloadLogin: "OwnerOne", RemoteHost: "127.0.0.1", Normalized: &results.Stats{Points: &points}, Payload: []byte("opaque-original-evidence")}
	if err = sink.Archive(report); err != nil || len(ledger.Snapshot()) != 0 {
		t.Fatal("report without measured native start was ranked", err)
	}
	if err = e.ScenarioPresence(handoff.Session.ID, true); err != nil {
		t.Fatal(err)
	}
	foreign := report
	foreign.RemoteHost = "127.0.0.2"
	sink.Archive(foreign)
	if len(ledger.Snapshot()) != 0 {
		t.Fatal("foreign transport context was ranked")
	}
	for i := 0; i < 3; i++ {
		if err = sink.Archive(report); err != nil {
			t.Fatal(err)
		}
	}
	if len(ledger.Snapshot()) != 1 || ledger.Points(world.GameFile1, 1) != 123 {
		t.Fatal("native upload retry awarded points twice")
	}
	last := archive.reports[len(archive.reports)-1]
	if last.OwnerID != 1 || last.SessionID != handoff.Session.ID || last.RoundID == "" {
		t.Fatal("journal lost proved canonical context")
	}
	japanese := report
	japanese.Protocol = "kddi"
	japanese.Command = 0x6138
	japanese.JapaneseFile2 = &reports.JapaneseFile2{
		GameSession:  []byte("000000000000042"),
		SummaryWords: [2]uint32{2130, 250},
	}
	if err = sink.Archive(japanese); err != nil {
		t.Fatal(err)
	}
	last = archive.reports[len(archive.reports)-1]
	if last.JapaneseFile2 == nil || last.JapaneseFile2.SummaryWords != [2]uint32{2130, 250} {
		t.Fatal("journal lost the decoded Japanese words")
	}
	if len(ledger.Snapshot()) != 1 || ledger.Points(world.GameFile1, 1) != 123 {
		t.Fatal("Japanese report interpreted using western schema")
	}
	e.ScenarioPresence(handoff.Session.ID, false)
	e.ScenarioPresence(handoff.Session.ID, true)
	archive.fail = true
	if err = sink.Archive(report); err == nil || ledger.Points(world.GameFile1, 1) != 123 {
		t.Fatal("points accepted without durable evidence")
	}
}
