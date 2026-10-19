package hospital

import (
	"errors"
	"fmt"
	"sync"
	"testing"
)

func mustHospital(t *testing.T, name string) *Hospital {
	t.Helper()
	h, err := NewHospital(name)
	if err != nil {
		t.Fatalf("NewHospital(%q): %v", name, err)
	}
	return h
}

func mustPatient(t *testing.T, name string, age int, level NarcolepsyLevel) *Patient {
	t.Helper()
	p, err := NewPatient(name, age, level)
	if err != nil {
		t.Fatalf("NewPatient(%q, %d, %v): %v", name, age, level, err)
	}
	return p
}

func mustDoctor(t *testing.T, name string, age int, specialty string) *Doctor {
	t.Helper()
	d, err := NewDoctor(name, age, specialty)
	if err != nil {
		t.Fatalf("NewDoctor(%q, %d, %q): %v", name, age, specialty, err)
	}
	return d
}

func mustOrderly(t *testing.T, name string, age int) *Orderly {
	t.Helper()
	o, err := NewOrderly(name, age)
	if err != nil {
		t.Fatalf("NewOrderly(%q, %d): %v", name, age, err)
	}
	return o
}

func newHospitalWithRooms(t *testing.T, numbers ...int) *Hospital {
	t.Helper()
	h := mustHospital(t, "test hospital")
	for _, n := range numbers {
		room, err := NewRoom(n, 1)
		if err != nil {
			t.Fatalf("NewRoom(%d, 1): %v", n, err)
		}
		if err := h.RegisterRoom(room); err != nil {
			t.Fatalf("RegisterRoom(%d): %v", n, err)
		}
	}
	return h
}

func TestAssignRoomWithFreeRoom(t *testing.T) {
	h := newHospitalWithRooms(t, 101)
	p := mustPatient(t, "Ana", 30, Mild)
	if err := h.AdmitPatient(p); err != nil {
		t.Fatalf("AdmitPatient: %v", err)
	}
	if err := p.SufferSleepAttack("hallway"); err != nil {
		t.Fatalf("SufferSleepAttack: %v", err)
	}

	room, err := h.AssignRoom(p)
	if err != nil {
		t.Fatalf("AssignRoom returned unexpected error: %v", err)
	}
	if room == nil {
		t.Fatal("AssignRoom returned a nil room")
	}
	if p.Room() != room {
		t.Fatalf("patient room = %v, want %v", p.Room(), room)
	}
	if p.State() != AsleepInBed {
		t.Fatalf("patient state = %v, want %v", p.State(), AsleepInBed)
	}
	if room.State() != Occupied {
		t.Fatalf("room state = %v, want %v", room.State(), Occupied)
	}
}

func TestAssignRoomNoFreeRoom(t *testing.T) {
	h := newHospitalWithRooms(t, 101)

	first := mustPatient(t, "Beto", 40, Mild)
	if err := h.AdmitPatient(first); err != nil {
		t.Fatalf("AdmitPatient(first): %v", err)
	}
	if err := first.SufferSleepAttack("hallway"); err != nil {
		t.Fatalf("SufferSleepAttack(first): %v", err)
	}
	if _, err := h.AssignRoom(first); err != nil {
		t.Fatalf("AssignRoom(first): %v", err)
	}

	second := mustPatient(t, "Carla", 50, Severe)
	if err := h.AdmitPatient(second); err != nil {
		t.Fatalf("AdmitPatient(second): %v", err)
	}
	if err := second.SufferSleepAttack("hallway"); err != nil {
		t.Fatalf("SufferSleepAttack(second): %v", err)
	}

	room, err := h.AssignRoom(second)
	if !errors.Is(err, ErrNoFreeRoom) {
		t.Fatalf("err = %v, want it to wrap %v", err, ErrNoFreeRoom)
	}
	if room != nil {
		t.Fatalf("room = %v, want nil when no room is free", room)
	}
	if second.Room() != nil {
		t.Fatalf("patient room = %v, want nil", second.Room())
	}
	if second.State() != AsleepInHallway {
		t.Fatalf("patient state = %v, want %v", second.State(), AsleepInHallway)
	}
}

