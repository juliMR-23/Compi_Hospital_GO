package hospital

import "slices"

type Orderly struct {
	Person                            //is a person too, saves id, name and age
	episodesAttended []*EpisodeRecord //as in doctor, this attender saves the history
}

func NewOrderly(id string, name string, age int) *Orderly {
	return &Orderly{
		Person: NewPerson(id, name, age),
	}
}

func (o *Orderly) EpisodesAttended() []*EpisodeRecord { return slices.Clone(o.episodesAttended) }

///Todo: implement the Attender interface
