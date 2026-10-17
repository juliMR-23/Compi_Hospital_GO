package hospital

import "errors"

var (
	ErrDocUnavailable = errors.New("doctor is not available yet")
	ErrPatientNil     = errors.New("patient is nil here")
	ErrIsAwake        = errors.New("patient is already awake")
	// TODO: more errors depending on the issue
)
