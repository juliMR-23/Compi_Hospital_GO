package hospital

import "slices"

const maxPatients = 4 //every Doctor has max 4 patients -> reach limit -> not Available
type Doctor struct {
	Person
	specialty         string
	patientsUnderCare []*Patient
	episodesAttended  []*EpisodeRecord
}

func NewDoctor(name string, age int, specialty string) *Doctor {
	return &Doctor{
		Person:    NewPerson(generateID(TypeDoctor), name, age),
		specialty: specialty,
	}
}

func (d *Doctor) Specialty() string             { return d.specialty }
func (d *Doctor) PatientsUnderCare() []*Patient { return slices.Clone(d.patientsUnderCare) }

func (d *Doctor) MyEpisodes() []EpisodeRecord {
	episodes := make([]EpisodeRecord, 0, len(d.episodesAttended))
	for _, e := range d.episodesAttended {
		episodes = append(episodes, *e)
	}
	return episodes
}

// / methods to implement the Attender interface
func (d *Doctor) IsAvailable() bool { return len(d.PatientsUnderCare()) < maxPatients }

func (d *Doctor) AttendSleepEmergency(p *Patient) error {
	if p == nil {
		return ErrPatientNil
	}
	if p.State() == Awake {
		return ErrIsAwake //only a sleeping patient is a sleep emergency
	}
	return nil //no error -> the emergency can be attended
}

func (d *Doctor) Attend(p *Patient, location string) (EpisodeRecord, error) {
	if err := d.AttendSleepEmergency(p); err != nil {
		return EpisodeRecord{}, err
	}
	episode := NewEpisode(p, d, location)
	d.episodesAttended = append(d.episodesAttended, episode) // attends the emergency (even if unavailable)
	return *episode, nil                                     //* now takes away the pointer -> return value only
}

func (d *Doctor) DiagnosePatient(p *Patient) error {
	if p == nil {
		return ErrPatientNil
	}
	if !d.IsAvailable() {
		return ErrDocUnavailable //becoming in charge of a new patient -> availability is required
	}
	if p.Doctor() != nil {
		if p.Doctor() == d {
			return ErrSameDoctor //already assigned to this same doctor
		}
		return ErrAssignedToOther
	}
	d.patientsUnderCare = append(d.patientsUnderCare, p)
	p.assignedDoctor = d
	return nil
}
