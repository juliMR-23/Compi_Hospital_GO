package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestGame(t *testing.T) *game {
	t.Helper()
	g, err := newGame()
	if err != nil {
		t.Fatalf("newGame: %v", err)
	}
	return g
}

func getWard(t *testing.T, g *game) wardDTO {
	t.Helper()
	rec := httptest.NewRecorder()
	g.handleWard(rec, httptest.NewRequest(http.MethodGet, "/api/ward", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/ward = %d, want 200", rec.Code)
	}
	var ward wardDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &ward); err != nil {
		t.Fatalf("decode ward: %v", err)
	}
	return ward
}

func post(t *testing.T, h http.HandlerFunc, body string) (int, wardDTO) {
	t.Helper()
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodPost, "/api/action", bytes.NewBufferString(body)))
	var ward wardDTO
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &ward); err != nil {
			t.Fatalf("decode ward: %v", err)
		}
	}
	return rec.Code, ward
}

func patientByID(ward wardDTO, id string) patientDTO {
	for _, p := range ward.Patients {
		if p.ID == id {
			return p
		}
	}
	return patientDTO{}
}

func awakeCount(ward wardDTO) int {
	n := 0
	for _, p := range ward.Patients {
		if p.StateKey == "Awake" {
			n++
		}
	}
	return n
}

func TestWardSeedsBothBedsAndHallway(t *testing.T) {
	ward := getWard(t, newTestGame(t))

	if ward.Total != len(patientRoster) {
		t.Errorf("total = %d, want %d", ward.Total, len(patientRoster))
	}
	if len(ward.Staff) != 3 {
		t.Errorf("staff = %d, want 3", len(ward.Staff))
	}
	if len(ward.Rooms) != 6 {
		t.Errorf("rooms = %d, want 6 registered beds", len(ward.Rooms))
	}
	if len(ward.Hallway) == 0 {
		t.Error("hallway is empty, want the seeded waiting patients")
	}
	if len(ward.History) == 0 {
		t.Error("history is empty, want the seeded dispatch episodes")
	}
	if len(ward.SevereReport) == 0 {
		t.Error("severe report is empty, want the severe patients")
	}
	// A couple of patients start awake, so the player has someone to interact
	// with; the rest are asleep.
	if ward.Awake != len(patientRoster)-len(ward.Hallway)-2 {
		t.Errorf("awake = %d, want %d", ward.Awake, len(patientRoster)-len(ward.Hallway)-2)
	}
}

func TestSeedFillsDoctorEpisodesAndCare(t *testing.T) {
	ward := getWard(t, newTestGame(t))

	episodes := 0
	for _, d := range ward.Doctors {
		episodes += len(d.Episodes)
	}
	for _, s := range ward.Staff {
		episodes += len(s.Episodes)
	}
	if episodes == 0 {
		t.Error("no attender has episodes, want the seeded dispatch to record them")
	}
}

func TestWakeRousesOnePatient(t *testing.T) {
	g := newTestGame(t)
	ward := getWard(t, g)

	var target patientDTO
	for _, p := range ward.Patients {
		if p.StateKey != "Awake" {
			target = p
			break
		}
	}
	if target.ID == "" {
		t.Fatal("no sleeping patient to wake")
	}
	before := awakeCount(ward)

	code, after := post(t, g.handleWake, `{"patient":"`+target.ID+`"}`)
	if code != http.StatusOK {
		t.Fatalf("wake status = %d, want 200", code)
	}
	if got := patientByID(after, target.ID); got.StateKey != "Awake" {
		t.Errorf("patient %s = %q, want Awake", target.ID, got.StateKey)
	}
	if awakeCount(after) != before+1 {
		t.Errorf("awake = %d, want %d", awakeCount(after), before+1)
	}
}

func TestWakeIsIdempotent(t *testing.T) {
	g := newTestGame(t)
	ward := getWard(t, g)

	var target patientDTO
	for _, p := range ward.Patients {
		if p.StateKey == "Awake" {
			target = p
			break
		}
	}
	if target.ID == "" {
		t.Fatal("no awake patient to re-wake")
	}

	code, after := post(t, g.handleWake, `{"patient":"`+target.ID+`"}`)
	if code != http.StatusOK {
		t.Fatalf("wake already-awake status = %d, want 200", code)
	}
	if awakeCount(after) != awakeCount(ward) {
		t.Errorf("awake = %d after no-op wake, want %d", awakeCount(after), awakeCount(ward))
	}
}

