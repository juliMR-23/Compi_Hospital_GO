package hospital

import "slices"

type Hospital struct {
	name     string
	doctors  []*Doctor
	patients []*Patient
	rooms    []*Room
	history  []*EpisodeRecord
}

func NewHospital(name string) *Hospital {
	return &Hospital{
		name: name,
	}
}

func (h *Hospital) Name() string              { return h.name }
func (h *Hospital) Doctors() []*Doctor        { return slices.Clone(h.doctors) }
func (h *Hospital) Patients() []*Patient      { return slices.Clone(h.patients) }
func (h *Hospital) Rooms() []*Room            { return slices.Clone(h.rooms) }
func (h *Hospital) History() []*EpisodeRecord { return slices.Clone(h.history) }
