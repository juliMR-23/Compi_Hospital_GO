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
	for _, s := range st.Staff {
		if s.Age == 0 {
			t.Errorf("staff %s has no age, want the embedded Person data", s.ID)
		}
	}
	hasUnderCare := false
	for _, s := range st.Staff {
		if len(s.UnderCare) > 0 {
			hasUnderCare = true
		}
	}
	if !hasUnderCare {
		t.Error("no doctor card exposes the patients under care")
	}
}

func TestActionDiagnoseAssignsDoctor(t *testing.T) {
	s := newTestServer(t)

	var target patientDTO
	for _, p := range getState(t, s).Patients {
		if p.StateKey == "Awake" && p.Doctor == nil {
			target = p
			break
		}
	}
	if target.ID == "" {
		t.Fatal("scenario has no awake patient without a doctor")
	}

	code, st := post(t, s, fmt.Sprintf(`{"type":"diagnose","patient":%q}`, target.ID))
	if code != http.StatusOK {
		t.Fatalf("diagnose status = %d, want 200", code)
	}
	if got := patientByID(st, target.ID); got.Doctor == nil {
		t.Fatalf("after diagnose patient %s has no doctor", target.ID)
	}
}

func TestActionAttendWithSpecificStaff(t *testing.T) {
	s := newTestServer(t)
	before := getState(t, s)

	orderly := ""
	for _, st := range before.Staff {
		if st.Role == "Orderly" {
			orderly = st.ID
			break
		}
	}
	if orderly == "" {
		t.Fatal("scenario has no orderly to act with")
	}

	var target patientDTO
	for _, p := range before.Patients {
		if p.StateKey == "Awake" {
			target = p
			break
		}
	}
	if target.ID == "" {
		t.Fatal("scenario has no awake patient")
	}
	if code, _ := post(t, s, fmt.Sprintf(`{"type":"sleep","patient":%q,"location":"pasillo"}`, target.ID)); code != http.StatusOK {
		t.Fatalf("sleep status = %d, want 200", code)
	}

	episodesBefore := 0
	for _, st := range before.Staff {
		if st.ID == orderly {
			episodesBefore = len(st.Episodes)
		}
	}

	code, st := post(t, s, fmt.Sprintf(`{"type":"attend","patient":%q,"staff":%q}`, target.ID, orderly))
	if code != http.StatusOK {
		t.Fatalf("attend status = %d, want 200", code)
	}
	if len(st.History) != len(before.History)+1 {
		t.Errorf("history = %d, want %d", len(st.History), len(before.History)+1)
	}
	for _, staff := range st.Staff {
		if staff.ID == orderly && len(staff.Episodes) != episodesBefore+1 {
			t.Errorf("orderly episodes = %d, want %d", len(staff.Episodes), episodesBefore+1)
		}
	}
}

func TestActionAttendForwardsModelError(t *testing.T) {
	s := newTestServer(t)
	before := getState(t, s)

	var awake patientDTO
	for _, p := range before.Patients {
		if p.StateKey == "Awake" {
			awake = p
			break
		}
	}
	if awake.ID == "" {
		t.Fatal("scenario has no awake patient")
	}
	orderly := ""
	for _, st := range before.Staff {
		if st.Role == "Orderly" {
			orderly = st.ID
			break
		}
	}
	if orderly == "" {
		t.Fatal("scenario has no orderly")
	}

	// The panel does not pre-check "the patient must be asleep"; the model does.
	if code, _ := post(t, s, fmt.Sprintf(`{"type":"attend","patient":%q,"staff":%q}`, awake.ID, orderly)); code != http.StatusConflict {
		t.Errorf("attend awake patient status = %d, want 409", code)
	}
	if code, _ := post(t, s, fmt.Sprintf(`{"type":"attend","patient":%q,"staff":"STF-999"}`, awake.ID)); code != http.StatusConflict {
		t.Errorf("attend unknown staff status = %d, want 409", code)
	}
}

func TestActionHireStaffAndRegisterRoom(t *testing.T) {
	s := newTestServer(t)
	before := getState(t, s)

	code, st := post(t, s, `{"type":"hire-doctor","name":"Nuevo Galeno","age":39,"specialty":"Cardiología"}`)
	if code != http.StatusOK {
		t.Fatalf("hire-doctor status = %d, want 200", code)
	}
	if len(st.Doctors) != len(before.Doctors)+1 || len(st.Staff) != len(before.Staff)+1 {
		t.Fatalf("after hire-doctor doctors=%d staff=%d, want +1 each", len(st.Doctors), len(st.Staff))
	}

	code, st = post(t, s, `{"type":"hire-orderly","name":"Nuevo Camillero","age":25}`)
	if code != http.StatusOK {
		t.Fatalf("hire-orderly status = %d, want 200", code)
	}
	if len(st.Staff) != len(before.Staff)+2 {
		t.Fatalf("after hire-orderly staff=%d, want %d", len(st.Staff), len(before.Staff)+2)
	}

	code, st = post(t, s, `{"type":"register-room","number":104,"capacity":2}`)
	if code != http.StatusOK {
		t.Fatalf("register-room status = %d, want 200", code)
	}
	if len(st.Rooms) != len(before.Rooms)+1 {
		t.Fatalf("after register-room rooms=%d, want %d", len(st.Rooms), len(before.Rooms)+1)
	}
}

func TestActionHireErrorsForwardedFromModel(t *testing.T) {
	s := newTestServer(t)

	// An invalid room number and an empty specialty are rejected by the model,
	// so the UI only has to display the error it receives.
	if code, _ := post(t, s, `{"type":"register-room","number":-1,"capacity":1}`); code != http.StatusConflict {
		t.Errorf("negative room number status = %d, want 409", code)
	}
	if code, _ := post(t, s, `{"type":"hire-doctor","name":"X","age":40,"specialty":""}`); code != http.StatusConflict {
		t.Errorf("empty specialty status = %d, want 409", code)
	}
}

func TestActionAttendSleepRejectsAwakePatient(t *testing.T) {
	s := newTestServer(t)

	var awake patientDTO
	for _, p := range getState(t, s).Patients {
		if p.StateKey == "Awake" {
			awake = p
			break
		}
	}
	if awake.ID == "" {
		t.Fatal("scenario has no awake patient")
	}
	if code, _ := post(t, s, fmt.Sprintf(`{"type":"attend-sleep","patient":%q}`, awake.ID)); code != http.StatusConflict {
		t.Errorf("attend-sleep on awake patient status = %d, want 409", code)
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