func TestAssignRoomRejectsAwakePatient(t *testing.T) {
	h := newHospitalWithRooms(t, 101)
	p := mustPatient(t, "Ana", 30, Mild)
	if err := h.AdmitPatient(p); err != nil {
		t.Fatalf("AdmitPatient: %v", err)
	}

	room, err := h.AssignRoom(p) // still Awake: no sleep attack happened
	if !errors.Is(err, ErrIsAwake) {
		t.Fatalf("err = %v, want it to wrap %v", err, ErrIsAwake)
	}
	if room != nil {
		t.Fatalf("room = %v, want nil for an awake patient", room)
	}
	if p.State() != Awake {
		t.Fatalf("state = %v, want %v (the guard must not change it)", p.State(), Awake)
	}
}

func TestPatientsInHallway(t *testing.T) {
	h := newHospitalWithRooms(t, 101)

	hallway := mustPatient(t, "Dora", 60, Mild)
	roomed := mustPatient(t, "Elías", 22, Mild)
	for _, p := range []*Patient{hallway, roomed} {
		if err := h.AdmitPatient(p); err != nil {
			t.Fatalf("AdmitPatient: %v", err)
		}
	}

	if err := hallway.SufferSleepAttack("ground floor"); err != nil {
		t.Fatalf("SufferSleepAttack(hallway): %v", err)
	}
	if err := roomed.SufferSleepAttack("lobby"); err != nil {
		t.Fatalf("SufferSleepAttack(roomed): %v", err)
	}
	if _, err := h.AssignRoom(roomed); err != nil {
		t.Fatalf("AssignRoom(roomed): %v", err)
	}

	got := h.PatientsInHallway()
	if len(got) != 1 {
		t.Fatalf("PatientsInHallway length = %d, want 1", len(got))
	}
	if got[0] != hallway {
		t.Fatalf("PatientsInHallway[0] = %v, want %v", got[0], hallway)
	}
}

func TestSevereReportCountsEpisodes(t *testing.T) {
	h := mustHospital(t, "test hospital")
	doc := mustDoctor(t, "Karen", 41, "Neurología")
	if err := h.HireDoctor(doc); err != nil {
		t.Fatalf("HireDoctor: %v", err)
	}

	severe := mustPatient(t, "Fabián", 55, Severe)
	mild := mustPatient(t, "Gloria", 33, Mild)
	for _, p := range []*Patient{severe, mild} {
		if err := h.AdmitPatient(p); err != nil {
			t.Fatalf("AdmitPatient: %v", err)
		}
	}

	if err := severe.SufferSleepAttack("cafetería"); err != nil {
		t.Fatalf("SufferSleepAttack: %v", err)
	}
	if _, err := h.Dispatch(severe, "cafetería"); err != nil { //Dispatch also records the episode
		t.Fatalf("Dispatch: %v", err)
	}

	report := h.SevereReport()
	if len(report) != 1 {
		t.Fatalf("SevereReport length = %d, want 1", len(report))
	}
	if _, ok := report[mild]; ok {
		t.Fatal("SevereReport must not include a Mild patient")
	}
	if got := report[severe]; got != 1 {
		t.Fatalf("report[severe] = %d, want 1", got)
	}
}

