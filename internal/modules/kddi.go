package modules

// Native directory configuration belongs to the game module. Wire formats and
// authenticated browse state belong to the common KDDI adapter and core.
type KDDIArea struct {
	Rules             []KDDIRule
	Name, Description string
	Status            byte
	SharedArea        int
	SharedByRoom      bool
}
type KDDIRule struct {
	Name     string
	Choices  []string
	Default  byte
	Editable bool
}
type KDDIRoomRules struct {
	Players, Difficulty int
	StatWord            uint32
}
type KDDIRoom struct {
	Name       string
	Status     byte
	SharedArea int
}
type KDDIBrowserEntry struct {
	Label, Info string
	Area        uint16
}

type KDDIDirectory struct {
	BrowserEntries []KDDIBrowserEntry
	Areas          []KDDIArea
	Rooms          []KDDIRoom
	Slots          uint16
}

func (d KDDIDirectory) Shared(area, room uint16) int {
	if area == 0 || int(area) > len(d.Areas) {
		return 0
	}
	a := d.Areas[area-1]
	if a.SharedByRoom {
		if room == 0 || int(room) > len(d.Rooms) {
			return 0
		}
		return d.Rooms[room-1].SharedArea
	}
	return a.SharedArea
}
