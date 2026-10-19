package hospital

import (
	"fmt"
	"slices"
	"strings"
)

const maxPatients = 4 //every Doctor has max 4 patients -> reach limit -> not Available
type Doctor struct {
	Person
	specialty         string
	patientsUnderCare []*Patient
	episodesAttended  []EpisodeRecord //value records: each is a snapshot, not a shared pointer
}

func NewDoctor(name string, age int, specialty string) (*Doctor, error) {
	if strings.TrimSpace(specialty) == "" {
		return nil, fmt.Errorf("new doctor: specialty is required: %w", ErrInvalidInput)
	}
	person, err := NewPerson(generateID(TypeDoctor), name, age)
	if err != nil {
		return nil, fmt.Errorf("new doctor: %w", err)
	}
	return &Doctor{
		Person:    person,
		specialty: specialty,
	}, nil
}

func (d *Doctor) Specialty() string             { return d.specialty }
func (d *Doctor) PatientsUnderCare() []*Patient { return slices.Clone(d.patientsUnderCare) }

func (d *Doctor) MyEpisodes() []EpisodeRecord {
	return slices.Clone(d.episodesAttended)
}

// / methods to implement the Attender interface
// IsAvailable reports whether the doctor can take on another patient under care.
// It is a Doctor-only concern (not part of the Attender interface): it gates
// DiagnosePatient only, never the dispatch of an emergency.
func (d *Doctor) IsAvailable() bool { return len(d.PatientsUnderCare()) < maxPatients }

func (d *Doctor) AttendSleepEmergency(p *Patient) error {
	if p == nil {
		return fmt.Errorf("doctor %s attend sleep emergency: %w", d.ID(), ErrPatientNil)
	}
	if p.State() == Awake {
		return fmt.Errorf("doctor %s attend sleep emergency %s: %w", d.ID(), p.ID(), ErrIsAwake) //only a sleeping patient is a sleep emergency
	}
	return nil //no error -> the emergency can be attended
}

func (d *Doctor) Attend(p *Patient, location string) (EpisodeRecord, error) {
	if err := d.AttendSleepEmergency(p); err != nil {
		return EpisodeRecord{}, fmt.Errorf("doctor %s attend: %w", d.ID(), err)
	}
	episode := NewEpisode(p, d, location)
	d.episodesAttended = append(d.episodesAttended, episode) // attends the emergency (even if unavailable)
	return episode, nil                                      //value only, no pointer
}

func (d *Doctor) DiagnosePatient(p *Patient) error {
	if p == nil {
		return fmt.Errorf("doctor %s diagnose patient: %w", d.ID(), ErrPatientNil)
	}
	if !d.IsAvailable() {
		return fmt.Errorf("doctor %s diagnose patient %s: %w", d.ID(), p.ID(), ErrDocUnavailable) //becoming in charge of a new patient -> availability is required
	}
	if p.Doctor() != nil {
		if p.Doctor() == d {
			return fmt.Errorf("doctor %s diagnose patient %s: %w", d.ID(), p.ID(), ErrSameDoctor) //already assigned to this same doctor
		}
		return fmt.Errorf("doctor %s diagnose patient %s: %w", d.ID(), p.ID(), ErrAssignedToOther)
	}
	d.patientsUnderCare = append(d.patientsUnderCare, p)
	p.assignedDoctor = d
	return nil
}
