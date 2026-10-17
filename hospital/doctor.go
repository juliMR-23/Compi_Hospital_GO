package hospital

import "slices"

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

func (d *Doctor) Specialty() string                  { return d.specialty }
func (d *Doctor) PatientsUnderCare() []*Patient      { return slices.Clone(d.patientsUnderCare) }
func (d *Doctor) EpisodesAttended() []*EpisodeRecord { return slices.Clone(d.episodesAttended) }

/// Todo: implement the Attender interface
