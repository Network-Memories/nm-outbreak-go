// Package modules describes measured game constraints. The shared core does not
// infer one game's native variants from another game's behaviour.
package modules

import "networkmemories/nm-outbreak-go/internal/world"

type Definition struct {
	NativeSharedHallAreas []uint32 // measured cross-dialect population at native room0
	NativeChatAreas       []uint32 // native lobby IDs with measured cross-dialect area scope
	NativeRoomChat        bool     // measured text envelopes; no opaque control packets

	AftergameJournalBytes int // zero until measured for this game

	SNAPAftergameProfile func([]byte) ([]byte, error)
	KDDIEmptyPlayerScore func(number byte) []byte

	KDDIAreaExitPayload []byte
	KDDIMedia           []uint16

	KDDIDownloadManifest []byte // nil: unsupported
	KDDIDownloadBlock    func([]byte) []byte

	KDDIHeartbeatPayload         []byte
	KDDIVersionReply             func(query, reply uint16, payload []byte) bool
	KDDILobbyLoginSuffix         []byte
	WaitingRule                  byte
	WaitingSeconds               []uint16
	SNAPWaitingRules             func(area int, stat uint32, profile []byte) ([]byte, error)
	SNAPEntryGreeting            bool
	MixedEnginePeers             int // maximum roster size measured for the native engine adapter
	SNAPLaunchRules              func(area int, stat uint32, profile []byte) ([]byte, error)
	KDDICreation                 func(area, scenario int, rules []byte) (KDDIRoomRules, error)
	KDDI                         KDDIDirectory
	ResultScenarios, ResultModes int
	Game                         world.Game
	SNAPTitles                   map[uint32]string
	Regions                      map[string][]string
	ProfileBytes                 map[string]int
	Areas                        []int
	ScenarioMin, ScenarioMax     int
	PlayersMin, PlayersMax       int
	DifficultyMax                int
}

func (d Definition) Clone() Definition {
	d.NativeSharedHallAreas = append([]uint32(nil), d.NativeSharedHallAreas...)
	d.NativeChatAreas = append([]uint32(nil), d.NativeChatAreas...)
	d.KDDIAreaExitPayload = append([]byte(nil), d.KDDIAreaExitPayload...)
	d.KDDIMedia = append([]uint16(nil), d.KDDIMedia...)
	d.KDDIDownloadManifest = append([]byte(nil), d.KDDIDownloadManifest...)
	d.KDDIHeartbeatPayload = append([]byte(nil), d.KDDIHeartbeatPayload...)
	d.KDDILobbyLoginSuffix = append([]byte(nil), d.KDDILobbyLoginSuffix...)
	d.WaitingSeconds = append([]uint16(nil), d.WaitingSeconds...)
	d.KDDI.BrowserEntries = append([]KDDIBrowserEntry(nil), d.KDDI.BrowserEntries...)
	d.KDDI.Areas = append([]KDDIArea(nil), d.KDDI.Areas...)
	for i := range d.KDDI.Areas {
		d.KDDI.Areas[i].Rules = append([]KDDIRule(nil), d.KDDI.Areas[i].Rules...)
		for j := range d.KDDI.Areas[i].Rules {
			d.KDDI.Areas[i].Rules[j].Choices = append([]string(nil), d.KDDI.Areas[i].Rules[j].Choices...)
		}
	}
	d.KDDI.Rooms = append([]KDDIRoom(nil), d.KDDI.Rooms...)
	titles := make(map[uint32]string)
	for code, region := range d.SNAPTitles {
		titles[code] = region
	}
	d.SNAPTitles = titles
	regions := make(map[string][]string, len(d.Regions))
	for key, values := range d.Regions {
		regions[key] = append([]string(nil), values...)
	}
	d.Regions = regions
	profiles := make(map[string]int, len(d.ProfileBytes))
	for key, size := range d.ProfileBytes {
		profiles[key] = size
	}
	d.ProfileBytes = profiles
	d.Areas = append([]int(nil), d.Areas...)
	return d
}
func (d Definition) Supports(protocol, region string) bool {
	for _, r := range d.Regions[protocol] {
		if r == region {
			return true
		}
	}
	return false
}
func (d Definition) ValidRoom(area, scenario, players, difficulty int) bool {
	supported := false
	for _, a := range d.Areas {
		if a == area {
			supported = true
			break
		}
	}
	return supported && scenario >= d.ScenarioMin && scenario <= d.ScenarioMax && players >= d.PlayersMin && players <= d.PlayersMax && difficulty >= 0 && difficulty <= d.DifficultyMax
}
