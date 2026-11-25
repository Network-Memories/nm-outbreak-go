package modules_test

import (
	"bytes"
	"encoding/json"
	"networkmemories/nm-outbreak-go/internal/modules/file1"
	"networkmemories/nm-outbreak-go/internal/modules/file2"
	"os"
	"testing"
)

func TestHostPropertiesAgainstExistingModules(t *testing.T) {
	path := os.Getenv("NM_PROFILE_REFERENCE_FIXTURE")
	if path == "" {
		t.Skip("existing-module reference corpus not provided")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var corpus []struct {
		Game                     string
		Area, Difficulty         int
		Rules, Profile, Expected []byte
	}
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	if len(corpus) == 0 {
		t.Fatal("reference corpus empty")
	}
	for i, item := range corpus {
		var got []byte
		var err error
		switch item.Game {
		case "outbreak":
			got, err = file1.HostPropertyToUS(item.Profile, item.Area, item.Rules, item.Difficulty)
		case "outbreak2":
			got, err = file2.HostPropertyToWestern(item.Profile, item.Area, item.Rules, item.Difficulty)
		default:
			t.Fatal("unknown reference game")
		}
		if err != nil || !bytes.Equal(got, item.Expected) {
			t.Fatalf("reference profile %d differs: %v", i, err)
		}
	}
	t.Logf("matched %d rule/profile combinations against existing title modules", len(corpus))
}
