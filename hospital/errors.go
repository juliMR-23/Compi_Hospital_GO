package hospital

import "errors"

var (
	ErrDocUnavailable           = errors.New("doctor is not available yet")
	ErrPatientNil               = errors.New("patient is nil here")
	ErrDoctorNil                = errors.New("doctor is nil here")
	ErrIsAwake                  = errors.New("patient is already awake")
	ErrIsAsleep                 = errors.New("patient is already asleep")
	ErrNoFreeRoom               = errors.New("no free rooms at this time")
	ErrRoomNil                  = errors.New("room is nil here")
	ErrRoomOccupied             = errors.New("room is occupied")
	ErrInvalidInput             = errors.New("invalid input")
	ErrRoomAlreadyRegistered    = errors.New("room is already registered in this hospital")
	ErrNotInRoom                = errors.New("patient is not in this room")
	ErrAlreadyInRoom            = errors.New("patient is already in this room")
	ErrHasRoom                  = errors.New("patient already has a room")
	ErrSameDoctor               = errors.New("patient is already under this doctor's care")
	ErrAssignedToOther          = errors.New("patient is already assigned to another doctor")
	ErrPatientAlreadyAdmitted   = errors.New("patient is already admitted in this hospital")
	ErrDoctorAlreadyHired       = errors.New("doctor is already hired in this hospital")
	ErrOrderlyNil               = errors.New("orderly is nil here")
	ErrOrderlyAlreadyHired      = errors.New("orderly is already hired in this hospital")
	ErrNoAttenderAvailable      = errors.New("no attender is available at this time")
	ErrEpisodeAlreadyRegistered = errors.New("episode is already registered in the history")
)