func TestDoctorDiagnosesUntilCapacity(t *testing.T) {
	doc := mustDoctor(t, "Karen", 41, "Neurología")
	if !doc.IsAvailable() {
		t.Fatal("a freshly hired doctor must be available")
	}

	first := mustPatient(t, "Ana", 30, Mild)
	if err := doc.DiagnosePatient(first); err != nil {
		t.Fatalf("DiagnosePatient(first): %v", err)
	}
	if first.Doctor() != doc {
		t.Fatalf("patient doctor = %v, want %v", first.Doctor(), doc)
	}
	if err := doc.DiagnosePatient(first); !errors.Is(err, ErrSameDoctor) {
		t.Fatalf("DiagnosePatient(first) again: err = %v, want it to wrap %v", err, ErrSameDoctor)
	}

	for i := 1; i < maxPatients; i++ {
		p := mustPatient(t, "Patient", 20, Mild)
		if err := doc.DiagnosePatient(p); err != nil {
			t.Fatalf("DiagnosePatient(%d): %v", i, err)
		}
	}
	if doc.IsAvailable() {
		t.Fatalf("doctor must be unavailable at %d patients under care", maxPatients)
	}
	if got := len(doc.PatientsUnderCare()); got != maxPatients {
		t.Fatalf("patients under care = %d, want %d", got, maxPatients)
	}

	overflow := mustPatient(t, "Beto", 40, Mild)
	if err := doc.DiagnosePatient(overflow); !errors.Is(err, ErrDocUnavailable) {
		t.Fatalf("DiagnosePatient(overflow): err = %v, want it to wrap %v", err, ErrDocUnavailable)
	}
	if overflow.Doctor() != nil {
		t.Fatalf("rejected patient must stay unattached, got doctor %v", overflow.Doctor())
	}
}

func TestRoomAcceptsMultipleUntilCapacity(t *testing.T) {
	room, err := NewRoom(201, 2)
	if err != nil {
		t.Fatalf("NewRoom: %v", err)
	}
	first := mustPatient(t, "Ana", 30, Mild)
	second := mustPatient(t, "Beto", 40, Mild)
	third := mustPatient(t, "Carla", 50, Mild)

	if !room.IsAvailable() || room.State() != Available {
		t.Fatalf("fresh room: available=%v state=%v, want true/%v", room.IsAvailable(), room.State(), Available)
	}
	if err := room.Occupy(first); err != nil {
		t.Fatalf("Occupy(first): %v", err)
	}
	if !room.IsAvailable() || room.State() != Available {
		t.Fatalf("room with one free slot must stay available (state=%v)", room.State())
	}
	if err := room.Occupy(second); err != nil {
		t.Fatalf("Occupy(second): %v", err)
	}
	if room.IsAvailable() || room.State() != Occupied {
		t.Fatalf("room at full capacity must be occupied (available=%v state=%v)", room.IsAvailable(), room.State())
	}
	if err := room.Occupy(first); !errors.Is(err, ErrAlreadyInRoom) {
		t.Fatalf("Occupy(first) again: err = %v, want it to wrap %v", err, ErrAlreadyInRoom)
	}
	if err := room.Occupy(third); !errors.Is(err, ErrRoomOccupied) {
		t.Fatalf("Occupy(third): err = %v, want it to wrap %v", err, ErrRoomOccupied)
	}
	if err := room.Release(first); err != nil {
		t.Fatalf("Release(first): %v", err)
	}
	if !room.IsAvailable() || room.State() != Available {
		t.Fatalf("after releasing one of two the room must be available (state=%v)", room.State())
	}
	if err := room.Occupy(third); err != nil {
		t.Fatalf("Occupy(third) after a free slot: %v", err)
	}
	if room.State() != Occupied {
		t.Fatalf("state = %v, want %v", room.State(), Occupied)
	}
}

func TestRoomKeepsPatientLink(t *testing.T) {
	room, err := NewRoom(201, 1)
	if err != nil {
		t.Fatalf("NewRoom: %v", err)
	}
	p := mustPatient(t, "Ana", 30, Mild)

	if err := room.Occupy(p); err != nil {
		t.Fatalf("Occupy: %v", err)
	}
	if p.Room() != room {
		t.Fatalf("patient room = %v, want %v (Occupy must set the back-reference)", p.Room(), room)
	}
	if err := room.Release(p); err != nil {
		t.Fatalf("Release: %v", err)
	}
	if p.Room() != nil {
		t.Fatalf("patient room = %v, want nil after Release", p.Room())
	}
}

