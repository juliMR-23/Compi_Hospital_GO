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

func TestWardStartsAsleep(t *testing.T) {
	ward := getWard(t, newTestGame(t))

	if ward.Total != len(patientRoster) {
		t.Errorf("total = %d, want %d", ward.Total, len(patientRoster))
	}
	if ward.Awake != 0 {
		t.Errorf("awake = %d, want 0 at the start", ward.Awake)
	}
	if len(ward.Staff) != 3 {
		t.Errorf("staff = %d, want 3", len(ward.Staff))
	}
	for _, p := range ward.Patients {
		if p.StateKey != "AsleepInHallway" {
			t.Errorf("patient %s starts %q, want AsleepInHallway", p.ID, p.StateKey)
		}
	}
}

func TestWakeRousesOnePatient(t *testing.T) {
	g := newTestGame(t)
	ward := getWard(t, g)
	target := ward.Patients[0].ID

	code, after := post(t, g.handleWake, `{"patient":"`+target+`"}`)
	if code != http.StatusOK {
		t.Fatalf("wake status = %d, want 200", code)
	}
	if after.Awake != 1 {
		t.Fatalf("awake = %d, want 1", after.Awake)
	}
	for _, p := range after.Patients {
		if p.ID == target && p.StateKey != "Awake" {
			t.Errorf("patient %s = %q, want Awake", target, p.StateKey)
		}
	}
}

func TestWakeIsIdempotent(t *testing.T) {
	g := newTestGame(t)
	target := getWard(t, g).Patients[1].ID

	post(t, g.handleWake, `{"patient":"`+target+`"}`)
	code, after := post(t, g.handleWake, `{"patient":"`+target+`"}`)
	if code != http.StatusOK {
		t.Fatalf("second wake status = %d, want 200", code)
	}
	if after.Awake != 1 {
		t.Errorf("awake = %d after waking the same patient twice, want 1", after.Awake)
	}
}

func TestVallenatoWakesSeveral(t *testing.T) {
	g := newTestGame(t)
	ward := getWard(t, g)
	ids := []string{ward.Patients[0].ID, ward.Patients[1].ID, ward.Patients[2].ID}

	body, _ := json.Marshal(actionRequest{Patients: ids})
	code, after := post(t, g.handleVallenato, string(body))
	if code != http.StatusOK {
		t.Fatalf("vallenato status = %d, want 200", code)
	}
	if after.Awake != len(ids) {
		t.Errorf("awake = %d, want %d", after.Awake, len(ids))
	}
}

func TestWakeUnknownPatient(t *testing.T) {
	g := newTestGame(t)
	if code, _ := post(t, g.handleWake, `{"patient":"P-999"}`); code != http.StatusConflict {
		t.Errorf("unknown patient status = %d, want 409", code)
	}
}

func TestResetPutsEveryoneBackToSleep(t *testing.T) {
	g := newTestGame(t)
	target := getWard(t, g).Patients[0].ID
	post(t, g.handleWake, `{"patient":"`+target+`"}`)

	code, after := post(t, g.handleReset, `{}`)
	if code != http.StatusOK {
		t.Fatalf("reset status = %d, want 200", code)
	}
	if after.Awake != 0 {
		t.Errorf("awake = %d after reset, want 0", after.Awake)
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
