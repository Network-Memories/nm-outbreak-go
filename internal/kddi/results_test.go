package kddi

import (
	"bytes"
	"testing"
)

func TestJapanesePostgameStructurePreservesUnknownStats(t *testing.T) {
	for _, pid := range []uint16{0, 127, 128, 259, 65535} {
		p := append([]byte{0x45, 0x67}, nativeField([]byte("000000000000042"), pid)...)
		summary := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14}
		p = append(p, summary...)
		p = append(p, 2, 1)
		p = append(p, nativeField([]byte("AB1234"), pid)...)
		nick := bytes.Repeat([]byte{0x82, 0x81}, 5)
		p = append(p, nativeField(nick, pid)...)
		for i := 0; i < 2; i++ {
			p = append(p, 0, 0)
			p = append(p, nativeField(make([]byte, 6), pid)...)
			p = append(p, nativeField(nil, pid)...)
		}
		trailer := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
		p = append(p, trailer...)
		f := LobbyFrame{Sender: 0x81, Kind: 2, Command: 0x6138, PacketID: pid, Payload: p}
		r, err := DecodeFile2Postgame(f)
		if err != nil || string(r.GameSession) != "000000000000042" || !bytes.Equal(r.OpaqueSummary, summary) || !bytes.Equal(r.OpaqueTrailer, trailer) || len(r.Peers) != 3 || len(r.Checksums) != 7 || r.Peers[0].Status != 2 || !bytes.Equal(r.Peers[0].Nickname, nick) {
			t.Fatal("measured Japanese structure differs", err)
		}
		if r.SummaryBytes != [7]byte{0, 1, 2, 3, 4, 5, 6} ||
			r.SummaryWords != [2]uint32{0x0708090a, 0x0b0c0d0e} ||
			r.TrailerWords != [8]uint16{0x0001, 0x0203, 0x0405, 0x0607, 0x0809, 0x0a0b, 0x0c0d, 0x0e0f} {
			t.Fatal("Japanese report integers differ from original wire layout")
		}
		for n := 0; n < len(p); n++ {
			f.Payload = p[:n]
			if _, err = DecodeFile2Postgame(f); err == nil {
				t.Fatal("truncated Japanese report accepted")
			}
		}
		f.Payload = append(append([]byte(nil), p...), 0)
		if _, err = DecodeFile2Postgame(f); err == nil {
			t.Fatal("unmeasured Japanese tail accepted")
		}
	}
}
