package hospital

import "errors"

var (
	ErrDuplicateID = errors.New("this id is already used")
	// TODO: more errors depending on the issue
)
