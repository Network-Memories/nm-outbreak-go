package results

import (
	"errors"
	"networkmemories/nm-outbreak-go/internal/world"
	"path/filepath"
	"testing"
)

func ptr[T any](v T) *T { return &v }
func TestCanonicalMixedReportsNoDuplicateAndRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "results.json")
	l, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	r := Record{RoundID: "server-round-one", Game: world.GameFile2, OwnerID: 42, Stats: Stats{Points: ptr(uint32(100)), Scenario: ptr(uint8(3))}, Sources: []Source{{"snap", "pal"}}}
	for i := 0; i < 3; i++ {
		if err = l.Accept(r); err != nil {
			t.Fatal(err)
		}
	}
	r.Sources = []Source{{"kddi", "ntsc-j"}}
	r.Stats.ClearTicks = ptr(uint32(12000))
	if err = l.Accept(r); err != nil {
		t.Fatal(err)
	}
	if len(l.Snapshot()) != 1 || l.Points(world.GameFile2, 42) != 100 || len(l.Snapshot()[0].Sources) != 2 {
		t.Fatal("mixed reports double counted")
	}
	copy := l.Snapshot()
	*copy[0].Stats.Points = 999
	if l.Points(world.GameFile2, 42) != 100 {
		t.Fatal("snapshot leaked pointers")
	}
	loaded, err := Open(path)
	if err != nil || loaded.Points(world.GameFile2, 42) != 100 {
		t.Fatal("restart duplicated/lost result", err)
	}
	r.Stats.Points = ptr(uint32(200))
	if err = loaded.Accept(r); !errors.Is(err, ErrConflict) {
		t.Fatal("conflict silently overwrote score", err)
	}
	if loaded.Points(world.GameFile2, 42) != 100 {
		t.Fatal("conflict changed ranking")
	}
	r.OwnerID = 43
	r.Stats.Points = ptr(uint32(50))
	if err = loaded.Accept(r); err != nil {
		t.Fatal(err)
	}
	if len(loaded.Snapshot()) != 2 {
		t.Fatal("different participant merged")
	}
	r.OwnerID = 42
	r.RoundID = "server-round-two"
	if err = loaded.Accept(r); err != nil {
		t.Fatal(err)
	}
	if loaded.Points(world.GameFile2, 42) != 150 {
		t.Fatal("identical subsequent run discarded")
	}
	r.Game = world.GameFile1
	if err = loaded.Accept(r); err != nil {
		t.Fatal(err)
	}
	if loaded.Points(world.GameFile1, 42) != 50 || loaded.Points(world.GameFile2, 42) != 150 {
		t.Fatal("File1 and File2 totals merged")
	}
}
func TestAbsentAndNativeZeroRemainDistinct(t *testing.T) {
	l, _ := Open("")
	r := Record{RoundID: "r", Game: world.GameFile1, OwnerID: 1, Sources: []Source{{"snap", "ntsc-u"}}}
	l.Accept(r)
	if l.Snapshot()[0].Stats.Points != nil {
		t.Fatal("missing points invented")
	}
	r.Stats.Points = ptr(uint32(0))
	l.Accept(r)
	if l.Snapshot()[0].Stats.Points == nil || *l.Snapshot()[0].Stats.Points != 0 {
		t.Fatal("native zero lost")
	}
}
