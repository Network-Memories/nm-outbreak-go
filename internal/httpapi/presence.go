package httpapi

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"networkmemories/nm-outbreak-go/internal/core"
	"networkmemories/nm-outbreak-go/internal/modules/file1"
	"networkmemories/nm-outbreak-go/internal/modules/file2"
	"networkmemories/nm-outbreak-go/internal/world"
)

type publicParticipant struct {
	AccountID     string `json:"account_id"`
	NMUserID      int64  `json:"nm_user_id,omitempty"`
	Name          string `json:"name"`
	Region        string `json:"region"`
	Host          bool   `json:"host"`
	CharacterID   *int   `json:"character_id,omitempty"`
	CharacterName string `json:"character_name"`
	PortraitKey   string `json:"character_portrait_key"`
}

type publicRoom struct {
	ID           string              `json:"id"`
	Title        string              `json:"title"`
	AreaID       int                 `json:"area_id,omitempty"`
	ScenarioID   int                 `json:"scenario_id"`
	DifficultyID int                 `json:"difficulty_id"`
	MaxPlayers   int                 `json:"maxplayers"`
	Status       string              `json:"status"`
	Private      bool                `json:"private"`
	Participants []publicParticipant `json:"participants"`
}

var mainCharacters = [...]string{"Kevin", "Mark", "Jim", "George", "David", "Alyssa", "Yoko", "Cindy"}

var portraitProfileSizes = map[world.Game]map[string]int{
	world.GameFile1: {"ntsc-j": file1.JPProfileSize, "ntsc-u": file1.USProfileSize},
	world.GameFile2: {"ntsc-j": file2.JPProfileSize, "pal": file2.WesternProfileSz, "ntsc-u": file2.WesternProfileSz},
}

func characterPortrait(game world.Game, profile []byte, region string) (*int, string, string) {
	size := portraitProfileSizes[game][region]
	var empty [file2.WesternProfileSz]byte
	if size == 0 || len(profile) != size || bytes.Equal(profile, empty[:size]) ||
		binary.LittleEndian.Uint16(profile[202:204]) != 0 {
		return nil, "???", "unknown"
	}
	// Both games store their primary character code at +200 (LE16).
	code := int(binary.LittleEndian.Uint16(profile[200:202]))
	if code >= len(mainCharacters) {
		return nil, "???", "unknown"
	}
	name := mainCharacters[code]
	return &code, name, strings.ToLower(name)
}

func (s *Server) EnablePresence(engine *core.Engine, nmIdentity bool) {
	instance := strconv.FormatInt(time.Now().UnixNano(), 36)
	mux := s.srv.Handler.(*http.ServeMux)
	mux.HandleFunc("/nm/v1/outbreak/state", presenceHandler(engine, world.GameFile1, "00000002", instance, nmIdentity))
	mux.HandleFunc("/nm/v1/outbreak2/state", presenceHandler(engine, world.GameFile2, "00000010", instance, nmIdentity))
}

func presenceHandler(engine *core.Engine, game world.Game, code, instance string, nmIdentity bool) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		rooms, waiting := engine.Presence(game)
		project := func(member world.Member, host bool) publicParticipant {
			protocol, _, _ := strings.Cut(member.Adapter, "-")
			player := publicParticipant{AccountID: protocol + ":" + member.OwnerID, Name: member.Name, Region: member.Region,
				Host: host, CharacterName: "???", PortraitKey: "unknown"}
			if nmIdentity {
				player.NMUserID, _ = strconv.ParseInt(member.OwnerID, 10, 64)
			}
			player.CharacterID, player.CharacterName, player.PortraitKey = characterPortrait(game, member.Profile, member.Region)
			return player
		}
		feed := make([]publicRoom, 0, len(rooms))
		for _, room := range rooms {
			row := publicRoom{ID: room.ID, Title: room.Title, AreaID: room.Area, ScenarioID: room.Scenario, DifficultyID: room.Difficulty, MaxPlayers: room.MaxPlayers, Private: room.Protected,
				Status: "En attente", Participants: make([]publicParticipant, 0, len(room.Members))}
			if room.Playing {
				row.Status = "En partie"
			}
			for _, member := range room.Members {
				row.Participants = append(row.Participants, project(member, member.Connection == room.HostConn))
			}
			feed = append(feed, row)
		}
		lobby := make([]publicParticipant, 0, len(waiting))
		for _, member := range waiting {
			lobby = append(lobby, project(member, false))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"schema_version": 1, "game_id": code, "instance": instance,
			"observed_at_ms": time.Now().UnixMilli(), "rooms": feed, "waiting": lobby,
		})
	}
}
