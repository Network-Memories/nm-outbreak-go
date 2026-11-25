package file2

import (
	"encoding/binary"
	"errors"
	"networkmemories/nm-outbreak-go/internal/modules"
	"networkmemories/nm-outbreak-go/internal/world"
)

// Measured File #2 (SLES_533.19 / SLUS_209.84 / SLPM_656.92) constants.
const (
	GameName         = "outbreak2"
	JPProfileSize    = 208
	WesternProfileSz = 240
	ScenarioCount    = 11
)

var WesternRows = map[int]int{
	1: 4, 2: 5, 3: 4, 4: 5, 5: 6, 7: 4, 8: 4, 9: 4,
}

var JapaneseExtra = map[int][]string{
	1: {},
	2: {"nightmare"},
	3: {"friendly_fire"},
	4: {"friendly_fire", "nightmare"},
	5: {"friendly_fire", "nightmare", "infinity"},
	7: {},
	8: {},
	9: {},
}

// HostPropertyToWestern projects a measured 208-byte JP File #2 host profile
// into the 240-byte PAL/US host profile (+208=1, +220=1, +224=rows, +233..+238=rules).
func HostPropertyToWestern(jpProfile []byte, area int, ruleValues []byte, difficulty int) ([]byte, error) {
	if len(jpProfile) != JPProfileSize {
		return nil, errors.New("file2: expected 208-byte JP profile")
	}
	rows, ok := WesternRows[area]
	if !ok {
		return nil, errors.New("file2: unmeasured area")
	}
	extra := JapaneseExtra[area]
	if len(ruleValues) != 3+len(extra) || ruleValues[0] > 2 || ruleValues[1] > 4 ||
		int(ruleValues[2]) != difficulty || difficulty < 0 || difficulty > 3 {
		return nil, errors.New("file2: unmeasured JP rule values")
	}
	named := make(map[string]byte, len(extra))
	for i, name := range extra {
		if ruleValues[3+i] > 1 {
			return nil, errors.New("file2: unmeasured JP extra rule value")
		}
		named[name] = ruleValues[3+i]
	}

	out := make([]byte, WesternProfileSz)
	copy(out[:JPProfileSize], jpProfile)
	out[208] = 1
	out[220] = 1
	out[224] = byte(rows)
	out[233] = ruleValues[0]
	out[234] = ruleValues[1]
	out[235] = byte(difficulty)
	out[236] = named["friendly_fire"]
	if rows >= 5 {
		out[237] = named["nightmare"]
	}
	if rows >= 6 {
		out[238] = named["infinity"]
	}
	return out, nil
}

// EncodeStatWord encodes a File #2 waiting room STAT word (0x020aWWSS).
func EncodeStatWord(scenario, players, waitIndex int) (uint32, error) {
	if scenario < 1 || scenario > ScenarioCount || players < 2 || players > 4 || waitIndex < 0 || waitIndex > 4 {
		return 0, errors.New("file2: invalid STAT parameters")
	}
	ww := uint32((waitIndex << 3) | ((players - 2) << 1))
	ss := uint32(scenario << 1)
	return 0x020a0000 | (ww << 8) | ss, nil
}

func CharacterCode(profile []byte) (uint16, error) {
	if len(profile) != JPProfileSize && len(profile) != WesternProfileSz {
		return 0, errors.New("file2: invalid profile length")
	}
	return binary.LittleEndian.Uint16(profile[200:202]), nil
}

// Definition declares the measured shared File2 areas and native profile sizes.
func Definition() modules.Definition {
	return modules.Definition{NativeSharedHallAreas: []uint32{1}, NativeChatAreas: []uint32{1}, NativeRoomChat: true, AftergameJournalBytes: 2048, SNAPAftergameProfile: AftergameProfile, KDDIEmptyPlayerScore: EmptyPlayerScore, KDDIAreaExitPayload: make([]byte, 6), KDDIMedia: []uint16{0x0011, 0x0012}, KDDIDownloadManifest: []byte{0}, KDDIHeartbeatPayload: []byte{0, 2, 0, 1, 3, 0xe7, 0, 1}, KDDIVersionReply: versionReply, KDDILobbyLoginSuffix: []byte{0, 2, 0, 0}, WaitingRule: 1, WaitingSeconds: []uint16{180, 300, 600, 900, 1800}, SNAPWaitingRules: WaitingRules, SNAPEntryGreeting: true, MixedEnginePeers: 4, SNAPLaunchRules: LaunchRules, KDDI: kddiDirectory(), KDDICreation: creationRules, ResultScenarios: 11, ResultModes: 8, SNAPTitles: map[uint32]string{0xCA19: "ntsc-u", 0xCAF3: "pal"}, Game: world.GameFile2, Regions: map[string][]string{"snap": {"pal", "ntsc-u"}, "kddi": {"ntsc-j"}}, ProfileBytes: map[string]int{"snap": WesternProfileSz, "kddi": JPProfileSize}, Areas: []int{1, 2, 3, 4, 5, 7, 8, 9}, ScenarioMin: 1, ScenarioMax: ScenarioCount, PlayersMin: 2, PlayersMax: 4, DifficultyMax: 3}
}
