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
func NewRoom(number int, capacity int) (*Room, error) {
	if capacity <= 0 {
		return nil, fmt.Errorf("new room %d: %w", number, ErrInvalidRoomCapacity)
	}
	return &Room{
		number:   number,
		capacity: capacity,
		//RoomState: Available by default (no occupants yet)
	}, nil
}

func (r *Room) ID() int               { return r.number }
func (r *Room) Capacity() int         { return r.capacity }
func (r *Room) State() RoomState      { return r.state }
func (r *Room) Occupants() []*Patient { return slices.Clone(r.occupants) }

func (r *Room) IsAvailable() bool {
	return len(r.occupants) < r.capacity //any free slot keeps the room available
}

// refreshState marks the room Occupied only when it is full; any free slot keeps it Available.
func (r *Room) refreshState() {
	if len(r.occupants) >= r.capacity {
		r.state = Occupied
	} else {
		r.state = Available
	}
}

func (r *Room) Occupy(p *Patient) error {
	if p == nil {
		return fmt.Errorf("room %d occupy: %w", r.number, ErrPatientNil)
	}
	if slices.Contains(r.occupants, p) {
		return fmt.Errorf("room %d occupy patient %s: %w", r.number, p.ID(), ErrAlreadyInRoom) //the same patient must not take two slots
	}
	if len(r.occupants) >= r.capacity {
		return fmt.Errorf("room %d occupy patient %s: %w", r.number, p.ID(), ErrRoomOccupied) //full -> refuse, do not overflow capacity
	}
	r.occupants = append(r.occupants, p)
	r.refreshState()
	return nil
}

func (r *Room) Release(p *Patient) error {
	if p == nil {
		return fmt.Errorf("room %d release: %w", r.number, ErrPatientNil)
	}
	for i, occ := range r.occupants {
		if occ == p {
			r.occupants = append(r.occupants[:i], r.occupants[i+1:]...)
			r.refreshState() //a freed slot always leaves the room with at least one free space
			return nil
		}
	}
	return fmt.Errorf("room %d release patient %s: %w", r.number, p.ID(), ErrNotInRoom)
}
