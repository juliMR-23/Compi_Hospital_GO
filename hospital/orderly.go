package hospital

type Orderly struct {
	Person                            //is a person too, saves id, name and age
	episodesAttended []*EpisodeRecord //as in doctor, this attender saves the history
}
