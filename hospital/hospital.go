package hospital

import (
	"fmt"
	"slices"
	"strings"
)

type Hospital struct {
	name         string
	doctors      []*Doctor
	patients     []*Patient
	rooms        []*Room
	history      []*EpisodeRecord
	staff        []Attender //polymorphism: holds doctors and orderlies dispatched together
	nextAttender int        //round-robin cursor: rotates dispatch so one always-available attender cannot starve the rest
}

func NewHospital(name string) (*Hospital, error) {
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("new hospital: name is required: %w", ErrInvalidInput)
	}
	return &Hospital{
		name: name,
	}, nil
}

func (h *Hospital) Name() string              { return h.name }
func (h *Hospital) Doctors() []*Doctor        { return slices.Clone(h.doctors) }
func (h *Hospital) Staff() []Attender         { return slices.Clone(h.staff) }
func (h *Hospital) Patients() []*Patient      { return slices.Clone(h.patients) }
func (h *Hospital) Rooms() []*Room            { return slices.Clone(h.rooms) }
func (h *Hospital) History() []*EpisodeRecord { return slices.Clone(h.history) }

func (h *Hospital) AdmitPatient(p *Patient) error {
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
	if r == nil {
		return fmt.Errorf("register room: %w", ErrRoomNil)
	}
	if slices.ContainsFunc(h.rooms, func(existing *Room) bool { return existing.ID() == r.ID() }) {
		return fmt.Errorf("register room %d: %w", r.ID(), ErrRoomAlreadyRegistered) //avoid registering the same room number twice, even with a different pointer
	}
	h.rooms = append(h.rooms, r)
	return nil
}

func (h *Hospital) AvailableAttender() Attender {
	n := len(h.staff)
	for i := 0; i < n; i++ {
		idx := (h.nextAttender + i) % n //walk the list from the cursor, wrapping around
		if h.staff[idx].IsAvailable() { //no concrete type known here -> pure polymorphism
			h.nextAttender = (idx + 1) % n //advance past the attender we just used
			return h.staff[idx]
		}
	}
	return nil
}

func (h *Hospital) Dispatch(p *Patient, location string) (EpisodeRecord, error) {
	if p == nil {
		return EpisodeRecord{}, fmt.Errorf("dispatch patient: %w", ErrPatientNil)
	}
	attender := h.AvailableAttender()
	if attender == nil {
		return EpisodeRecord{}, fmt.Errorf("dispatch patient %s: %w", p.ID(), ErrNoAttenderAvailable) //nobody free -> no invented attender
	}
	episode, err := attender.Attend(p, location) //dispatched without knowing if Doctor or Orderly
	if err != nil {
		return EpisodeRecord{}, fmt.Errorf("dispatch patient %s: %w", p.ID(), err)
	}
	if err := h.RegisterEpisode(&episode); err != nil { //the hospital keeps the history it coordinated, not the caller
		return EpisodeRecord{}, fmt.Errorf("dispatch patient %s: %w", p.ID(), err)
	}
	return episode, nil
}

// AttendHallwayPatients dispatches an available attender to every patient
// currently stuck in a hallway. The hospital decides WHEN to act (the state
// AsleepInHallway) and WHO goes (the round-robin cursor over []Attender, which
// keeps alternating between doctors and orderlies). It never panics: failures
// are returned so the caller can report them.
func (h *Hospital) AttendHallwayPatients() []error {
	var errs []error
	for _, p := range h.PatientsInHallway() {
		if _, err := h.Dispatch(p, p.Location()); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

func (h *Hospital) AssignRoom(p *Patient) (*Room, error) {
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
		return nil, fmt.Errorf("assign room %s: %w", p.ID(), ErrIsAwake) //only a sleeping patient needs a bed
	}
	for _, r := range h.rooms {
		if r.IsAvailable() {
			if err := r.Occupy(p); err != nil {
				return nil, fmt.Errorf("assign room %s: %w", p.ID(), err)
			}
			p.state = AsleepInBed //the room already set the back-reference on Occupy
			return r, nil
		}
	}
	return nil, fmt.Errorf("assign room %s: %w", p.ID(), ErrNoFreeRoom)
}

func (h *Hospital) RegisterEpisode(e *EpisodeRecord) error {
	if e == nil {
		return fmt.Errorf("register episode: %w", ErrEpisodeNil)
	}
	if slices.ContainsFunc(h.history, func(ep *EpisodeRecord) bool { return ep.ID() == e.ID() }) {
		return fmt.Errorf("register episode %s: %w", e.ID(), ErrEpisodeAlreadyRegistered) //avoid duplicating the episode in the history
	}
	h.history = append(h.history, e)
	return nil
}

func (h *Hospital) PatientsInHallway() []*Patient {
	var hallway []*Patient
	for _, p := range h.patients {
		if p.state == AsleepInHallway {
			hallway = append(hallway, p)
		}
	}
	return hallway
}

func (h *Hospital) SevereReport() map[*Patient]int {
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