func TestVallenatoWakesSeveral(t *testing.T) {
	g := newTestGame(t)
	ward := getWard(t, g)

	var ids []string
	for _, p := range ward.Patients {
		if p.StateKey != "Awake" {
			ids = append(ids, p.ID)
			if len(ids) == 3 {
				break
			}
		}
	}
	before := awakeCount(ward)

	body, _ := json.Marshal(actionRequest{Patients: ids})
	code, after := post(t, g.handleVallenato, string(body))
	if code != http.StatusOK {
		t.Fatalf("vallenato status = %d, want 200", code)
	}
	if awakeCount(after) != before+len(ids) {
		t.Errorf("awake = %d, want %d", awakeCount(after), before+len(ids))
	}
}

func TestCareHallwayAssignsBedsAndDispatches(t *testing.T) {
	g := newTestGame(t)
	before := getWard(t, g)
	if len(before.Hallway) == 0 {
		t.Fatal("ward starts with nobody in a hallway")
	}

	g.mu.Lock()
	g.careHallwayLocked()
	after := g.wardLocked()
	g.mu.Unlock()

	if len(after.History) <= len(before.History) {
		t.Errorf("history = %d, want more than %d after caring for the hallway", len(after.History), len(before.History))
	}
	if len(after.Hallway) >= len(before.Hallway) {
		t.Errorf("hallway = %d, want fewer than %d after beds were assigned", len(after.Hallway), len(before.Hallway))
	}
	for _, id := range before.Hallway {
		if p := patientByID(after, id); p.StateKey == "AsleepInBed" && p.Room == nil {
			t.Errorf("patient %s is in bed without a room", id)
		}
	}
}

func TestResleepPutsSomeoneToSleep(t *testing.T) {
	g := newTestGame(t)
	before := awakeCount(getWard(t, g))

	g.mu.Lock()
	g.resleepLocked()
	after := awakeCount(g.wardLocked())
	g.mu.Unlock()

	if after != before-1 {
		t.Errorf("awake = %d after one sleep attack, want %d", after, before-1)
	}
}

func TestDispatchEndpoints(t *testing.T) {
	g := newTestGame(t)
	ward := getWard(t, g)

	var sleeping patientDTO
	for _, p := range ward.Patients {
		if p.StateKey != "Awake" {
			sleeping = p
			break
		}
	}
	if sleeping.ID == "" {
		t.Fatal("no sleeping patient to dispatch")
	}

	beforeHistory := len(ward.History)
	code, after := post(t, g.handleDispatch, `{"patient":"`+sleeping.ID+`"}`)
	if code != http.StatusOK {
		t.Fatalf("dispatch status = %d, want 200", code)
	}
	if len(after.History) != beforeHistory+1 {
		t.Errorf("history = %d, want %d after dispatch", len(after.History), beforeHistory+1)
	}
}

func TestDispatchRejectsAwakePatient(t *testing.T) {
	g := newTestGame(t)
	ward := getWard(t, g)

	var awake patientDTO
	for _, p := range ward.Patients {
		if p.StateKey == "Awake" {
			awake = p
			break
		}
	}
	if awake.ID == "" {
		t.Fatal("no awake patient")
	}
	if code, _ := post(t, g.handleDispatch, `{"patient":"`+awake.ID+`"}`); code != http.StatusConflict {
		t.Errorf("dispatch on awake patient status = %d, want 409", code)
	}
}

func TestWakeUnknownPatient(t *testing.T) {
	g := newTestGame(t)
	if code, _ := post(t, g.handleWake, `{"patient":"P-999"}`); code != http.StatusConflict {
		t.Errorf("unknown patient status = %d, want 409", code)
	}
}

func TestResetRebuildsTheWard(t *testing.T) {
	g := newTestGame(t)

	// Wake everyone first, then reset and check the ward is seeded again.
	ward := getWard(t, g)
	for _, p := range ward.Patients {
		if p.StateKey != "Awake" {
			post(t, g.handleWake, `{"patient":"`+p.ID+`"}`)
		}
	}

	code, after := post(t, g.handleReset, `{}`)
	if code != http.StatusOK {
		t.Fatalf("reset status = %d, want 200", code)
	}
	if len(after.Hallway) == 0 || len(after.History) == 0 {
		t.Errorf("after reset hallway=%d history=%d, want a fresh seeded ward", len(after.Hallway), len(after.History))
	}
}

func TestWakeRejectsGet(t *testing.T) {
	g := newTestGame(t)
	rec := httptest.NewRecorder()
	g.handleWake(rec, httptest.NewRequest(http.MethodGet, "/api/wake", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET /api/wake status = %d, want 405", rec.Code)
	}
}
