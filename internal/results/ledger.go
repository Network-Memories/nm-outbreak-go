// Package results stores one canonical result per server round and proved owner.
// Native codecs supply measured fields; transport retransmissions never award
// points twice. This journal does not infer identity from a name or IP.
package results

import (
	"encoding/json"
	"errors"
	"fmt"
	"networkmemories/nm-outbreak-go/internal/world"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"sync"
)

var ErrConflict = errors.New("conflicting reports for the same round and owner")

// Pointers distinguish absent measurements from a native zero.
type Stats struct {
	Scenario   *uint8  `json:"scenario,omitempty"`
	Mode       *uint8  `json:"mode,omitempty"`
	Cleared    *bool   `json:"cleared,omitempty"`
	Character  *uint16 `json:"character,omitempty"`
	ClearTicks *uint32 `json:"clear_ticks,omitempty"`
	Points     *uint32 `json:"points,omitempty"`
}
type Source struct {
	Protocol string `json:"protocol"`
	Region   string `json:"region"`
}
type Record struct {
	RoundID string     `json:"round_id"`
	Game    world.Game `json:"game"`
	OwnerID int64      `json:"owner_id"`
	Stats   Stats      `json:"stats"`
	Sources []Source   `json:"sources"`
}
type Ledger struct {
	mu      sync.Mutex
	path    string
	records map[string]Record
}

func Open(path string) (*Ledger, error) {
	l := &Ledger{path: path, records: make(map[string]Record)}
	if path == "" {
		return l, nil
	}
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return l, nil
	}
	if err != nil {
		return nil, err
	}
	var doc struct {
		Version int      `json:"version"`
		Records []Record `json:"records"`
	}
	if err = json.Unmarshal(raw, &doc); err != nil || doc.Version != 1 {
		return nil, errors.New("invalid canonical results file")
	}
	for _, r := range doc.Records {
		if !valid(r) {
			return nil, errors.New("invalid canonical result")
		}
		k := key(r)
		if _, exists := l.records[k]; exists {
			return nil, errors.New("duplicate canonical result key")
		}
		l.records[k] = copyRecord(r)
	}
	return l, nil
}
func valid(r Record) bool {
	if r.RoundID == "" || len(r.RoundID) > 64 || r.OwnerID < 1 || (r.Game != world.GameFile1 && r.Game != world.GameFile2) || len(r.Sources) == 0 {
		return false
	}
	for _, s := range r.Sources {
		if s.Protocol != "snap" && s.Protocol != "kddi" {
			return false
		}
		if s.Region != "pal" && s.Region != "ntsc-u" && s.Region != "ntsc-j" {
			return false
		}
	}
	return true
}
func key(r Record) string { return fmt.Sprintf("%s:%s:%d", r.Game, r.RoundID, r.OwnerID) }
func copyRecord(r Record) Record {
	b, _ := json.Marshal(r)
	var out Record
	_ = json.Unmarshal(b, &out)
	return out
}
func merge[T comparable](dst **T, src *T) error {
	if src == nil {
		return nil
	}
	if *dst != nil && **dst != *src {
		return ErrConflict
	}
	v := *src
	*dst = &v
	return nil
}
func combine(a Stats, b Stats) (Stats, error) {
	for _, err := range []error{merge(&a.Scenario, b.Scenario), merge(&a.Mode, b.Mode), merge(&a.Cleared, b.Cleared), merge(&a.Character, b.Character), merge(&a.ClearTicks, b.ClearTicks), merge(&a.Points, b.Points)} {
		if err != nil {
			return Stats{}, err
		}
	}
	return a, nil
}

// Accept must be called only after the core resolves the authenticated session
// to its immutable round roster. This API is never exposed directly to clients.
func (l *Ledger) Accept(r Record) error {
	if !valid(r) {
		return errors.New("proved canonical result context required")
	}
	r = copyRecord(r)
	l.mu.Lock()
	defer l.mu.Unlock()
	k := key(r)
	old, exists := l.records[k]
	if exists {
		stats, err := combine(copyRecord(old).Stats, r.Stats)
		if err != nil {
			return err
		}
		merged := copyRecord(old)
		merged.Stats = stats
		for _, src := range r.Sources {
			found := false
			for _, prior := range merged.Sources {
				if prior == src {
					found = true
					break
				}
			}
			if !found {
				merged.Sources = append(merged.Sources, src)
			}
		}
		r = merged
	}
	if exists && reflect.DeepEqual(old, r) {
		return nil
	}
	l.records[k] = r
	if err := l.persist(); err != nil {
		if exists {
			l.records[k] = old
		} else {
			delete(l.records, k)
		}
		return err
	}
	return nil
}
func (l *Ledger) persist() error {
	if l.path == "" {
		return nil
	}
	dir := filepath.Dir(l.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".results-")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	records := l.snapshot()
	err = json.NewEncoder(tmp).Encode(struct {
		Version int      `json:"version"`
		Records []Record `json:"records"`
	}{1, records})
	if err == nil {
		err = tmp.Sync()
	}
	closeErr := tmp.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(name, l.path)
}
func (l *Ledger) snapshot() []Record {
	out := make([]Record, 0, len(l.records))
	for _, r := range l.records {
		out = append(out, copyRecord(r))
	}
	sort.Slice(out, func(i, j int) bool { return key(out[i]) < key(out[j]) })
	return out
}
func (l *Ledger) Snapshot() []Record { l.mu.Lock(); defer l.mu.Unlock(); return l.snapshot() }

// Totals derive from canonical records rather than replay-sensitive increments.
func (l *Ledger) Points(game world.Game, owner int64) uint64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	var n uint64
	for _, r := range l.records {
		if r.Game == game && r.OwnerID == owner && r.Stats.Points != nil {
			n += uint64(*r.Stats.Points)
		}
	}
	return n
}
