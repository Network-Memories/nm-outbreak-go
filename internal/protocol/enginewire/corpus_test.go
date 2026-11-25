package enginewire

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

// Optional private original-client corpus. No capture, account or infrastructure
// path is distributed; the independent Python reference produces expected bodies.
func TestMeasuredPrivateEngineCorpus(t *testing.T) {
	path := os.Getenv("NM_ENGINE_CORPUS")
	if path == "" {
		t.Skip("private measured corpus not supplied")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Western []struct {
			Payload  []byte
			Expected []Frame
		}
		Japanese []struct {
			Record   []byte
			Expected Frame
		}
	}
	if err = json.Unmarshal(raw, &corpus); err != nil {
		t.Fatal(err)
	}
	if len(corpus.Western) == 0 || len(corpus.Japanese) == 0 {
		t.Fatal("empty original-client dialect")
	}
	for i, item := range corpus.Western {
		frames, err := WesternDecode(item.Payload, 2)
		if err != nil || len(frames) != len(item.Expected) {
			t.Fatal("Western", i, err)
		}
		for j, want := range item.Expected {
			if frames[j].Peer != want.Peer || !bytes.Equal(frames[j].Body, want.Body) {
				t.Fatal("Western body differs", i, j)
			}
			if frames[j].Peer != 15 {
				native, err := JapaneseEncode(frames[j])
				if err != nil {
					t.Fatal("Western cannot fit Japanese envelope", i, j, err)
				}
				back, known, err := JapaneseData(native)
				if err != nil || !known || back.Peer != want.Peer || !bytes.Equal(back.Body, want.Body) {
					t.Fatal("Western to Japanese body differs", i, j, err)
				}
			}
		}
	}
	for i, item := range corpus.Japanese {
		f, known, err := JapaneseData(item.Record)
		if err != nil || !known || f.Peer != item.Expected.Peer || !bytes.Equal(f.Body, item.Expected.Body) {
			t.Fatal("Japanese body differs", i, err)
		}
		w, err := WesternEncode(f)
		if err != nil {
			t.Fatal(i, err)
		}
		back, err := WesternDecode(w, 2)
		if err != nil || len(back) != 1 || !bytes.Equal(back[0].Body, f.Body) {
			t.Fatal("roundtrip", i, err)
		}
	}
	t.Logf("original capture envelopes: western=%d japanese=%d", len(corpus.Western), len(corpus.Japanese))
}
