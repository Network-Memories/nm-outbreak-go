package file2

import (
	"embed"
	"networkmemories/nm-outbreak-go/internal/modules"
)

// Generated game configuration data from openSNAP outbreak2/netbio.py, AGPL-3.0.
// Measured Area rules in appdata/areas.json; no executable client patch.
//
//go:embed appdata/*.bin
var appFiles embed.FS

func AppDefinition() modules.AppDefinition {
	a, _ := appFiles.ReadFile("appdata/0.bin")
	b, _ := appFiles.ReadFile("appdata/1.bin")
	token, _ := appFiles.ReadFile("appdata/token.bin")
	return modules.AppDefinition{Game: Definition().Game, Builds: []byte{4, 6}, Files: [][]byte{a, b}, Token: token, AreaKey: "area", AreaCount: 10, Capacity: 400}
}
