package hospital

type Doctor struct {
	Person
	specialty         string
	patientsUnderCare []*Patient
	episodesAttended  []*EpisodeRecord
}
