package hospital

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

func Summary() string {
	return ""
}
