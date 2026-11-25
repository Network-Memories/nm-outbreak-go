package kddi

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

func TestCapturedNativeStreams(t *testing.T) {
	path := os.Getenv("NM_KDDI_CAPTURE_FIXTURE")
	if path == "" {
		t.Skip("private native capture corpus not provided")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Lobby []struct {
			Raw    []byte
			Frames []LobbyFrame
		}
		Game []struct {
			Raw     []byte
			Records [][]byte
		}
	}
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	if len(corpus.Lobby) == 0 || len(corpus.Game) == 0 {
		t.Fatal("capture corpus empty")
	}
	frames, records := 0, 0
	for i, entry := range corpus.Lobby {
		var stream LobbyStream
		var got []LobbyFrame
		for at := 0; at < len(entry.Raw); at += 7 {
			end := at + 7
			if end > len(entry.Raw) {
				end = len(entry.Raw)
			}
			got = append(got, stream.Feed(entry.Raw[at:end])...)
		}
		if len(got) != len(entry.Frames) {
			t.Fatalf("lobby stream %d count differs", i)
		}
		for j, want := range entry.Frames {
			f := got[j]
			if f.Sender != want.Sender || f.Kind != want.Kind || f.Command != want.Command || f.PacketID != want.PacketID || f.Error != want.Error || f.Reserved != want.Reserved || !bytes.Equal(f.Payload, want.Payload) {
				t.Fatalf("lobby stream %d frame %d differs", i, j)
			}
		}
		frames += len(got)
	}
	for i, entry := range corpus.Game {
		var stream GameStream
		var got [][]byte
		for at := 0; at < len(entry.Raw); at += 3 {
			end := at + 3
			if end > len(entry.Raw) {
				end = len(entry.Raw)
			}
			part, err := stream.Feed(entry.Raw[at:end])
			if err != nil {
				t.Fatalf("game stream %d: %v", i, err)
			}
			got = append(got, part...)
		}
		if len(got) != len(entry.Records) {
			t.Fatalf("game stream %d count differs", i)
		}
		for j, want := range entry.Records {
			if !bytes.Equal(got[j], want) {
				t.Fatalf("game stream %d record %d differs", i, j)
			}
		}
		records += len(got)
	}
	t.Logf("matched %d lobby frames and %d game records with fragmented native streams", frames, records)
}
