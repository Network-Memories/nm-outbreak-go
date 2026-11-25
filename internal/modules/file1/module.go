package file1

import (
	"encoding/binary"
	"errors"
	"networkmemories/nm-outbreak-go/internal/modules"
	"networkmemories/nm-outbreak-go/internal/world"
)

// Measured File #1 (SLUS_207.65 / SLPM_654.28) constants.
const (
	GameName         = "outbreak"
	JPProfileSize    = 208
	USProfileSize    = 240
	ScenarioCount    = 5
	SharedAreaMin    = 1
	SharedAreaMax    = 10
	OpenRoomStatBase = uint32(0x00020000)
)

var ScenarioNames = [ScenarioCount]string{
	"Outbreak",
	"Below Freezing Point",
	"The Hive",
	"Hellfire",
	"Decisions, Decisions",
}

// HostPropertyToUS projects a measured 208-byte JP File #1 host profile into
// the 240-byte US host profile (+208=1, +209..+239=0).
func HostPropertyToUS(jpProfile []byte, area int, ruleValues []byte, difficulty int) ([]byte, error) {
	if len(jpProfile) != JPProfileSize {
		return nil, errors.New("file1: expected 208-byte JP profile")
	}
	if area < SharedAreaMin || area > SharedAreaMax {
		return nil, errors.New("file1: area outside shared range 1..10")
	}
	if len(ruleValues) != 4 || ruleValues[0] > 2 || ruleValues[1] > 4 ||
		int(ruleValues[2]) != difficulty || difficulty < 0 || difficulty > 3 || ruleValues[3] > 1 {
		return nil, errors.New("file1: unmeasured JP rule values")
	}
	out := make([]byte, USProfileSize)
	copy(out[:JPProfileSize], jpProfile)
	out[208] = 1 // measured File #1 host marker
	return out, nil
}

// CharacterCode returns the measured character selection at +200 (LE16).
func CharacterCode(profile []byte) (uint16, error) {
	if len(profile) != JPProfileSize && len(profile) != USProfileSize {
		return 0, errors.New("file1: invalid profile length")
	}
	return binary.LittleEndian.Uint16(profile[200:202]), nil
}

// Original File1 lobby captures show a 33-byte version response with a
// client-side packet counter. Keep the empty echoed response for fixtures.
func versionReply(query, reply uint16, payload []byte) bool {
	return (query == reply && len(payload) == 0) || len(payload) == 33
}

// EmptyPlayerScore is the short 6917 envelope: player number followed by a zero short.
func EmptyPlayerScore(number byte) []byte {
	if number < 1 || number > 4 {
		return nil
	}
	return []byte{number, 0, 0}
}

// KDDILaunchRules builds the 18-byte File #1 GAMEDIFF (0x6914) reply from
// measured Japanese room rules (players, wait, difficulty, friendly_fire).
func KDDILaunchRules(ruleValues []byte, difficulty int) ([]byte, error) {
	if len(ruleValues) != 4 || ruleValues[0] > 2 || ruleValues[1] > 4 ||
		int(ruleValues[2]) != difficulty || difficulty < 0 || difficulty > 3 || ruleValues[3] > 1 {
		return nil, errors.New("file1: unmeasured JP launch rules")
	}
	out := make([]byte, 18)
	out[1] = 16
	out[2] = 1
	out[3] = byte(difficulty)
	out[4] = ruleValues[3]
	return out, nil
}

// EncodeStatWord encodes a File #1 US waiting room STAT word (0x00020000 | (rules << 9) | (scenario - 1) << 1)
// measured in SLUS_207.65 netwk.bin (0x00589570..0x00589664, 0x00599d58, 0x0058a77c).
func EncodeStatWord(scenario, players, waitIndex, difficulty, friendlyFire int) (uint32, error) {
	if scenario < 1 || scenario > ScenarioCount || players < 2 || players > 4 ||
		waitIndex < 0 || waitIndex > 3 || difficulty < 0 || difficulty > 3 || friendlyFire < 0 || friendlyFire > 1 {
		return 0, errors.New("file1: invalid STAT parameters")
	}
	r := uint32((players - 2) | (waitIndex << 2) | (difficulty << 4) | (friendlyFire << 6))
	return OpenRoomStatBase | (r << 9) | (uint32(scenario-1) << 1), nil
}

