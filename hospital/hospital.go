package hospital

import "slices"

type Hospital struct {
	name     string
	doctors  []*Doctor
	patients []*Patient
	rooms    []*Room
	history  []*EpisodeRecord
	staff    []Attender //polymorphism: holds doctors and orderlies dispatched together
}

func NewHospital(name string) *Hospital {
	return &Hospital{
		name: name,
	}
}

func (h *Hospital) Name() string              { return h.name }
func (h *Hospital) Doctors() []*Doctor        { return slices.Clone(h.doctors) }
func (h *Hospital) Staff() []Attender         { return slices.Clone(h.staff) }
func (h *Hospital) Patients() []*Patient      { return slices.Clone(h.patients) }
func (h *Hospital) Rooms() []*Room            { return slices.Clone(h.rooms) }
func (h *Hospital) History() []*EpisodeRecord { return slices.Clone(h.history) }

func (h *Hospital) AdmitPatient(p *Patient) error {
	if p == nil {
		return ErrPatientNil
	}
	if slices.Contains(h.patients, p) {
		return ErrPatientAlreadyAdmitted //avoid duplicating the patient in the list
	}
	h.patients = append(h.patients, p)
	return nil
}

func (h *Hospital) HireDoctor(d *Doctor) error {
	if d == nil {
		return ErrDoctorNil
	}
	if slices.Contains(h.doctors, d) {
		return ErrDoctorAlreadyHired //avoid duplicating the doctor in the staff
	}
	h.doctors = append(h.doctors, d)
	h.staff = append(h.staff, d) //a doctor is also an Attender
	return nil
}

func (h *Hospital) HireOrderly(o *Orderly) error {
	if o == nil {
		return ErrOrderlyNil
	}
	if slices.Contains(h.staff, Attender(o)) {
		return ErrOrderlyAlreadyHired //avoid duplicating the orderly in the staff
	}
	h.staff = append(h.staff, o) //an orderly is an Attender too
	return nil
}

func (h *Hospital) AvailableAttender() Attender {
	for _, a := range h.staff {
		if a.IsAvailable() { //no concrete type known here -> pure polymorphism
			return a
		}
	}
	return nil
}

func (h *Hospital) Dispatch(p *Patient, location string) (EpisodeRecord, error) {
	if p == nil {
		return EpisodeRecord{}, ErrPatientNil
	}
	attender := h.AvailableAttender()
	if attender == nil {
		return EpisodeRecord{}, ErrNoAttenderAvailable //nobody free -> no invented attender
	}
	return attender.Attend(p, location) //dispatched without knowing if Doctor or Orderly
}

func (h *Hospital) AssignRoom(p *Patient) (*Room, error) {
	if h.rooms == nil {
		return nil, ErrNoFreeRoom
	}
	if p == nil {
		return nil, ErrPatientNil
	}
	if p.Room() != nil {
		return nil, ErrHasRoom
	}
	for _, r := range h.rooms {
		if r.IsAvailable() {
			if err := r.Occupy(p); err != nil {
				return nil, err
			}
			p.room = r
			p.state = AsleepInBed
			return r, nil
		}
	}
	return nil, ErrNoFreeRoom
}

func (h *Hospital) RegisterEpisode(e *EpisodeRecord) error {
	if e == nil {
		return ErrEpisodeNil
	}
	if slices.Contains(h.history, e) {
		return ErrEpisodeAlreadyRegistered //avoid duplicating the episode in the history
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
