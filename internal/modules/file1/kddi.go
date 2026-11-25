package file1

import (
	"errors"
	"fmt"
	"networkmemories/nm-outbreak-go/internal/modules"
)

// Directory shape and status bytes from BioServer File1 Areas/Rooms/Slots.java.
func kddiDirectory() modules.KDDIDirectory {
	d := modules.KDDIDirectory{BrowserEntries: []modules.KDDIBrowserEntry{{Label: "WEST TOWN", Area: 2, Info: "Free AREA"}, {Label: "EAST TOWN", Area: 1, Info: "Scenario Mode"}}, Slots: 20, Areas: []modules.KDDIArea{
		{Name: "Quartier Est", Status: 3},
		{Name: "Quartier Ouest", Status: 3, SharedByRoom: true},
	}}
	for i := 1; i <= 10; i++ {
		name := fmt.Sprintf("R%d", i)
		if i == 10 {
			name = "RA"
		}
		d.Rooms = append(d.Rooms, modules.KDDIRoom{Name: name, Status: 3, SharedArea: i})
	}
	for i := range d.Areas {
		d.Areas[i].Rules = []modules.KDDIRule{
			{Name: "Nombre de joueurs", Choices: []string{"Deux joueurs", "Trois joueurs", "Quatre joueurs"}, Default: 2, Editable: true},
			{Name: "Limite de temps", Choices: []string{"Trois minutes", "Cinq minutes", "Dix minutes", "Quinze minutes", "Trente minutes"}, Default: 1, Editable: true},
			{Name: "Difficulte", Choices: []string{"Facile", "Normal", "Difficile", "Tres difficile"}, Default: 1, Editable: true},
			{Name: "Tir allie", Choices: []string{"Non", "Oui"}, Editable: true},
		}
	}
	return d
}

func creationRules(area, scenario int, r []byte) (modules.KDDIRoomRules, error) {
	if area < 1 || area > 10 || scenario < 1 || scenario > 5 || len(r) != 4 || r[0] > 2 || r[1] > 4 || r[2] > 3 || r[3] > 1 {
		return modules.KDDIRoomRules{}, errors.New("unmeasured File1 creation")
	}
	usWait := int(r[1]) - 1
	if usWait < 0 {
		usWait = 0
	}
	stat, err := EncodeStatWord(scenario, int(r[0])+2, usWait, int(r[2]), int(r[3]))
	if err != nil {
		return modules.KDDIRoomRules{}, err
	}
	return modules.KDDIRoomRules{Players: int(r[0]) + 2, Difficulty: int(r[2]), StatWord: stat}, nil
}
