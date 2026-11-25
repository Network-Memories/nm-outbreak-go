package file2

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"networkmemories/nm-outbreak-go/internal/modules"
)

//go:embed appdata/areas.json
var areaSettings []byte

// Reuse the APP area's configured labels/enabled state, not a second registry.
// Native room and slot shape comes from BioServer File2 Rooms/Slots.java.
func kddiDirectory() modules.KDDIDirectory {
	var areas []struct {
		Title   string
		Enabled *bool
		Rules   map[string]string
	}
	if err := json.Unmarshal(areaSettings, &areas); err != nil {
		panic(err)
	}
	d := modules.KDDIDirectory{BrowserEntries: []modules.KDDIBrowserEntry{{Label: "ALPHA SERVER", Info: "Please select the server."}}, Slots: 20}
	for i, a := range areas {
		status := byte(3)
		if a.Enabled != nil && !*a.Enabled {
			status = 0
		}
		rules := []modules.KDDIRule{
			{Name: "Nombre de joueurs", Choices: []string{"Deux joueurs", "Trois joueurs", "Quatre joueurs"}, Default: 2, Editable: true},
			{Name: "Limite de temps", Choices: []string{"Trois minutes", "Cinq minutes", "Dix minutes", "Quinze minutes", "Trente minutes"}, Default: 1, Editable: true},
			{Name: "Difficulte", Choices: []string{"Facile", "Normal", "Difficile", "Tres difficile"}, Default: 1, Editable: true},
		}
		flag := func(name, key string, value byte, editable bool) {
			// The native 0/1 choice is measured; the common area configuration
			// decides whether that choice is fixed or left to the host.
			switch a.Rules[key] {
			case "on":
				value = 1
				editable = false
			case "off":
				value = 0
				editable = false
			case "host":
				editable = true
			}
			rules = append(rules, modules.KDDIRule{Name: name, Choices: []string{"Non", "Oui"}, Default: value, Editable: editable})
		}
		switch i + 1 {
		case 2:
			flag("Cauchemar", "nightmare", 1, false)
		case 3:
			flag("Tir allie", "friendly_fire", 0, false)
		case 4:
			flag("Tir allie", "friendly_fire", 1, false)
			flag("Cauchemar", "nightmare", 1, false)
		case 5:
			flag("Tir allie", "friendly_fire", 1, true)
			flag("Cauchemar", "nightmare", 0, true)
			flag("Infinie", "infinity", 1, false)
		}
		d.Areas = append(d.Areas, modules.KDDIArea{Name: a.Title, Status: status, SharedArea: i + 1, Rules: rules})
	}
	d.Rooms = append(d.Rooms, modules.KDDIRoom{Name: "free", Status: 3})
	for i := 1; i <= 5; i++ {
		d.Rooms = append(d.Rooms, modules.KDDIRoom{Name: fmt.Sprintf("R%d", i)})
	}
	return d
}

func creationRules(area, scenario int, r []byte) (modules.KDDIRoomRules, error) {
	if _, ok := WesternRows[area]; !ok || len(r) != 3+len(JapaneseExtra[area]) {
		return modules.KDDIRoomRules{}, errors.New("unmeasured File2 creation")
	}
	if r[0] > 2 || r[1] > 4 || r[2] > 3 {
		return modules.KDDIRoomRules{}, errors.New("invalid File2 rule")
	}
	for _, v := range r[3:] {
		if v > 1 {
			return modules.KDDIRoomRules{}, errors.New("invalid File2 flag")
		}
	}
	stat, err := EncodeStatWord(scenario, int(r[0])+2, int(r[1]))
	return modules.KDDIRoomRules{Players: int(r[0]) + 2, Difficulty: int(r[2]), StatWord: stat}, err
}
