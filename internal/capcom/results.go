// APP 6301 decoder ported from openSNAP outbreak/scenario_report.py (AGPL-3.0).
// Schema is shared by File 1 and File 2; their mode semantics stay distinct.
package capcom

import (
	"encoding/binary"
	"networkmemories/nm-outbreak-go/internal/results"
	"strings"
)

type Teammate struct {
	Name      string `json:"name"`
	Alive     bool   `json:"alive"`
	Character byte   `json:"character"`
}
type ScenarioResult struct {
	Login       string        `json:"login"`
	Stats       results.Stats `json:"stats"`
	Teammates   []Teammate    `json:"teammates"`
	Diagnostics [8]uint32     `json:"network_diagnostics"`
}

func decodedText(b []byte) string {
	b = []byte(strings.SplitN(string(b), "\x00", 2)[0])
	for i, c := range b {
		if c > 127 {
			b[i] = '?'
		}
	}
	return string(b)
}
func DecodeScenarioResult(payload []byte, sequence byte) (ScenarioResult, error) {
	var r ScenarioResult
	if len(payload) < 3 {
		return r, errCodec
	}
	cleared, mode, scenario := payload[0], payload[1], payload[2]
	login, n, err := DecodeField(payload[3:], sequence)
	if err != nil {
		return r, err
	}
	offset := 3 + n
	if len(payload) < offset+10 || cleared > 1 || payload[offset] != cleared {
		return r, errCodec
	}
	character := uint16(payload[offset+1])
	ticks := binary.BigEndian.Uint32(payload[offset+2:])
	points := binary.BigEndian.Uint32(payload[offset+6:])
	done := cleared == 1
	r.Login = decodedText(login)
	r.Stats = results.Stats{Scenario: &scenario, Mode: &mode, Cleared: &done, Character: &character, ClearTicks: &ticks, Points: &points}
	offset += 10
	for i := 0; i < 3; i++ {
		name, n, err := DecodeField(payload[offset:], sequence)
		if err != nil {
			return ScenarioResult{}, err
		}
		offset += n
		if len(payload) < offset+2 {
			return ScenarioResult{}, errCodec
		}
		if text := decodedText(name); text != "" {
			r.Teammates = append(r.Teammates, Teammate{Name: text, Alive: payload[offset] == 1, Character: payload[offset+1]})
		}
		offset += 2
	}
	if len(payload) != offset+32 {
		return ScenarioResult{}, errCodec
	}
	for i := range r.Diagnostics {
		r.Diagnostics[i] = binary.BigEndian.Uint32(payload[offset+i*4:])
	}
	return r, nil
}