// DecodeStatWord decodes a File #1 US waiting room STAT word into scenario (1..5),
// players (2..4), waitIndex (0..3 -> 1..4 in KDDI), difficulty (0..3), and friendlyFire (0..1).
func DecodeStatWord(stat uint32) (scenario, players, waitIndex, difficulty, friendlyFire int, err error) {
	w := (stat &^ uint32(0x40000000))
	if (w & uint32(0xfffe01e1)) != OpenRoomStatBase {
		return 0, 0, 0, 0, 0, errors.New("file1: unmeasured STAT word")
	}
	scenIdx := int((w >> 1) & 0x0f)
	if scenIdx < 0 || scenIdx >= ScenarioCount {
		return 0, 0, 0, 0, 0, errors.New("file1: invalid STAT scenario")
	}
	r := int((w >> 9) & 0x7f)
	players = (r & 0x03) + 2
	waitIndex = (r >> 2) & 0x03
	difficulty = (r >> 4) & 0x03
	friendlyFire = (r >> 6) & 0x01
	return scenIdx + 1, players, waitIndex, difficulty, friendlyFire, nil
}

// WaitingRules projects a measured File #1 US waiting room STAT word and host profile
// into the 4 KDDI rule choices (players-2, kddiWaitIndex, difficulty, friendly_fire).
func WaitingRules(area int, stat uint32, profile []byte) ([]byte, error) {
	if area < SharedAreaMin || area > SharedAreaMax || len(profile) != USProfileSize || profile[208] != 1 {
		return nil, errors.New("file1: unmeasured US waiting property")
	}
	_, players, usWait, difficulty, ff, err := DecodeStatWord(stat)
	if err != nil {
		return nil, err
	}
	// US wait choices 0..3 (5, 10, 15, 30 min) map to KDDI wait choices 1..4 (3, 5, 10, 15, 30 min).
	return []byte{byte(players - 2), byte(usWait + 1), byte(difficulty), byte(ff)}, nil
}

// LaunchRules projects a measured File #1 US room STAT word and host profile to KDDI 6914 (18 bytes).
func LaunchRules(area int, stat uint32, profile []byte) ([]byte, error) {
	rules, err := WaitingRules(area, stat, profile)
	if err != nil {
		return nil, err
	}
	return KDDILaunchRules(rules, int(rules[2]))
}

// Definition declares only this title's measured native variants; PAL has no File1 online mode.
func Definition() modules.Definition {
	return modules.Definition{
		KDDIEmptyPlayerScore: EmptyPlayerScore,
		KDDIAreaExitPayload:  make([]byte, 6),
		KDDIMedia:            []uint16{0x0001},
		SNAPWaitingRules:     WaitingRules,
		SNAPLaunchRules:      LaunchRules,
		SNAPEntryGreeting:    true,
		SNAPAftergameProfile: AftergameProfile,
		// After-game journal: 2048 bytes, same envelopes as File 2 (go-f1-us-us-aftergame.pcap,
		// 64 SN@P blocks of 32 bytes; JAP EVENTDAT of 76 bytes = 64-byte blocks).
		AftergameJournalBytes: 2048,
		MixedEnginePeers:      4, // three Japanese peers measured on the reference server (java-f1-jp3-game.pcap)
		KDDIVersionReply:      versionReply, KDDILobbyLoginSuffix: []byte{0, 2, 0, 0},
		KDDIHeartbeatPayload: []byte{0, 2, 0, 1, 3, 0xe7, 0, 1}, WaitingRule: 1, WaitingSeconds: []uint16{180, 300, 600, 900, 1800}, KDDI: kddiDirectory(), KDDICreation: creationRules, ResultScenarios: 5, ResultModes: 2, SNAPTitles: map[uint32]string{0xCAE0: "ntsc-u"}, Game: world.GameFile1, Regions: map[string][]string{"snap": {"ntsc-u"}, "kddi": {"ntsc-j"}}, ProfileBytes: map[string]int{"snap": USProfileSize, "kddi": JPProfileSize}, Areas: []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, ScenarioMin: 0, ScenarioMax: ScenarioCount, PlayersMin: 2, PlayersMax: 4, DifficultyMax: 3,
	}
}
