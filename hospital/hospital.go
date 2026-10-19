package hospital

import (
	"fmt"
	"slices"
	"strings"
	"sync"
)

// Hospital owns every piece of shared mutable state in the model (staff, patients,
// rooms and the episode history). Because the optional concurrent simulation runs
// one goroutine per patient, every exported method takes mu before touching that
// state. Helpers whose name ends in "Locked" assume the lock is already held and
// never lock again, because sync.Mutex is not reentrant.
type Hospital struct {
	mu           sync.Mutex
	name         string
	doctors      []*Doctor
	patients     []*Patient
	rooms        []*Room
	history      []EpisodeRecord
	staff        []Attender //polymorphism: holds doctors and orderlies dispatched together
	nextAttender int        //round-robin cursor: every emergency is attended, so the cursor rotates fairly across all staff
}

func NewHospital(name string) (*Hospital, error) {
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("new hospital: name is required: %w", ErrInvalidInput)
	}
	return &Hospital{
		name: name,
	}, nil
}

// Name reads a field that NewHospital sets once and never mutates, so it needs no lock.
func (h *Hospital) Name() string { return h.name }

func (h *Hospital) Doctors() []*Doctor {
	h.mu.Lock()
	defer h.mu.Unlock()
	return slices.Clone(h.doctors)
}

func (h *Hospital) Staff() []Attender {
	h.mu.Lock()
	defer h.mu.Unlock()
	return slices.Clone(h.staff)
}

func (h *Hospital) Patients() []*Patient {
	h.mu.Lock()
	defer h.mu.Unlock()
	return slices.Clone(h.patients)
}

func (h *Hospital) Rooms() []*Room {
	h.mu.Lock()
	defer h.mu.Unlock()
	return slices.Clone(h.rooms)
}

func (h *Hospital) History() []EpisodeRecord {
	h.mu.Lock()
	defer h.mu.Unlock()
	return slices.Clone(h.history)
}

func (h *Hospital) AdmitPatient(p *Patient) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if p == nil {
		return fmt.Errorf("admit patient: %w", ErrPatientNil)
	}
	if slices.Contains(h.patients, p) {
		return fmt.Errorf("admit patient %s: %w", p.ID(), ErrPatientAlreadyAdmitted) //avoid duplicating the patient in the list
	}
	h.patients = append(h.patients, p)
	return nil
}

func (h *Hospital) HireDoctor(d *Doctor) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if d == nil {
		return fmt.Errorf("hire doctor: %w", ErrDoctorNil)
	}
	if slices.Contains(h.doctors, d) {
		return fmt.Errorf("hire doctor %s: %w", d.ID(), ErrDoctorAlreadyHired) //avoid duplicating the doctor in the staff
	}
	h.doctors = append(h.doctors, d)
	h.staff = append(h.staff, d) //a doctor is also an Attender
	return nil
}

func (h *Hospital) HireOrderly(o *Orderly) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if o == nil {
		return fmt.Errorf("hire orderly: %w", ErrOrderlyNil)
	}
	if slices.Contains(h.staff, Attender(o)) {
		return fmt.Errorf("hire orderly %s: %w", o.ID(), ErrOrderlyAlreadyHired) //avoid duplicating the orderly in the staff
	}
	h.staff = append(h.staff, o) //an orderly is an Attender too
	return nil
}

func (h *Hospital) RegisterRoom(r *Room) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if r == nil {
		return fmt.Errorf("register room: %w", ErrRoomNil)
	}
	if slices.ContainsFunc(h.rooms, func(existing *Room) bool { return existing.ID() == r.ID() }) {
		return fmt.Errorf("register room %d: %w", r.ID(), ErrRoomAlreadyRegistered) //avoid registering the same room number twice, even with a different pointer
	}
	h.rooms = append(h.rooms, r)
	return nil
}

// NextAttender returns the next attender in round-robin order. Emergencies are
// always attended, so there is no availability gate here: any doctor or orderly
// is eligible, and the cursor keeps the rotation fair. It returns nil only when
// the hospital has hired nobody.
func (h *Hospital) NextAttender() Attender {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.nextAttenderLocked()
}

// nextAttenderLocked is the lock-held core of NextAttender; it also advances the cursor.
func (h *Hospital) nextAttenderLocked() Attender {
	n := len(h.staff)
	if n == 0 {
		return nil
	}
	attender := h.staff[h.nextAttender]
	h.nextAttender = (h.nextAttender + 1) % n
	return attender
}

func (h *Hospital) Dispatch(p *Patient, location string) (EpisodeRecord, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.dispatchLocked(p, location)
}

