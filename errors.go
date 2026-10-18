package endpoint

import "errors"

// Errors returned by Insert.
var (
	// ErrDuplicateRoute is returned when the same route is inserted twice.
	ErrDuplicateRoute = errors.New("endpoint: duplicate route")
	// ErrParamConflict is returned when two routes capture the same path
	// position with different parameter names.
	ErrParamConflict = errors.New("endpoint: conflicting parameter name")
	// ErrCatchAllNotLast is returned when a catch-all parameter is followed
	// by more path segments.
	ErrCatchAllNotLast = errors.New("endpoint: catch-all is not the last segment")
	// ErrEmptyParamName is returned when a parameter segment has no name,
	// such as ":" or "*".
	ErrEmptyParamName = errors.New("endpoint: empty parameter name")
	// ErrInvalidRoute is returned when a route is malformed, for example
	// when it does not start with "/".
	ErrInvalidRoute = errors.New("endpoint: invalid route")
)

// ErrNotFound is returned by At when no route matches the path.
var ErrNotFound = errors.New("endpoint: no matching route")
