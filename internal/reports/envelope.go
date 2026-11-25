// Package reports preserves native evidence from both transports in one journal.
// A claimed APP login is never promoted to an owner. A decoded report alone
// does not prove a scenario or qualify for the canonical results ledger.
package reports

import (
	"networkmemories/nm-outbreak-go/internal/results"
	"networkmemories/nm-outbreak-go/internal/world"
	"sync"
	"time"
)

type Envelope struct {
	JapaneseFile2 *JapaneseFile2 `json:"japanese_file2_structure,omitempty"`
	RemoteHost    string         `json:"transport_host,omitempty"`
	RoundID       string         `json:"canonical_round,omitempty"`
	PayloadLogin  string         `json:"payload_login,omitempty"`
	Game          world.Game     `json:"game"`
	Protocol      string         `json:"protocol"`
	Command       uint16         `json:"command"`
	ClaimedLogin  string         `json:"claimed_login,omitempty"`
	SessionID     string         `json:"verified_session,omitempty"`
	OwnerID       int64          `json:"verified_owner,omitempty"`
	NativeHandle  string         `json:"native_handle,omitempty"`
	Sequence      byte           `json:"sequence"`
	PacketID      uint16         `json:"packet_id,omitempty"`
	Payload       []byte         `json:"payload"`
	NativeFrame   []byte         `json:"native_frame,omitempty"`
	Normalized    *results.Stats `json:"decoded,omitempty"`
	ReceivedAt    time.Time      `json:"received_at"`
}
type Sink interface{ Archive(Envelope) error }
type FileSink struct {
	mu   sync.Mutex
	Path string
}

// JapaneseFile2 preserves only the measured structural fields. Summary/trailer
// are opaque; peer status is not assumed to mean alive/dead.
type JapanesePeer struct {
	Status, Character byte
	Handle, Nickname  []byte
}
type JapaneseFile2 struct {
	GameSession   []byte
	OpaqueSummary []byte
	SummaryBytes  [7]byte
	SummaryWords  [2]uint32
	Peers         []JapanesePeer
	OpaqueTrailer []byte
	TrailerWords  [8]uint16
	Checksums     [][]byte
}
