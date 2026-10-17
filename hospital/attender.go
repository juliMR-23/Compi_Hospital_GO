package hospital

type Attender interface {
	ID() string
	Name() string
	IsAvailable() bool
	Attend(p *Patient, location string) (EpisodeRecord, error)
}
