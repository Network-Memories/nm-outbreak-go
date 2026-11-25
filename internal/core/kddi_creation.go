package core

import (
	"errors"
	"networkmemories/nm-outbreak-go/internal/modules"
	"networkmemories/nm-outbreak-go/internal/world"
)

type KDDICreationView struct {
	Slot      uint16
	Title     string
	Scenario  int
	Media     uint16
	Rules     []byte
	Protected bool
}
type kddiDraft struct {
	KDDICreationView
	key      kddiBrowseKey
	password []byte
}

func (e *Engine) KDDICreationForOwner(id string) (KDDICreationView, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if d := e.kddiDrafts[id]; d != nil {
		v := d.KDDICreationView
		v.Rules = append([]byte(nil), d.Rules...)
		return v, true
	}
	return KDDICreationView{}, false
}

func (e *Engine) cancelKDDIDraftLocked(id string) {
	if d := e.kddiDrafts[id]; d != nil {
		for i := range d.password {
			d.password[i] = 0
		}
		delete(e.kddiDrafts, id)
	}
}

func (e *Engine) BeginKDDICreation(id string, slot uint16) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	s := e.sessions[id]
	if s == nil || s.Protocol != KDDI || s.RoomID != "" || s.InScenario || s.Area == 0 {
		return ErrSession
	}
	if d := e.kddiDrafts[id]; d != nil {
		if d.Slot == slot {
			return nil
		}
		return ErrSession
	}
	slots, err := e.kddiSlotsLocked(id)
	if err != nil {
		return err
	}
	config := e.modules[s.Game].KDDI
	if slot == 0 || slot > config.Slots {
		return ErrSession
	}
	if _, ok := slots[slot]; ok {
		return errors.New("native slot occupied")
	}
	key := kddiBrowseKey{s.Game, uint16(s.NativeLobbyID), s.NativeRoomID}
	for _, d := range e.kddiDrafts {
		if d.key == key && d.Slot == slot {
			return errors.New("native slot reserved")
		}
	}
	d := &kddiDraft{key: key, KDDICreationView: KDDICreationView{Slot: slot}}
	for _, rule := range config.Areas[s.NativeLobbyID-1].Rules {
		d.Rules = append(d.Rules, rule.Default)
	}
	if len(d.Rules) < 3 {
		return errors.New("native rules unavailable")
	}
	if e.kddiDrafts == nil {
		e.kddiDrafts = make(map[string]*kddiDraft)
	}
	e.kddiDrafts[id] = d
	return nil
}

// KDDICreation reports a reservation without publishing an incomplete world.
func (e *Engine) KDDICreation(id string, slot uint16) (KDDICreationView, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	s := e.sessions[id]
	if s == nil || s.Protocol != KDDI {
		return KDDICreationView{}, false
	}
	key := kddiBrowseKey{s.Game, uint16(s.NativeLobbyID), s.NativeRoomID}
	for _, d := range e.kddiDrafts {
		if d.key == key && d.Slot == slot {
			v := d.KDDICreationView
			v.Rules = append([]byte(nil), d.Rules...)
			return v, true
		}
	}
	return KDDICreationView{}, false
}