// dispatchLocked is the lock-held core of Dispatch, reused by AttendHallwayPatients.
func (h *Hospital) dispatchLocked(p *Patient, location string) (EpisodeRecord, error) {
	if p == nil {
		return EpisodeRecord{}, fmt.Errorf("dispatch patient: %w", ErrPatientNil)
	}
	attender := h.nextAttenderLocked()
	if attender == nil {
		return EpisodeRecord{}, fmt.Errorf("dispatch patient %s: %w", p.ID(), ErrNoAttenderAvailable) //nobody hired -> no invented attender
	}
	episode, err := attender.Attend(p, location) //dispatched without knowing if Doctor or Orderly
	if err != nil {
		return EpisodeRecord{}, fmt.Errorf("dispatch patient %s: %w", p.ID(), err)
	}
	if err := h.registerEpisodeLocked(episode); err != nil { //the hospital keeps the history it coordinated, not the caller
		return EpisodeRecord{}, fmt.Errorf("dispatch patient %s: %w", p.ID(), err)
	}
	return episode, nil
}

// AttendHallwayPatients dispatches an attender to every patient currently stuck
// in a hallway. The hospital decides WHEN to act (the state AsleepInHallway) and
// WHO goes (the round-robin cursor over []Attender, which keeps alternating
// between doctors and orderlies). It never panics: failures are returned so the
// caller can report them. The whole pass runs under a single lock, so the
// hallway snapshot and the dispatches it triggers cannot interleave with another
// goroutine.
func (h *Hospital) AttendHallwayPatients() []error {
	h.mu.Lock()
	defer h.mu.Unlock()
	var errs []error
	for _, p := range h.patientsInHallwayLocked() {
		if _, err := h.dispatchLocked(p, p.Location()); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

func (h *Hospital) AssignRoom(p *Patient) (*Room, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if p == nil {
		return nil, fmt.Errorf("assign room: %w", ErrPatientNil)
	}
	if h.rooms == nil {
		return nil, fmt.Errorf("assign room %s: %w", p.ID(), ErrNoFreeRoom)
	}
	if p.Room() != nil {
		return nil, fmt.Errorf("assign room %s: %w", p.ID(), ErrHasRoom)
	}
	if p.State() == Awake {
		return nil, fmt.Errorf("assign room %s: %w", p.ID(), ErrIsAwake) //only a sleeping patient gets a room
	}
	for _, r := range h.rooms {
		if r.IsAvailable() {
			if err := r.Occupy(p); err != nil {
				return nil, fmt.Errorf("assign room %s: %w", p.ID(), err)
			}
			p.state = AsleepInBed //the room already set the patient's back-reference
			return r, nil
		}
	}
	return nil, fmt.Errorf("assign room %s: %w", p.ID(), ErrNoFreeRoom)
}

func (h *Hospital) RegisterEpisode(e EpisodeRecord) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.registerEpisodeLocked(e)
}

// registerEpisodeLocked is the lock-held core of RegisterEpisode, reused by Dispatch.
func (h *Hospital) registerEpisodeLocked(e EpisodeRecord) error {
	if e.ID() == "" {
		return fmt.Errorf("register episode: %w", ErrInvalidInput) //a zero-value record has no id
	}
	if slices.ContainsFunc(h.history, func(existing EpisodeRecord) bool { return existing.ID() == e.ID() }) {
		return fmt.Errorf("register episode %s: %w", e.ID(), ErrEpisodeAlreadyRegistered) //avoid duplicating the episode in the history
	}
	h.history = append(h.history, e)
	return nil
}

// SleepAttack makes a patient fall asleep through the hospital, so the mutation of
// the patient's state happens under the same lock as every other state change.
func (h *Hospital) SleepAttack(p *Patient, location string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if p == nil {
		return fmt.Errorf("sleep attack: %w", ErrPatientNil)
	}
	return p.SufferSleepAttack(location)
}

// WakeUp wakes a patient through the hospital, releasing their room under the lock.
func (h *Hospital) WakeUp(p *Patient) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if p == nil {
		return fmt.Errorf("wake up: %w", ErrPatientNil)
	}
	return p.WakeUp()
}

func (h *Hospital) PatientsInHallway() []*Patient {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.patientsInHallwayLocked()
}

// patientsInHallwayLocked is the lock-held core of PatientsInHallway.
func (h *Hospital) patientsInHallwayLocked() []*Patient {
	var hallway []*Patient
	for _, p := range h.patients {
		if p.state == AsleepInHallway {
			hallway = append(hallway, p)
		}
	}
	return hallway
}

func (h *Hospital) SevereReport() map[*Patient]int {
	h.mu.Lock()
	defer h.mu.Unlock()
	report := make(map[*Patient]int)
	for _, p := range h.patients {
		if p.Level() == Severe {
			count := 0
			for _, e := range h.history {
				if e.patient == p {
					count++
				}
			}
			report[p] = count
		}
	}
	return report
}
