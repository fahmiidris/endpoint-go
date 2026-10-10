// Package endpoint implements a small URL path router backed by a radix tree
// (a compressed prefix tree).
//
// Routes are registered with [Router.Insert] and matched with [Router.At]:
//
//	r := endpoint.New[string]()
//	if err := r.Insert("/users/:id", "profile"); err != nil {
//		// handle error
//	}
//	m, err := r.At("/users/42")
//	id, _ := m.Params.Get("id") // id == "42", m.Value == "profile"
//
// # Route syntax
//
// A route starts with "/" and is made of segments. Each segment is either
// static ("/users/list"), a named parameter (":id"), or a catch-all ("*path").
// A parameter matches exactly one non-empty segment. A catch-all matches the
// rest of the path, which may be empty, and must be the last segment. A ":"
// or "*" only starts a parameter at a segment boundary; anywhere else it is an
// ordinary character, so "/a:b" is a static route.
//
// # Matching
//
// At a given tree position a static segment wins over a parameter, and a
// parameter wins over a catch-all. If a static branch fails deeper in the
// path, matching backtracks to a parameter or catch-all branch that can still
// match. For example, with "/users/new", "/users/:id", and "/users/*rest"
// registered:
//
//	/users/new → "/users/new"
//	/users/42  → "/users/:id" with id = "42"
//	/users/a/b → "/users/*rest" with rest = "a/b"
//
// Trailing slashes are significant: "/users" and "/users/" are different
// paths and are never redirected. Parameter values are substrings of the path
// passed to At, so capturing never copies.
//
// # Concurrency
//
// At is safe to call from multiple goroutines. Insert is not safe for
// concurrent use and must not run concurrently with At: register all routes
// before serving lookups.
//
// # Allocations
//
// At does not allocate for ordinary lookups. Captured parameters are written
// into a fixed-size array in the caller's stack frame, covering routes with up
// to eight parameters; routes with more fall back to an exactly sized heap
// slice. If the returned Match and its Params escape the caller, Go promotes
// the backing array to the heap, which is required for soundness; lookups that
// do not retain the Match allocate nothing. Misses return [ERROR_NOT_FOUND]
// unwrapped and allocate nothing either.
package endpoint

import "fmt"

// maxInlineParams is the number of captured parameters that fit in the
// fixed-size array carried through a lookup. Routes with more parameters still
// work; their lookups fall back to an exactly sized heap slice.
const maxInlineParams = 8

// Router maps route patterns to values of type T.
//
// A Router is safe for concurrent use by multiple goroutines as long as no
// goroutine calls Insert concurrently with another operation. The usual
// pattern is to register all routes, then serve lookups.
type Router[T any] struct {
	root node[T]
}

// New returns an empty Router ready for [Router.Insert].
func New[T any]() *Router[T] {
	return &Router[T]{}
}

// Insert registers value for route.
//
// It returns ERROR_INVALID_ROUTE, ERROR_DUPLICATE_ROUTE, ERROR_PARAM_CONFLICT,
// ERROR_EMPTY_PARAM_NAME, or ERROR_CATCH_ALL_NOT_LAST for malformed, duplicated, or
// conflicting routes. Every returned error includes the route and wraps the
// relevant sentinel, so errors.Is works.
func (r *Router[T]) Insert(path string, value T) error {
	if path == "" || path[0] != '/' {
		return fmt.Errorf("endpoint: insert route %q: %w", path, ERROR_INVALID_ROUTE)
	}

	if err := r.root.insert(path, value); err != nil {
		return fmt.Errorf("endpoint: insert route %q: %w", path, err)
	}

	return nil
}

// Match is the result of a successful [Router.At] lookup.
type Match[T any] struct {
	// Value is the value registered for the matched route.
	Value T

	// Params holds the parameters captured by the match, in the order they
	// appear in the route. Params is nil when the route captures nothing.
	Params Params
}

// At returns the value registered for the route that matches path, together
// with the parameters it captured. If no route matches, At returns the zero
// Match and ERROR_NOT_FOUND.
func (r *Router[T]) At(path string) (Match[T], error) {
	var buf [maxInlineParams]Param

	return r.at(path, &buf)
}

// at performs the lookup. buf stores the captured parameters for routes with
// at most len(buf) captures; routes with more are matched a second time into
// an exactly sized slice.
//
// at is deliberately not inlinable: that keeps At's body small enough to
// stay within the compiler's inlining budget, which is what lets the inline
// parameter array live in the caller's stack frame instead of the heap.
//
//go:noinline
func (r *Router[T]) at(path string, buf *[maxInlineParams]Param) (Match[T], error) {
	matched, n, ok := r.root.search(path, buf[:], 0)

	if !ok {
		return Match[T]{}, ERROR_NOT_FOUND
	}

	if n <= maxInlineParams {
		var params Params

		if n > 0 {
			params = buf[:n]
		}

		return Match[T]{Value: matched.value, Params: params}, nil
	}
	// More captures than the inline array can hold: match again into an
	// exactly sized slice. The tree is immutable, so the second pass is
	// guaranteed to find the same route.
	params := make([]Param, n)

	r.root.search(path, params, 0)

	return Match[T]{Value: matched.value, Params: params}, nil
}