func (e *Engine) SetKDDICreationScene(id string, media uint16, scenario int) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	d := e.kddiDrafts[id]
	s := e.sessions[id]
	if d == nil || s == nil {
		return ErrSession
	}
	m := e.modules[s.Game]
	allowedMedia := false
	for _, allowed := range m.KDDIMedia {
		if allowed == media {
			allowedMedia = true
			break
		}
	}
	if !allowedMedia {
		return ErrSession
	}
	if s.Game == world.GameFile1 {
		if scenario < 0 || scenario >= m.ScenarioMax {
			return errors.New("native scenario outside module")
		}
		scenario++
	} else if scenario < 1 || scenario > m.ScenarioMax {
		return errors.New("native scenario outside module")
	}
	d.Media = media
	d.Scenario = scenario
	return nil
}
func (e *Engine) SetKDDICreationTitle(id, title string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	d := e.kddiDrafts[id]
	if d == nil {
		return ErrSession
	}
	if len(title) > 16 {
		return errors.New("native title too long")
	}
	for _, c := range []byte(title) {
		if c < 32 || c > 126 {
			return errors.New("unmeasured native title encoding")
		}
	}
	d.Title = title
	return nil
}
func (e *Engine) SetKDDICreationPassword(id string, password []byte) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	d := e.kddiDrafts[id]
	if d == nil || len(password) > 64 {
		return ErrSession
	}
	for i := range d.password {
		d.password[i] = 0
	}
	d.password = append([]byte(nil), password...)
	d.Protected = len(password) > 0
	return nil
}
func (e *Engine) SetKDDICreationRule(id string, index, value byte) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	d := e.kddiDrafts[id]
	s := e.sessions[id]
	if d == nil || s == nil || int(index) >= len(d.Rules) {
		return ErrSession
	}
	rule := e.modules[s.Game].KDDI.Areas[d.key.area-1].Rules[index]
	if int(value) >= len(rule.Choices) || (!rule.Editable && value != rule.Default) {
		return errors.New("native rule choice refused")
	}
	d.Rules[index] = value
	return nil
}

func (e *Engine) KDDIRules(id string, slot uint16) ([]modules.KDDIRule, []byte, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	s := e.sessions[id]
	if s == nil || s.Protocol != KDDI || s.NativeLobbyID == 0 || s.NativeRoomID == 0 {
		return nil, nil, ErrSession
	}
	m := e.modules[s.Game].Clone()
	if slot == 0 || slot > m.KDDI.Slots {
		return nil, nil, ErrSession
	}
	schema := m.KDDI.Areas[s.NativeLobbyID-1].Rules
	values := make([]byte, len(schema))
	for i, r := range schema {
		values[i] = r.Default
	}
	key := kddiBrowseKey{s.Game, uint16(s.NativeLobbyID), s.NativeRoomID}
	for _, d := range e.kddiDrafts {
		if d.key == key && d.Slot == slot {
			return schema, append([]byte(nil), d.Rules...), nil
		}
	}
	slots, err := e.kddiSlotsLocked(id)
	if err != nil {
		return nil, nil, err
	}
	if room, ok := slots[slot]; ok {
		values, err = e.nativeRuleValuesLocked(room, schema)
		if err != nil {
			return nil, nil, err
		}
	}
	return schema, values, nil
}

func (e *Engine) CommitKDDICreation(id string) (*world.Room, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	s := e.sessions[id]
	if s == nil || s.Protocol != KDDI {
		return nil, ErrSession
	}
	d := e.kddiDrafts[id]
	if d == nil {
		if r, ok := e.worlds.GetRoom(s.RoomID); ok && r.HostConn == id {
			return r, nil
		}
		return nil, ErrSession
	}
	m := e.modules[s.Game]
	if m.KDDICreation == nil || d.Scenario == 0 || d.Media == 0 {
		return nil, errors.New("native creation incomplete")
	}
	rules, err := m.KDDICreation(s.Area, d.Scenario, d.Rules)
	if err != nil {
		return nil, err
	}
	room, err := e.createRoomLocked(id, RoomOptions{NativeMedia: d.Media, Area: s.Area, Scenario: d.Scenario, MaxPlayers: rules.Players, Difficulty: rules.Difficulty, Title: d.Title, Password: d.password, StatWord: rules.StatWord, RuleValues: append([]byte(nil), d.Rules...)})
	if err != nil {
		return nil, err
	}
	if e.kddiSlots[d.key] == nil {
		e.kddiSlots[d.key] = make(map[string]uint16)
	}
	e.kddiSlots[d.key][room.ID] = d.Slot
	e.cancelKDDIDraftLocked(id)
	return room, nil
}
