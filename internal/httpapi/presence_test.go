package httpapi

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"networkmemories/nm-outbreak-go/internal/core"
	"networkmemories/nm-outbreak-go/internal/world"
)

func TestPublicPresenceProjectsNativePlayersWithoutPrivateFields(t *testing.T) {
	reg := world.NewRegistry()
	profile := make([]byte, 240)
	profile[200] = 1 // PAL main character Mark.
	reg.CreateRoom(world.Room{Game: world.GameFile2, HostConn: "private-connection",
		Title: "Wild Things", Scenario: 1, Difficulty: 1, MaxPlayers: 2, Protected: true, Playing: true,
		Members: []world.Member{
			{Connection: "private-connection", Adapter: "snap-file2", OwnerID: "7", Name: "PALname", Region: "pal", Profile: profile},
			{Connection: "other-private-connection", Adapter: "kddi-file2", OwnerID: "8", Name: "あ", Region: "ntsc-j", Handle: "ABC123"},
		},
	})
	e := core.New(reg, nil)
	for _, useNM := range []bool{true, false} {
		s := NewServer("127.0.0.1:0", reg, nil)
		s.EnablePresence(e, useNM)
		response := httptest.NewRecorder()
		s.srv.Handler.ServeHTTP(response, httptest.NewRequest("GET", "/nm/v1/outbreak2/state", nil))
		var feed struct {
			SchemaVersion int                 `json:"schema_version"`
			GameID        string              `json:"game_id"`
			Rooms         []publicRoom        `json:"rooms"`
			Waiting       []publicParticipant `json:"waiting"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &feed); err != nil {
			t.Fatal(err)
		}
		if response.Code != 200 || feed.SchemaVersion != 1 || feed.GameID != "00000010" || len(feed.Rooms) != 1 || feed.Waiting == nil {
			t.Fatalf("incorrect presence format: %s", response.Body)
		}
		room := feed.Rooms[0]
		if room.ScenarioID != 1 || room.DifficultyID != 1 || room.Status != "En partie" || !room.Private || room.MaxPlayers != 2 || len(room.Participants) != 2 {
			t.Fatalf("incorrect room projection: %+v", room)
		}
		host, guest := room.Participants[0], room.Participants[1]
		if !host.Host || host.Name != "PALname" || host.PortraitKey != "mark" || host.CharacterID == nil || *host.CharacterID != 1 || guest.Host || guest.Name != "あ" || guest.PortraitKey != "unknown" || guest.CharacterID != nil {
			t.Fatalf("incorrect participant projection: %+v", room.Participants)
		}
		wantID := int64(0)
		if useNM {
			wantID = 7
		}
		if host.NMUserID != wantID || strings.Contains(response.Body.String(), "private-connection") || strings.Contains(response.Body.String(), "ABC123") {
			t.Fatal("presence exposes private state or an unproved NM identifier")
		}
	}
}

func TestCharacterPortraitUsesNativeSelection(t *testing.T) {
	for game, regions := range portraitProfileSizes {
		for region, size := range regions {
			profile := make([]byte, size)
			profile[200], profile[206] = 1, 1
			code, name, portrait := characterPortrait(game, profile, region)
			if code == nil || *code != 1 || name != "Mark" || portrait != "mark" {
				t.Fatalf("%s %s: Mark not recognized", game, region)
			}
			profile[202] = 1
			if code, _, portrait := characterPortrait(game, profile, region); code != nil || portrait != "unknown" {
				t.Fatalf("%s %s: secondary character got a main portrait", game, region)
			}
		}
	}
}
