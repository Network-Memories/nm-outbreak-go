// Structural File 2 decoder ported from NM-OUTBREAK interop/wire/kddi_results.py.
// It has no score, duration, clear-state or ranking projection.
package kddi

import (
	"bytes"
	"encoding/binary"

	"networkmemories/nm-outbreak-go/internal/core"
	"networkmemories/nm-outbreak-go/internal/reports"
)

func DecodeFile2Postgame(f LobbyFrame) (reports.JapaneseFile2, error) {
	var r reports.JapaneseFile2
	if f.Sender != 0x81 || f.Kind != 2 || f.Command != 0x6138 || f.Error != 0 || len(f.Payload) < 2 || !bytes.Equal(f.Payload[:2], []byte{0x45, 0x67}) {
		return r, core.ErrSession
	}
	at := 2
	field := func() ([]byte, error) {
		if len(f.Payload) < at+4 {
			return nil, core.ErrSession
		}
		plain, size, err := decodeNativeField(f.Payload[at:], f.PacketID)
		if err != nil {
			return nil, err
		}
		r.Checksums = append(r.Checksums, append([]byte(nil), f.Payload[at+2:at+4]...))
		at += size
		return plain, nil
	}
	session, err := field()
	if err != nil || len(session) != 15 {
		return r, core.ErrSession
	}
	for _, c := range session {
		if c < '0' || c > '9' {
			return r, core.ErrSession
		}
	}
	r.GameSession = session
	if len(f.Payload) < at+15 {
		return r, core.ErrSession
	}
	r.OpaqueSummary = append([]byte(nil), f.Payload[at:at+15]...)
	// The original client writes seven bytes followed by two big-endian words.
	// Keep the wire values without assigning unverified meanings to them.
	copy(r.SummaryBytes[:], r.OpaqueSummary[:7])
	r.SummaryWords[0] = binary.BigEndian.Uint32(r.OpaqueSummary[7:11])
	r.SummaryWords[1] = binary.BigEndian.Uint32(r.OpaqueSummary[11:15])
	at += 15
	for i := 0; i < 3; i++ {
		if len(f.Payload) < at+2 {
			return r, core.ErrSession
		}
		peer := reports.JapanesePeer{Status: f.Payload[at], Character: f.Payload[at+1]}
		at += 2
		peer.Handle, err = field()
		if err != nil || len(peer.Handle) != 6 {
			return r, core.ErrSession
		}
		peer.Nickname, err = field()
		if err != nil {
			return r, err
		}
		r.Peers = append(r.Peers, peer)
	}
	if len(f.Payload) != at+16 {
		return r, core.ErrSession
	}
	r.OpaqueTrailer = append([]byte(nil), f.Payload[at:]...)
	for i := range r.TrailerWords {
		r.TrailerWords[i] = binary.BigEndian.Uint16(r.OpaqueTrailer[i*2:])
	}
	return r, nil
}
