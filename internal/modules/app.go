package modules

import "networkmemories/nm-outbreak-go/internal/world"

// AppDefinition supplies title data to the shared Capcom APP transport.
// Commands specific to a title are selected by the declared build dialect.
type AppDefinition struct {
	Game                world.Game
	Builds              []byte
	Files               [][]byte
	Token               []byte
	AreaKey             string
	AreaCount, Capacity uint16
}

func (d AppDefinition) Clone() AppDefinition {
	d.Builds = append([]byte(nil), d.Builds...)
	d.Token = append([]byte(nil), d.Token...)
	files := make([][]byte, len(d.Files))
	for i, b := range d.Files {
		files[i] = append([]byte(nil), b...)
	}
	d.Files = files
	return d
}
