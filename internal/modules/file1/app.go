package file1

import (
	"embed"
	"networkmemories/nm-outbreak-go/internal/modules"
)

// Generated game configuration data from openSNAP outbreak/netbio.py, AGPL-3.0.
// This is native NETBIO configuration, never an executable client patch.
//
//go:embed appdata/*.bin
var appFiles embed.FS

func AppDefinition() modules.AppDefinition {
	a, _ := appFiles.ReadFile("appdata/0.bin")
	b, _ := appFiles.ReadFile("appdata/1.bin")
	token, _ := appFiles.ReadFile("appdata/token.bin")
	return modules.AppDefinition{Game: Definition().Game, Builds: []byte{1}, Files: [][]byte{a, b}, Token: token}
}
