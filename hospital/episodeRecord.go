package hospital

import (
	"fmt"
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

// NewEpisode returns an EpisodeRecord by value (no pointer): the record is a
// screenshot of what happened at this instant, so it is copied where it is
// stored and never aliases the caller's variable.
func NewEpisode(patient *Patient, attender Attender, location string) EpisodeRecord {
	return EpisodeRecord{
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
func (e EpisodeRecord) Summary() string {
	roomName := "hallway"
	if e.room != nil {
		roomName = fmt.Sprintf("room %d", e.room.ID())
	}
	return fmt.Sprintf("%s %s -> %s", e.patient.Name(), e.location, roomName)
}