func TestDispatchRotatesThroughStaff(t *testing.T) {
	h := mustHospital(t, "test hospital")
	doc := mustDoctor(t, "Karen", 41, "Neurología")
	orderly := mustOrderly(t, "Inés", 29)
	if err := h.HireDoctor(doc); err != nil {
		t.Fatalf("HireDoctor: %v", err)
	}
	if err := h.HireOrderly(orderly); err != nil {
		t.Fatalf("HireOrderly: %v", err)
	}

	got := make([]Attender, 0, 2)
	for i := 0; i < 2; i++ {
		p := mustPatient(t, "Patient", 20, Mild)
		if err := h.AdmitPatient(p); err != nil {
			t.Fatalf("AdmitPatient(%d): %v", i, err)
		}
		if err := p.SufferSleepAttack("hallway"); err != nil {
			t.Fatalf("SufferSleepAttack(%d): %v", i, err)
		}
		episode, err := h.Dispatch(p, "hallway")
		if err != nil {
			t.Fatalf("Dispatch(%d): %v", i, err)
		}
		got = append(got, episode.Attender())
	}

	if got[0] != doc || got[1] != orderly {
		t.Fatalf("dispatch order = [%v, %v], want [%v, %v] (round-robin must rotate)", got[0], got[1], doc, orderly)
	}
	if len(doc.MyEpisodes()) != 1 {
		t.Fatalf("doctor episodes = %d, want 1 (dispatch must record it on the attender)", len(doc.MyEpisodes()))
	}
	if len(h.History()) != 2 {
		t.Fatalf("history length = %d, want 2 (dispatch must record every episode)", len(h.History()))
	}
}

func TestDispatchAttendsWithDoctorAtCapacity(t *testing.T) {
	h := mustHospital(t, "test hospital")
	doc := mustDoctor(t, "Karen", 41, "Neurología")
	orderly := mustOrderly(t, "Inés", 29)
	if err := h.HireDoctor(doc); err != nil {
		t.Fatalf("HireDoctor: %v", err)
	}
	if err := h.HireOrderly(orderly); err != nil {
		t.Fatalf("HireOrderly: %v", err)
	}

	for i := 0; i < maxPatients; i++ {
		if err := doc.DiagnosePatient(mustPatient(t, "UnderCare", 20, Mild)); err != nil {
			t.Fatalf("DiagnosePatient(%d): %v", i, err)
		}
	}
	if doc.IsAvailable() {
		t.Fatalf("doctor must be unavailable at %d patients under care", maxPatients)
	}

	p := mustPatient(t, "Fabián", 55, Mild)
	if err := h.AdmitPatient(p); err != nil {
		t.Fatalf("AdmitPatient: %v", err)
	}
	if err := p.SufferSleepAttack("hallway"); err != nil {
		t.Fatalf("SufferSleepAttack: %v", err)
	}

	episode, err := h.Dispatch(p, "hallway")
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if episode.Attender() != doc {
		t.Fatalf("dispatch attender = %v, want the Doctor %v (an emergency is never gated by under-care load)", episode.Attender(), doc)
	}
	if len(doc.PatientsUnderCare()) != maxPatients {
		t.Fatalf("patients under care = %d, want %d (attending must not change the under-care list)", len(doc.PatientsUnderCare()), maxPatients)
	}
}

