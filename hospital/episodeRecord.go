package hospital

import (
	"time"
)

type EpisodeRecord struct {
	id       string
	dateTime time.Time
	patient  *Patient
	attender Attender //Originally only doctor -> interface, any attender
	location string
	room     *Room
}

func NewEpisode(patient *Patient, attender Attender, location string) *EpisodeRecord {
	return &EpisodeRecord{
		id:       generateID(TypeEpisode),
		dateTime: time.Now(),
		patient:  patient,
		attender: attender,
		location: location,
		room:     patient.Room(), //could be nil -> episode occurred on the hallway...
	}
}

func (e EpisodeRecord) ID() string         { return e.id }
func (e EpisodeRecord) Time() time.Time    { return e.dateTime }
func (e EpisodeRecord) Patient() *Patient  { return e.patient }
func (e EpisodeRecord) Attender() Attender { return e.attender }
func (e EpisodeRecord) Location() string   { return e.location }
func (e EpisodeRecord) Room() *Room        { return e.room }
