package hospital

import "slices"

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
