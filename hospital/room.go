package hospital

import (
	"fmt"
	"slices"
)

type Room struct {
	number    int
	capacity  int
	state     RoomState
	occupants []*Patient
}

type RoomState int

const (
	Available RoomState = iota
	Occupied
)

func (r RoomState) String() string {
	switch r {
	case Available:
		return "Available"
	case Occupied:
		return "Occupied"
	default:
		return "Unknown"
	}
} // for readable printing
func NewRoom(number int, capacity int) *Room {
	return &Room{
		number:   number,
		capacity: capacity,
		//RoomState: Available by default
	}
}

func (r *Room) ID() int               { return r.number }
func (r *Room) Capacity() int         { return r.capacity }
func (r *Room) State() RoomState      { return r.state }
func (r *Room) Occupants() []*Patient { return slices.Clone(r.occupants) }

func (r *Room) IsAvailable() bool {
	return r.state == Available
}

func (r *Room) Occupy(p *Patient) error {
	if p == nil {
		return fmt.Errorf("room %d occupy: %w", r.number, ErrPatientNil)
	}
	if r.state != Available {
		return fmt.Errorf("room %d occupy patient %s: %w", r.number, p.ID(), ErrRoomOccupied)
	}
	r.state = Occupied
	r.occupants = append(r.occupants, p)
	return nil
}

func (r *Room) Release(p *Patient) error {
	if p == nil {
		return fmt.Errorf("room %d release: %w", r.number, ErrPatientNil)
	}
	for i, occ := range r.occupants {
		if occ == p {
			r.occupants = append(r.occupants[:i], r.occupants[i+1:]...)
			if len(r.occupants) == 0 {
				r.state = Available
			}
			return nil
		}
	}
	return fmt.Errorf("room %d release patient %s: %w", r.number, p.ID(), ErrNotInRoom)
}
