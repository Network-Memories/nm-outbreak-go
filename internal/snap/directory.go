package snap

import (
	"fmt"

	"networkmemories/nm-outbreak-go/internal/world"
)

type Area struct {
	ID   uint32
	Name string
}

// DefaultAreas returns the measured SNAP area directory for File #1 or File #2.
func DefaultAreas(game world.Game) []Area {
	if game == world.GameFile1 {
		areas := make([]Area, 0, 16)
		var id uint32 = 1
		areas = append(areas, Area{ID: id, Name: "obmft"})
		id++
		for i := 1; i <= 5; i++ {
			areas = append(areas, Area{ID: id, Name: fmt.Sprintf("obms%02d", i)})
			id++
		}
		for i := 1; i <= 10; i++ {
			areas = append(areas, Area{ID: id, Name: fmt.Sprintf("obmf%02d", i)})
			id++
		}
		return areas
	}
	// File #2: 10 areas (1..10)
	areas := make([]Area, 0, 10)
	for i := 1; i <= 10; i++ {
		areas = append(areas, Area{ID: uint32(i), Name: fmt.Sprintf("area%02d", i)})
	}
	return areas
}

// SharedAreaToLobby maps a shared 1..10 area number to the SNAP lobby_id.
func SharedAreaToLobby(game world.Game, area int) int {
	if game == world.GameFile1 {
		if area >= 1 && area <= 10 {
			return area + 6 // obmf01 is area_id 7
		}
		return 0
	}
	return area
}

// LobbyToSharedArea maps a SNAP lobby_id to the shared 1..10 area number.
func LobbyToSharedArea(game world.Game, lobbyID int) int {
	if game == world.GameFile1 {
		if lobbyID >= 7 && lobbyID <= 16 {
			return lobbyID - 6
		}
		return 0
	}
	return lobbyID
}
