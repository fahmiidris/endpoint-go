package endpoint

import "errors"

// Errors returned by Insert.
var (
	// ERROR_DUPLICATE_ROUTE is returned when the same route is inserted twice.
	ERROR_DUPLICATE_ROUTE = errors.New("endpoint: duplicate route")
	// ERROR_PARAM_CONFLICT is returned when two routes capture the same path
	// position with different parameter names.
	ERROR_PARAM_CONFLICT = errors.New("endpoint: conflicting parameter name")
	// ERROR_CATCH_ALL_NOT_LAST is returned when a catch-all parameter is followed
	// by more path segments.
	ERROR_CATCH_ALL_NOT_LAST = errors.New("endpoint: catch-all is not the last segment")
	// ERROR_EMPTY_PARAM_NAME is returned when a parameter segment has no name,
	// such as ":" or "*".
	ERROR_EMPTY_PARAM_NAME = errors.New("endpoint: empty parameter name")
	// ERROR_INVALID_ROUTE is returned when a route is malformed, for example
	// when it does not start with "/".
	ERROR_INVALID_ROUTE = errors.New("endpoint: invalid route")
)

// ERROR_NOT_FOUND is returned by At when no route matches the path.
var ERROR_NOT_FOUND = errors.New("endpoint: no matching route")
