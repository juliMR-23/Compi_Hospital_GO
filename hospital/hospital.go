package hospital

import "slices"

type Hospital struct {
	name     string
	doctors  []*Doctor
	patients []*Patient
	rooms    []*Room
	history  []*EpisodeRecord
}

func NewHospital(name string) *Hospital {
	return &Hospital{
		name: name,
	}
}

func (h *Hospital) Name() string              { return h.name }
func (h *Hospital) Doctors() []*Doctor        { return slices.Clone(h.doctors) }
func (h *Hospital) Patients() []*Patient      { return slices.Clone(h.patients) }
func (h *Hospital) Rooms() []*Room            { return slices.Clone(h.rooms) }
func (h *Hospital) History() []*EpisodeRecord { return slices.Clone(h.history) }

func (h *Hospital) AdmitPatient(p *Patient) error {
	h.patients = append(h.patients, p)
	return nil
}

func (h *Hospital) HireDoctor(d *Doctor) error {
	h.doctors = append(h.doctors, d)
	return nil
}

func (h *Hospital) AssignRoom(p *Patient) (*Room, error) {
	if h.rooms == nil {
		return nil, ErrNoFreeRoom
	}
	for _, r := range h.rooms {
		if r.IsAvailable() {
			r.state = Occupied
			p.room = r
			p.state = AsleepInBed
			return r, nil
		}
	}
	return nil, ErrNoFreeRoom
}

func (h *Hospital) RegisterEpisode(e *EpisodeRecord) error {
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
