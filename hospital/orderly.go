package hospital

import "slices"

type Orderly struct {
	Person                            //is a person too, saves id, name and age
	episodesAttended []*EpisodeRecord //as in doctor, this attender saves the history
}

func NewOrderly(name string, age int) *Orderly {
	return &Orderly{
		Person: NewPerson(generateID(TypeOrderly), name, age),
	}
}

func (o *Orderly) EpisodesAttended() []*EpisodeRecord { return slices.Clone(o.episodesAttended) }

// /methods from Attender interface
func (o *Orderly) IsAvailable() bool { return true } //always available -> around the hospital transferring patients
func (o *Orderly) Attend(p *Patient, location string) (EpisodeRecord, error) {
	if p == nil {
		return EpisodeRecord{}, ErrPatientNil
	}
	if p.State() == Awake {
		return EpisodeRecord{}, ErrIsAwake //Attend only Asleep patients -> Awake: not an emergency
	}
	episode := NewEpisode(p, o, location)
	o.episodesAttended = append(o.episodesAttended, episode)
	return *episode, nil //return value only
}
