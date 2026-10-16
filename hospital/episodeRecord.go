package hospital

import "time"

type EpisodeRecord struct {
	id       string
	dateTime time.Time
	patient  Patient
	attender Attender //Originalmente solo doctor -> interfaz, cualquier attender
	location string
	room     Room
}
