package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestServer(t *testing.T) *server {
	t.Helper()
	h, err := newScenario()
	if err != nil {
		t.Fatalf("newScenario: %v", err)
	}
	return &server{h: h}
}

func getState(t *testing.T, s *server) stateDTO {
	t.Helper()
	rec := httptest.NewRecorder()
	s.handleState(rec, httptest.NewRequest(http.MethodGet, "/api/state", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/state = %d, want 200", rec.Code)
	}
	var st stateDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &st); err != nil {
		t.Fatalf("decode state: %v", err)
	}
	return st
}

func post(t *testing.T, s *server, body string) (int, stateDTO) {
	t.Helper()
	rec := httptest.NewRecorder()
	s.handleAction(rec, httptest.NewRequest(http.MethodPost, "/api/action", bytes.NewBufferString(body)))
	var st stateDTO
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &st); err != nil {
			t.Fatalf("decode state: %v", err)
		}
	}
	return rec.Code, st
}

func patientByID(st stateDTO, id string) patientDTO {
	for _, p := range st.Patients {
		if p.ID == id {
			return p
		}
	}
	return patientDTO{}
}

func TestStateEndpoint(t *testing.T) {
	st := getState(t, newTestServer(t))

	if st.Name == "" {
		t.Fatal("hospital name is empty")
	}
	if len(st.Staff) != 3 {
		t.Errorf("staff = %d, want 3", len(st.Staff))
	}
	if len(st.Doctors) != 2 {
		t.Errorf("doctors = %d, want 2", len(st.Doctors))
	}
	if len(st.Rooms) != 3 {
		t.Errorf("rooms = %d, want 3", len(st.Rooms))
	}
	if len(st.Patients) != 5 {
		t.Errorf("patients = %d, want 5", len(st.Patients))
	}
	if len(st.Hallway) != 1 {
		t.Errorf("hallway patients = %d, want 1", len(st.Hallway))
	}
	if len(st.SevereReport) != 2 {
		t.Errorf("severe report = %d, want 2", len(st.SevereReport))
	}
	if len(st.History) == 0 {
		t.Error("history is empty, want the seeded dispatch episodes")
	}
}

func TestActionAdmit(t *testing.T) {
	s := newTestServer(t)
	code, st := post(t, s, `{"type":"admit","name":"Test Patient","age":31,"level":"Severe"}`)
	if code != http.StatusOK {
		t.Fatalf("admit status = %d, want 200", code)
	}
	if len(st.Patients) != 6 {
		t.Fatalf("patients = %d, want 6 after admit", len(st.Patients))
	}
	found := false
	for _, p := range st.Patients {
		if p.Name == "Test Patient" {
			found = true
			if p.Level != "Severe" || p.StateKey != "Awake" {
				t.Errorf("admitted patient = %+v, want Severe and Awake", p)
			}
		}
	}
	if !found {
		t.Fatal("admitted patient not present in state")
	}
}

func TestActionSleepAssignWake(t *testing.T) {
	s := newTestServer(t)
	st := getState(t, s)

	var target patientDTO
	for _, p := range st.Patients {
		if p.StateKey == "Awake" {
			target = p
			break
		}
	}
	if target.ID == "" {
		t.Fatal("scenario has no awake patient")
	}

	code, st := post(t, s, fmt.Sprintf(`{"type":"sleep","patient":%q,"location":"pasillo"}`, target.ID))
	if code != http.StatusOK {
		t.Fatalf("sleep status = %d, want 200", code)
	}
	if got := patientByID(st, target.ID); got.StateKey != "AsleepInHallway" {
		t.Fatalf("after sleep state = %q, want AsleepInHallway", got.StateKey)
	}

	code, st = post(t, s, fmt.Sprintf(`{"type":"assign-room","patient":%q}`, target.ID))
	if code != http.StatusOK {
		t.Fatalf("assign-room status = %d, want 200", code)
	}
	got := patientByID(st, target.ID)
	if got.StateKey != "AsleepInBed" || got.Room == nil {
		t.Fatalf("after assign-room = %+v, want AsleepInBed with a room", got)
	}

	code, st = post(t, s, fmt.Sprintf(`{"type":"wake","patient":%q}`, target.ID))
	if code != http.StatusOK {
		t.Fatalf("wake status = %d, want 200", code)
	}
	got = patientByID(st, target.ID)
	if got.StateKey != "Awake" || got.Room != nil {
		t.Fatalf("after wake = %+v, want Awake without a room", got)
	}
}

func TestActionErrors(t *testing.T) {
	s := newTestServer(t)

	if code, _ := post(t, s, `{"type":"nonsense"}`); code != http.StatusBadRequest {
		t.Errorf("unknown action status = %d, want 400", code)
	}
	if code, _ := post(t, s, `{"type":"sleep","patient":"P-999"}`); code != http.StatusConflict {
		t.Errorf("unknown patient status = %d, want 409", code)
	}
	if code, _ := post(t, s, `{"type":"admit","name":"","age":30,"level":"Severe"}`); code != http.StatusConflict {
		t.Errorf("admit with empty name status = %d, want 409", code)
	}

	rec := httptest.NewRecorder()
	s.handleAction(rec, httptest.NewRequest(http.MethodGet, "/api/action", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET /api/action status = %d, want 405", rec.Code)
	}
}

func TestActionAttendHallway(t *testing.T) {
	s := newTestServer(t)
	before := getState(t, s)
	if len(before.Hallway) == 0 {
		t.Fatal("scenario should start with a patient asleep in the hallway")
	}

	code, st := post(t, s, `{"type":"attend-hallway"}`)
	if code != http.StatusOK {
		t.Fatalf("attend-hallway status = %d, want 200", code)
	}
	if len(st.History) <= len(before.History) {
		t.Errorf("history = %d, want more than %d after attending the hallway", len(st.History), len(before.History))
	}
}
