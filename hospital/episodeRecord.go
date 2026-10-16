package hospital

import "time"

type EpisodeRecord struct {
	id       string
	dateTime time.Time
	patient  *Patient
	attender Attender //Originally only doctor -> interface, any attender
	location string
	room     *Room
}
