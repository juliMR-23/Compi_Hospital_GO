package hospital

import "errors"

var (
	ErrDocUnavailable = errors.New("doctor is not available yet")
	ErrPatientNil     = errors.New("patient is nil here")
	ErrIsAwake        = errors.New("patient is already awake")
	ErrIsAsleep       = errors.New("patient is already asleep")
	ErrNoFreeRoom     = errors.New("no free rooms at this time")
	ErrRoomOccupied   = errors.New("room is occupied")
	ErrNotInRoom      = errors.New("patient is not in this room")
	// TODO: more errors depending on the issue
)
