package file1

import (
	"bytes"
	"testing"
)

func TestFile1ProjectionAndMeasuredBounds(t *testing.T) {
	raw := make([]byte, 208)
	raw[200] = 4
	out, err := HostPropertyToUS(raw, 1, []byte{2, 4, 3, 1}, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 240 || !bytes.Equal(out[:208], raw) || out[208] != 1 || out[220] != 0 || out[224] != 0 {
		t.Fatal("File2 fields leaked into File1")
	}
	if char, err := CharacterCode(out); err != nil || char != 4 {
		t.Fatal("native character not retained")
	}
	for _, test := range []struct {
		area, difficulty int
		rules            []byte
	}{{0, 0, []byte{0, 0, 0, 0}}, {11, 0, []byte{0, 0, 0, 0}}, {1, 0, []byte{3, 0, 0, 0}}, {1, 0, []byte{0, 5, 0, 0}}, {1, 1, []byte{0, 0, 0, 0}}, {1, 0, []byte{0, 0, 0, 2}}} {
		if _, err := HostPropertyToUS(raw, test.area, test.rules, test.difficulty); err == nil {
			t.Fatal("unmeasured File1 rules accepted")
		}
	}
	if _, err := HostPropertyToUS(raw[:207], 1, []byte{0, 0, 0, 0}, 0); err == nil {
		t.Fatal("short profile accepted")
	}
	if Definition().Supports("snap", "pal") {
		t.Fatal("PAL File1 online support invented")
	}
	stat, err := EncodeStatWord(2, 4, 3, 2, 0)
	if err != nil || stat != 0x00025c02 {
		t.Fatalf("unexpected File1 STAT word: 0x%08x (%v)", stat, err)
	}
	scen, players, wait, diff, ff, err := DecodeStatWord(stat)
	if err != nil || scen != 2 || players != 4 || wait != 3 || diff != 2 || ff != 0 {
		t.Fatalf("unexpected decoded File1 STAT: %d %d %d %d %d (%v)", scen, players, wait, diff, ff, err)
	}
	usHost := make([]byte, 240)
	usHost[208] = 1
	waiting, err := WaitingRules(1, 0x0002cc00, usHost)
	if err != nil || !bytes.Equal(waiting, []byte{2, 2, 2, 1}) {
		t.Fatalf("unexpected File1 waiting rules: %v (%v)", waiting, err)
	}
	launch, err := LaunchRules(1, 0x0002cc00, usHost)
	if err != nil || len(launch) != 18 || launch[1] != 16 || launch[2] != 1 || launch[3] != 2 || launch[4] != 1 {
		t.Fatalf("unexpected File1 launch rules: %x (%v)", launch, err)
	}
}
