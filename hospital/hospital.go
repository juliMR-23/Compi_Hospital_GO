package hospital

type Hospital struct {
	name     string
	doctors  []*Doctor
	patients []*Patient
	rooms    []*Room
	history  []*EpisodeRecord
}
