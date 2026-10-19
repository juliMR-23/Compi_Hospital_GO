package hospital

import (
	"fmt"
	"slices"
)

type Orderly struct {
	Person                           //is a person too, saves id, name and age
	episodesAttended []EpisodeRecord //as in doctor, this attender keeps value snapshots
}

func NewOrderly(name string, age int) (*Orderly, error) {
	person, err := NewPerson(generateID(TypeOrderly), name, age)
	if err != nil {
		return nil, fmt.Errorf("new orderly: %w", err)
	}
	return &Orderly{
		Person: person,
	}, nil
}

func (o *Orderly) EpisodesAttended() []EpisodeRecord { return slices.Clone(o.episodesAttended) }

// /methods from Attender interface
func (o *Orderly) Attend(p *Patient, location string) (EpisodeRecord, error) {
	if p == nil {
		return EpisodeRecord{}, fmt.Errorf("orderly %s attend: %w", o.ID(), ErrPatientNil)
	}
	if p.State() == Awake {
		return EpisodeRecord{}, fmt.Errorf("orderly %s attend patient %s: %w", o.ID(), p.ID(), ErrIsAwake) //Attend only Asleep patients -> Awake: not an emergency
	}
	episode := NewEpisode(p, o, location)
	o.episodesAttended = append(o.episodesAttended, episode)
	return episode, nil //value only, no pointer
}