func TestAttendHallwayPatients(t *testing.T) {
	h := mustHospital(t, "test hospital")
	orderly := mustOrderly(t, "Inés", 29)
	if err := h.HireOrderly(orderly); err != nil {
		t.Fatalf("HireOrderly: %v", err)
	}

	stuck := mustPatient(t, "Gloria", 33, Mild)
	for _, p := range []*Patient{stuck} {
		if err := h.AdmitPatient(p); err != nil {
			t.Fatalf("AdmitPatient: %v", err)
		}
	}
	if err := stuck.SufferSleepAttack("ground floor"); err != nil {
		t.Fatalf("SufferSleepAttack: %v", err)
	}

	if errs := h.AttendHallwayPatients(); len(errs) != 0 {
		t.Fatalf("AttendHallwayPatients returned errors: %v", errs)
	}
	if len(h.History()) != 1 {
		t.Fatalf("history length = %d, want 1", len(h.History()))
	}
	if stuck.State() != AsleepInHallway {
		t.Fatalf("attending must not change the state, got %v", stuck.State())
	}
}

// TestHospitalConcurrentAccess hammers the hospital from many goroutines at once:
// writers admit + put patients to sleep + compete for rooms + dispatch, while
// readers run the queries. Run it with `go test -race` to prove the mutex closes
// the race; the assertions check the invariants that must hold for any interleaving.
func TestHospitalConcurrentAccess(t *testing.T) {
	h := mustHospital(t, "test hospital")
	if err := h.HireDoctor(mustDoctor(t, "Karen", 41, "Neurología")); err != nil {
		t.Fatalf("HireDoctor: %v", err)
	}
	if err := h.HireOrderly(mustOrderly(t, "Inés", 29)); err != nil {
		t.Fatalf("HireOrderly: %v", err)
	}
	for _, number := range []int{101, 102, 103, 104, 105} {
		room, err := NewRoom(number, 2)
		if err != nil {
			t.Fatalf("NewRoom(%d): %v", number, err)
		}
		if err := h.RegisterRoom(room); err != nil {
			t.Fatalf("RegisterRoom(%d): %v", number, err)
		}
	}

	const patients = 40
	ps := make([]*Patient, patients)
	for i := range ps {
		ps[i] = mustPatient(t, fmt.Sprintf("Patient %d", i), 20, Mild)
	}

	var wg sync.WaitGroup
	for i, p := range ps {
		wg.Add(1)
		go func(i int, p *Patient) {
			defer wg.Done()
			if err := h.AdmitPatient(p); err != nil {
				t.Errorf("AdmitPatient(%d): %v", i, err)
				return
			}
			if err := h.SleepAttack(p, "hallway"); err != nil {
				t.Errorf("SleepAttack(%d): %v", i, err)
				return
			}
			if _, err := h.AssignRoom(p); err != nil && !errors.Is(err, ErrNoFreeRoom) {
				t.Errorf("AssignRoom(%d): %v", i, err)
			}
			if _, err := h.Dispatch(p, p.Location()); err != nil {
				t.Errorf("Dispatch(%d): %v", i, err)
			}
		}(i, p)
	}

	// Reader goroutines run the queries concurrently with the writers.
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = h.PatientsInHallway()
			_ = h.SevereReport()
			_ = h.History()
			_ = h.Staff()
			_ = h.Patients()
			_ = h.Rooms()
		}()
	}
	wg.Wait()

	if got := len(h.History()); got != patients {
		t.Fatalf("history length = %d, want %d (every dispatch must be recorded exactly once)", got, patients)
	}

	seen := make(map[*Patient]int)
	occupants := 0
	for _, r := range h.Rooms() {
		if len(r.Occupants()) > r.Capacity() {
			t.Fatalf("room %d holds %d occupants, capacity %d", r.ID(), len(r.Occupants()), r.Capacity())
		}
		for _, p := range r.Occupants() {
			seen[p]++
			occupants++
		}
	}
	for p, n := range seen {
		if n != 1 {
			t.Fatalf("patient %s appears in %d rooms, want exactly 1", p.ID(), n)
		}
	}
	inRoom := 0
	for _, p := range h.Patients() {
		if p.Room() != nil {
			inRoom++
		}
	}
	if inRoom != occupants {
		t.Fatalf("patients claiming a room = %d but rooms hold %d (back-references out of sync)", inRoom, occupants)
	}
}
