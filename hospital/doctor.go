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
func (d *Doctor) MyEpisodes() []*EpisodeRecord  { return slices.Clone(d.episodesAttended) }

// / methods to implement the Attender interface
func (d *Doctor) IsAvailable() bool { return len(d.PatientsUnderCare()) < maxPatients }
func (d *Doctor) Attend(p *Patient, location string) (EpisodeRecord, error) {
	if p == nil {
		return EpisodeRecord{}, ErrPatientNil
	}
	if p.State() == Awake {
		return EpisodeRecord{}, ErrIsAwake //Attend only Asleep patients -> Awake: not an emergency
	}
	episode := NewEpisode(p, d, location)
	d.episodesAttended = append(d.episodesAttended, episode)
	return *episode, nil //* now takes away the pointer -> return value only
}
