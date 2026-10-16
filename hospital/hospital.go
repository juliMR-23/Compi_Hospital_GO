package hospital

type Hospital struct {
	Name     string
	Doctors  []Doctor
	Patients []Patient
	Rooms    []Room
	History  []EpisodeRecord
}
