package kddi

import "testing"

func TestLobbyStreamAndGameStream(t *testing.T) {
	in := LobbyFrame{
		Sender:   0x81,
		Kind:     0x02,
		Command:  0x6190,
		PacketID: 7,
		Error:    0,
		Payload:  []byte{0x00, 0x04, 0xde, 0xad, 0xbe, 0xef},
	}
	raw, err := in.Encode()
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}
	var ls LobbyStream
	// Feed in two fragmented TCP chunks
	if got := ls.Feed(raw[:5]); len(got) != 0 {
		t.Fatalf("expected 0 frames on partial header")
	}
	got := ls.Feed(raw[5:])
	if len(got) != 1 || got[0].Command != 0x6190 || got[0].PacketID != 7 || len(got[0].Payload) != 6 {
		t.Fatalf("unexpected decoded frame: %+v", got)
	}

	var gs GameStream
	recs, err := gs.Feed([]byte{0x04, 0xaa, 0xbb, 0xcc, 0x02, 0xdd})
	if err != nil || len(recs) != 2 || len(recs[0]) != 4 || len(recs[1]) != 2 {
		t.Fatalf("unexpected game records: %v err=%v", recs, err)
	}
}

func FuzzNativeStreams(f *testing.F) {
	f.Add([]byte{0x81, 1, 0x61, 1, 0, 0, 0, 1, 0, 0xff, 0xff, 0xff})
	f.Fuzz(func(t *testing.T, raw []byte) {
		var lobby LobbyStream
		for at := 0; at < len(raw); at += 7 {
			end := at + 7
			if end > len(raw) {
				end = len(raw)
			}
			for _, frame := range lobby.Feed(raw[at:end]) {
				if len(frame.Payload) > 65535 {
					t.Fatal("lobby length overflow")
				}
			}
		}
		var game GameStream
		_, _ = game.Feed(raw)
	})
}
