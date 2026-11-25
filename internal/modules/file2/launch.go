// Rule projection adapted from the existing NM ForeignRoomRules.java measurements.
package file2

import "errors"

// LaunchRules projects only measured File2 PAL/US room rules to KDDI6914.
// Neither a missing flag nor a contradictory host property becomes a default.
func LaunchRules(area int, stat uint32, p []byte) ([]byte, error) {
	rows, ok := WesternRows[area]
	if !ok || len(p) != 240 || int(p[224]) != rows {
		return nil, errors.New("unmeasured File2 launch property")
	}
	players, wait := int(p[233])+2, int(p[234])
	difficulty, ff := p[235], p[236]
	scenario := int((stat & 255) >> 1)
	expected, err := EncodeStatWord(scenario, players, wait)
	if err != nil || stat&^uint32(0x40000000) != expected || difficulty > 3 || ff > 1 {
		return nil, errors.New("contradictory File2 launch rules")
	}
	nightmare, infinity := byte(0), byte(0)
	if rows >= 5 {
		nightmare = p[237]
	}
	if rows >= 6 {
		infinity = p[238]
	}
	if nightmare > 1 || infinity > 1 {
		return nil, errors.New("unmeasured File2 launch flags")
	}
	out := make([]byte, 34)
	out[1] = 32
	out[2] = 1
	out[3] = difficulty
	out[4] = ff
	out[7] = nightmare
	out[8] = infinity
	return out, nil
}
