package hospital

import (
	"errors"
	"testing"
)

func newHospitalWithRooms(t *testing.T, numbers ...int) *Hospital {
	t.Helper()
	h := NewHospital("test hospital")
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
	p := NewPatient("Ana", 30, Mild)
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

	first := NewPatient("Beto", 40, Mild)
	if err := h.AdmitPatient(first); err != nil {
		t.Fatalf("AdmitPatient(first): %v", err)
	}
	if err := first.SufferSleepAttack("hallway"); err != nil {
		t.Fatalf("SufferSleepAttack(first): %v", err)
	}
	if _, err := h.AssignRoom(first); err != nil {
		t.Fatalf("AssignRoom(first): %v", err)
	}

	second := NewPatient("Carla", 50, Severe)
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

func TestPatientsInHallway(t *testing.T) {
	h := newHospitalWithRooms(t, 101)

	hallway := NewPatient("Dora", 60, Mild)
	roomed := NewPatient("Elías", 22, Mild)
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
	h := NewHospital("test hospital")
	doc := NewDoctor("Karen", 41, "Neurología")
	if err := h.HireDoctor(doc); err != nil {
		t.Fatalf("HireDoctor: %v", err)
	}

	severe := NewPatient("Fabián", 55, Severe)
	mild := NewPatient("Gloria", 33, Mild)
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

func TestAttenderPolymorphism(t *testing.T) {
	staff := []Attender{
		NewDoctor("Hugo", 45, "Neumología"),
		NewOrderly("Inés", 29),
	}
	if len(staff) != 2 {
		t.Fatalf("staff length = %d, want 2", len(staff))
	}

	for _, a := range staff {
		p := NewPatient("Patient", 20, Mild)
		if err := p.SufferSleepAttack("hallway"); err != nil {
			t.Fatalf("SufferSleepAttack: %v", err)
		}
		episode, err := a.Attend(p, "hallway")
		if err != nil {
			t.Fatalf("Attend(%s): %v", a.Name(), err)
		}
		if episode.Attender() != a {
			t.Fatalf("episode attender = %v, want %v", episode.Attender(), a)
		}
	}
}

func TestNewRoomRejectsInvalidCapacity(t *testing.T) {
	for _, capacity := range []int{0, -1} {
		room, err := NewRoom(101, capacity)
		if !errors.Is(err, ErrInvalidRoomCapacity) {
			t.Fatalf("NewRoom(101, %d): err = %v, want it to wrap %v", capacity, err, ErrInvalidRoomCapacity)
		}
		if room != nil {
			t.Fatalf("NewRoom(101, %d): room = %v, want nil", capacity, room)
		}
	}
}

func TestRoomAcceptsMultipleUntilCapacity(t *testing.T) {
	room, err := NewRoom(201, 2)
	if err != nil {
		t.Fatalf("NewRoom: %v", err)
	}
	first := NewPatient("Ana", 30, Mild)
	second := NewPatient("Beto", 40, Mild)
	third := NewPatient("Carla", 50, Mild)

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

func TestRoomRejectsDuplicatePatient(t *testing.T) {
	room, err := NewRoom(202, 2)
	if err != nil {
		t.Fatalf("NewRoom: %v", err)
	}
	p := NewPatient("Dora", 60, Mild)
	if err := room.Occupy(p); err != nil {
		t.Fatalf("Occupy(p): %v", err)
	}
	if err := room.Occupy(p); !errors.Is(err, ErrAlreadyInRoom) {
		t.Fatalf("Occupy(p) again: err = %v, want it to wrap %v", err, ErrAlreadyInRoom)
	}
	if len(room.Occupants()) != 1 {
		t.Fatalf("occupants = %d, want 1 (a duplicate must not take a slot)", len(room.Occupants()))
	}
}

func TestAssignRoomFillsCapacityBeforeNextRoom(t *testing.T) {
	h := NewHospital("test hospital")
	for _, number := range []int{101, 102} {
		room, err := NewRoom(number, 2)
		if err != nil {
			t.Fatalf("NewRoom(%d): %v", number, err)
		}
		if err := h.RegisterRoom(room); err != nil {
			t.Fatalf("RegisterRoom(%d): %v", number, err)
		}
	}

	want := []int{101, 101, 102}
	for i, roomNumber := range want {
		p := NewPatient("Patient", 20, Mild)
		if err := h.AdmitPatient(p); err != nil {
			t.Fatalf("AdmitPatient(%d): %v", i, err)
		}
		if err := p.SufferSleepAttack("hallway"); err != nil {
			t.Fatalf("SufferSleepAttack(%d): %v", i, err)
		}
		room, err := h.AssignRoom(p)
		if err != nil {
			t.Fatalf("AssignRoom(%d): %v", i, err)
		}
		if room.ID() != roomNumber {
			t.Fatalf("AssignRoom(%d) = room %d, want room %d", i, room.ID(), roomNumber)
		}
	}
}

func TestDispatchUsesOrderlyWhenNoDoctor(t *testing.T) {
	h := NewHospital("test hospital")
	orderly := NewOrderly("Inés", 29)
	if err := h.HireOrderly(orderly); err != nil {
		t.Fatalf("HireOrderly: %v", err)
	}

	p := NewPatient("Fabián", 55, Mild)
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
	if episode.Attender() != orderly {
		t.Fatalf("dispatch attender = %v, want the Orderly %v", episode.Attender(), orderly)
	}
	if len(h.History()) != 1 {
		t.Fatalf("history length = %d, want 1 (Dispatch must record the episode)", len(h.History()))
	}
}

func TestDispatchRotatesThroughStaff(t *testing.T) {
	h := NewHospital("test hospital")
	doc := NewDoctor("Karen", 41, "Neurología")
	orderly := NewOrderly("Inés", 29)
	if err := h.HireDoctor(doc); err != nil {
		t.Fatalf("HireDoctor: %v", err)
	}
	if err := h.HireOrderly(orderly); err != nil {
		t.Fatalf("HireOrderly: %v", err)
	}

	got := make([]Attender, 0, 2)
	for i := 0; i < 2; i++ {
		p := NewPatient("Patient", 20, Mild)
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
}

func TestAttendHallwayPatients(t *testing.T) {
	h := NewHospital("test hospital")
	orderly := NewOrderly("Inés", 29)
	if err := h.HireOrderly(orderly); err != nil {
		t.Fatalf("HireOrderly: %v", err)
	}

	stuck := NewPatient("Gloria", 33, Mild)
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
