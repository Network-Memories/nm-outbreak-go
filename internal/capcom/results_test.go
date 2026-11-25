package capcom

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func TestScenarioResultPythonReferenceAndTruncation(t *testing.T) {
	raw, err := os.ReadFile("testdata/python-results.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Sequence byte
		Payload  string
		Expected struct {
			Login                     string
			Scenario, Mode, Character byte
			Cleared                   bool
			ClearTicks                uint32 `json:"clear_ticks"`
			Points                    uint32 `json:"points"`
		}
	}
	if err = json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		p, _ := hex.DecodeString(row.Payload)
		got, err := DecodeScenarioResult(p, row.Sequence)
		if err != nil || got.Login != row.Expected.Login || *got.Stats.Scenario != row.Expected.Scenario || *got.Stats.Mode != row.Expected.Mode || *got.Stats.Cleared != row.Expected.Cleared || *got.Stats.ClearTicks != row.Expected.ClearTicks || *got.Stats.Points != row.Expected.Points || *got.Stats.Character != uint16(row.Expected.Character) || len(got.Teammates) != 2 || got.Diagnostics[7] != 7 {
			t.Fatal("reference result mismatch", err)
		}
		for n := 0; n < len(p); n++ {
			if _, err = DecodeScenarioResult(p[:n], row.Sequence); err == nil {
				t.Fatalf("truncated report accepted at %d", n)
			}
		}
		p[0] ^= 1
		if _, err = DecodeScenarioResult(p, row.Sequence); err == nil {
			t.Fatal("inconsistent clear flags accepted")
		}
	}
}
