# endpoint-go

A small URL path router for Go, backed by a radix tree (compressed prefix
tree). It maps route patterns to values of a generic type `T`; a lookup returns
the value plus any captured parameters. No dependencies, no `unsafe`, no
`reflect`.

## Installation

Requires Go 1.26.3 or newer.

```
go get github.com/fahmiidris/endpoint-go
```

## Usage

```go
package main

import (
	"fmt"

	endpoint "github.com/fahmiidris/endpoint-go"
)

func main() {
	r := endpoint.New[string]()

	if err := r.Insert("/users/:id", "profile"); err != nil {
		panic(err)
	}

	if err := r.Insert("/files/*path", "static files"); err != nil {
		panic(err)
	}

	m, err := r.At("/users/42")
	
	if err != nil {
		panic(err)
	}

	id, _ := m.Params.Get("id")

	fmt.Println(m.Value, id) // profile 42

	m, err = r.At("/files/notes/todo.txt")

	if err != nil {
		panic(err)
	}

	path, _ := m.Params.Get("path")

		fmt.Println(m.Value, path) // static files notes/todo.txt
	}
	```

	A zero-value `Router[T]` is an empty router, so `var r endpoint.Router[string]`
	works as well; `New` only spells that out.

## Route syntax

| Pattern   | Example        | Matches                                             |
| --------- | -------------- | --------------------------------------------------- |
| static    | `/users/list`  | exactly that path                                   |
| parameter | `/users/:id`   | exactly one non-empty segment                       |
| catch-all | `/files/*path` | the rest of the path (possibly empty); must be last |

`:` and `*` only start a parameter at a segment boundary (at the start of the
route or right after `/`). Anywhere else they are ordinary characters, so
`/a:b` is a static route. Parameter names are non-empty runs of characters
that are not `/`, `:`, or `*`.

## Matching rules

1. Static segments beat parameters, and parameters beat catch-alls.
2. If a static branch fails deeper in the path, matching backtracks into a
   parameter or catch-all branch that can still match.
3. Parameters never match an empty segment: `/users/:id` does not match
   `/users/`.
4. Trailing slashes are significant: `/a` and `/a/` are different paths, and
   there are no redirects.
5. A catch-all captures the remainder of the path without the leading `/`:
   `/files/*path` matches `/files/a/b` with `path == "a/b"`, and `/files/`
   with `path == ""`.
6. The same position in the tree cannot capture under two different names;
   inserting such a route returns `ErrParamConflict`.
7. Captured parameters are returned in the order they appear in the route.
   `Params.Get` reports whether a name was captured and returns its value; if
   a route uses the same name at more than one position, as in
   `/users/:id/posts/:id`, Get returns the first capture. A route that captures
   nothing yields a nil `Params`.

## Internals

Every route is registered in a single tree. Here is the tree for
`/users`, `/users/new`, `/users/:id`, `/users/:id/posts`, and `/files/*path`:

```
root
└── "/"                                  static, no value
    ├── "users"  (value)                 → /users
    │   └── "/"                          static, no value
    │       ├── "new"  (value)           → /users/new
    │       └── :id                      param "id"
    │           ├── (value)              → /users/:id
    │           └── "/posts"  (value)    → /users/:id/posts
    └── "files/"                         static, no value
        └── *path  (catch-all, value)    → /files/*path
```

Each node stores only the path fragment its subtree shares (a _compressed_
prefix) plus:

- `indices` — the first byte of every static child's prefix, so the next byte
  of the path selects a child with a tiny linear scan;
- `children`, `param`, `catchAll` — the node's children;
- `value`/`hasValue` — the route that ends exactly here. `hasValue` is
  explicit because `T` may be a zero value.

Insertion walks down the tree, splitting nodes where prefixes diverge:

```
insert "/apple", then "/app":

    "/apple"        →       "/app"  (value)
                              └── "le"
```

Inserting `/app` splits the existing `/apple` node into the shared prefix
`/app` (which now holds the value) and the remainder `le`, so lookups can
reuse the longest possible prefix match.

A lookup consumes the path byte by byte, trying children in priority order
(static, param, catch-all). If a higher-priority branch fails deeper in the
path, the search backtracks to the next branch. No path text is copied or
converted to `[]byte`; parameter values are substrings that share memory with
the input path, so a retained `Match` keeps the whole path string alive.

## Allocations

`At` does not allocate for ordinary lookups in an ordinary build —
`TestAtDoesNotAllocate` asserts this with `testing.AllocsPerRun`, and the
benchmarks report `0 B/op, 0 allocs/op` (Apple M3 Pro):

```
BenchmarkAt_Static-12    30830100    37.27 ns/op    0 B/op    0 allocs/op
BenchmarkAt_Params-12    23494783    50.69 ns/op    0 B/op    0 allocs/op
```

Captured parameters are written into a fixed-size array declared in `At`.
Because `At` stays within the compiler's inlining budget, the array lives in
the caller's stack frame. Caveats:

- The array only stays on the stack while `At` is inlined into its caller,
  which is the default. Coverage instrumentation (`go test -cover`) and builds
  with inlining disabled (`-gcflags=all=-l`) push `At` over the inline budget,
  and lookups then allocate one 256-byte buffer per call.
  `TestAtDoesNotAllocate` skips itself when coverage is enabled.
- If the returned `Match` outlives the call, Go promotes the array to the
  heap. That is required for soundness and only happens when the caller
  retains the result.
- Routes capturing more than eight parameters fall back to an exactly sized
  heap slice (they still work, they just allocate).

Misses return `ErrNotFound` unwrapped, so they do not allocate either.

## Errors

`Insert` returns one of these sentinels, wrapped with the route, so
`errors.Is` works:

| Error                | Cause                                                                        |
| -------------------- | ---------------------------------------------------------------------------- |
| `ErrInvalidRoute`    | route is empty or does not start with `/`, or a parameter token is malformed |
| `ErrDuplicateRoute`  | the same route is already registered                                         |
| `ErrParamConflict`   | two routes capture the same position with different names                    |
| `ErrEmptyParamName`  | a `:` or `*` segment has no name                                             |
| `ErrCatchAllNotLast` | a `*name` segment is followed by more segments                               |

`At` returns `ErrNotFound` when nothing matches.

`Insert` is not transactional: it validates the route as it descends the tree,
so a rejected route can leave empty, value-less nodes behind. Those nodes never
change the result of a lookup.

## Concurrency

`At` is safe for concurrent use by multiple goroutines. `Insert` is not safe
for concurrent use and must not run concurrently with `At`: register all
routes before serving lookups.

## Out of scope

Route removal, case-insensitive matching, path cleaning or escaping, redirect
hints, HTTP method handling, and `net/http` integration.

## Testing

```
go test ./...
go test -race ./...
go test -bench . -benchmem
go test -fuzz=FuzzAtMatchesReference -fuzztime 30s
```

`TestAtMatchesReference` and `FuzzAtMatchesReference` register random route sets
and compare `At` against a brute-force reference matcher over generated paths,
which is stronger than fixed cases for precedence and backtracking. The
randomized test uses a fixed seed, so a failure repeats.
