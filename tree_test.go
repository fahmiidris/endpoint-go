package endpoint_test

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"
	"testing"

	endpoint "github.com/fahmiidris/endpoint-go"
)

// mustInsert registers route or fails the test.
func mustInsert[T any](t *testing.T, r *endpoint.Router[T], path string, value T) {
	t.Helper()

	if err := r.Insert(path, value); err != nil {
		t.Fatalf("Insert(%q) error: %v", path, err)
	}
}

func TestAtStatic(t *testing.T) {
	r := endpoint.New[int]()

	for path, value := range map[string]int{
		"/":                     0,
		"/users":                1,
		"/users/new":            2,
		"/users/list":           3,
		"/users/list/active":    4,
		"/app":                  5,
		"/apple":                6,
		"/application":          7,
		"/application/settings": 8,
	} {
		mustInsert(t, r, path, value)
	}

	tests := []struct {
		path  string
		value int
		found bool
	}{
		{"/", 0, true},
		{"/users", 1, true},
		{"/users/new", 2, true},
		{"/users/list", 3, true},
		{"/users/list/active", 4, true},
		{"/app", 5, true},
		{"/apple", 6, true},
		{"/application", 7, true},
		{"/application/settings", 8, true},
		{"/user", 0, false},
		{"/userz", 0, false},
		{"/users/", 0, false},
		{"/users/new/", 0, false},
		{"/apples", 0, false},
		{"/appli", 0, false},
		{"/application/setting", 0, false},
		{"/application/settings/", 0, false},
	}

	for _, tt := range tests {
		t.Run(strings.ReplaceAll(tt.path, "/", "_"), func(t *testing.T) {
			m, err := r.At(tt.path)

			if !tt.found {
				if !errors.Is(err, endpoint.ErrNotFound) {
					t.Fatalf("At(%q) error = %v, want ErrNotFound", tt.path, err)
				}

				return
			}

			if err != nil {
				t.Fatalf("At(%q) error = %v", tt.path, err)
			}

			if m.Value != tt.value {
				t.Errorf("At(%q) value = %d, want %d", tt.path, m.Value, tt.value)
			}

			if len(m.Params) != 0 {
				t.Errorf("At(%q) params = %v, want none", tt.path, m.Params)
			}
		})
	}
}

// TestInsertSplitOrders inserts routes that force node splitting in every
// order to make sure splitting preserves values.
func TestInsertSplitOrders(t *testing.T) {
	paths := []string{"/app", "/apple", "/application"}
	perms := [][3]int{{0, 1, 2}, {0, 2, 1}, {1, 0, 2}, {1, 2, 0}, {2, 0, 1}, {2, 1, 0}}

	for _, perm := range perms {
		t.Run(fmt.Sprint(perm), func(t *testing.T) {
			r := endpoint.New[int]()

			for _, idx := range perm {
				mustInsert(t, r, paths[idx], idx)
			}

			for idx, path := range paths {
				m, err := r.At(path)

				if err != nil {
					t.Fatalf("At(%q) error = %v", path, err)
				}

				if m.Value != idx {
					t.Errorf("At(%q) value = %d, want %d", path, m.Value, idx)
				}
			}

			if _, err := r.At("/apples"); !errors.Is(err, endpoint.ErrNotFound) {
				t.Errorf(`At("/apples") error = %v, want ErrNotFound`, err)
			}
		})
	}
}

func TestInsertDuplicateStatic(t *testing.T) {
	r := endpoint.New[int]()

	mustInsert(t, r, "/app", 1)

	err := r.Insert("/app", 2)

	if !errors.Is(err, endpoint.ErrDuplicateRoute) {
		t.Errorf("Insert duplicate error = %v, want ErrDuplicateRoute", err)
	}

	if !strings.Contains(err.Error(), "/app") {
		t.Errorf("duplicate error = %q, want it to include the route", err)
	}
}

func TestAtParams(t *testing.T) {
	r := endpoint.New[string]()

	for path, value := range map[string]string{
		"/users/new":            "static",
		"/users/:id":            "user",
		"/users/:id/posts":      "posts",
		"/users/:id/posts/:pid": "post",
		"/:a/:b":                "ab",
	} {
		mustInsert(t, r, path, value)
	}

	tests := []struct {
		path   string
		value  string
		params map[string]string
		found  bool
	}{
		// A static segment wins over a parameter.
		{"/users/new", "static", nil, true},
		{"/users/42", "user", map[string]string{"id": "42"}, true},
		{"/users/42/posts", "posts", map[string]string{"id": "42"}, true},
		{"/users/42/posts/7", "post", map[string]string{"id": "42", "pid": "7"}, true},
		{"/x/y", "ab", map[string]string{"a": "x", "b": "y"}, true},
		// Parameters must be non-empty; trailing slashes are significant.
		{"/users/", "", nil, false},
		{"/users", "", nil, false},
		{"/users/42/", "", nil, false},
		{"/users//posts", "", nil, false},
		{"/users/42/post", "", nil, false},
		// Not enough segments.
		{"/x", "", nil, false},
		{"/x/y/z", "", nil, false},
	}

	for _, tt := range tests {
		t.Run(strings.ReplaceAll(tt.path, "/", "_"), func(t *testing.T) {
			m, err := r.At(tt.path)

			if !tt.found {
				if !errors.Is(err, endpoint.ErrNotFound) {
					t.Fatalf("At(%q) error = %v, want ErrNotFound", tt.path, err)
				}

				return
			}

			if err != nil {
				t.Fatalf("At(%q) error = %v", tt.path, err)
			}

			if m.Value != tt.value {
				t.Errorf("At(%q) value = %q, want %q", tt.path, m.Value, tt.value)
			}

			if len(m.Params) != len(tt.params) {
				t.Errorf("At(%q) params = %v, want %v", tt.path, m.Params, tt.params)
			}

			for key, want := range tt.params {
				got, ok := m.Params.Get(key)

				if !ok || got != want {
					t.Errorf("At(%q) Params.Get(%q) = %q, %v; want %q, true", tt.path, key, got, ok, want)
				}
			}
		})
	}
}

// TestAtDuplicateParamNames checks that a route may capture the same name at
// different positions, in which case Get returns the first capture.
func TestAtDuplicateParamNames(t *testing.T) {
	r := endpoint.New[int]()

	mustInsert(t, r, "/users/:id/posts/:id", 1)

	m, err := r.At("/users/7/posts/9")

	if err != nil {
		t.Fatalf("At error = %v", err)
	}

	if len(m.Params) != 2 {
		t.Fatalf("len(params) = %d, want 2", len(m.Params))
	}

	if got, ok := m.Params.Get("id"); !ok || got != "7" {
		t.Errorf("Get(%q) = %q, %v; want %q, true", "id", got, ok, "7")
	}
}

func TestInsertParamErrors(t *testing.T) {
	tests := []struct {
		name  string
		setup []string
		path  string
		want  error
	}{
		{"empty name", nil, "/users/:", endpoint.ErrEmptyParamName},
		{"empty name at root", nil, "/:", endpoint.ErrEmptyParamName},
		{"misplaced token", nil, "/users/:id:x", endpoint.ErrInvalidRoute},
		{"duplicate", []string{"/users/:id"}, "/users/:id", endpoint.ErrDuplicateRoute},
		{"conflict", []string{"/users/:id"}, "/users/:name", endpoint.ErrParamConflict},
		{"conflict before suffix", []string{"/users/:id"}, "/users/:name/posts", endpoint.ErrParamConflict},
		{"conflict beneath shared prefix", []string{"/a/:x/b/:z"}, "/a/:x/b/:y", endpoint.ErrParamConflict},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := endpoint.New[int]()

			for _, path := range tt.setup {
				mustInsert(t, r, path, 1)
			}

			err := r.Insert(tt.path, 2)

			if err == nil {
				t.Fatalf("Insert(%q) succeeded, want %v", tt.path, tt.want)
			}

			if !errors.Is(err, tt.want) {
				t.Errorf("Insert(%q) error = %v, want %v", tt.path, err, tt.want)
			}

			if !strings.Contains(err.Error(), tt.path) {
				t.Errorf("Insert(%q) error = %q, want it to include the route", tt.path, err)
			}
		})
	}
}

func TestInsertParamsWithoutConflict(t *testing.T) {
	t.Run("different branches", func(t *testing.T) {
		r := endpoint.New[int]()

		// Different static branches may use different parameter names at the
		// same depth.
		mustInsert(t, r, "/users/:id/posts/:post", 1)
		mustInsert(t, r, "/users/:id/comments/:comment", 2)
	})

	t.Run("same name at different positions", func(t *testing.T) {
		r := endpoint.New[int]()

		mustInsert(t, r, "/users/:id/posts/:id", 1)
	})
}

func TestAtCatchAll(t *testing.T) {
	r := endpoint.New[string]()

	mustInsert(t, r, "/files/*path", "files")
	mustInsert(t, r, "/files/list", "list")
	mustInsert(t, r, "/files/:id", "one")
	mustInsert(t, r, "/*rest", "root")
	mustInsert(t, r, "/", "slash")

	tests := []struct {
		path   string
		value  string
		params map[string]string
		found  bool
	}{
		{"/files/a/b.txt", "files", map[string]string{"path": "a/b.txt"}, true},
		{"/files/", "files", map[string]string{"path": ""}, true},
		// Static and parameters win where they match.
		{"/files/list", "list", nil, true},
		{"/files/42", "one", map[string]string{"id": "42"}, true},
		// Backtracking into the catch-all when the more specific branches
		// fail deeper.
		{"/files/42/deep", "files", map[string]string{"path": "42/deep"}, true},
		{"/files/list/deep", "files", map[string]string{"path": "list/deep"}, true},
		// The root catch-all only matches when no static branch can.
		{"/", "slash", nil, true},
		{"/files", "root", map[string]string{"rest": "files"}, true},
		{"/other", "root", map[string]string{"rest": "other"}, true},
		{"/a", "root", map[string]string{"rest": "a"}, true},
	}

	for _, tt := range tests {
		t.Run(strings.ReplaceAll(tt.path, "/", "_"), func(t *testing.T) {
			m, err := r.At(tt.path)

			if !tt.found {
				if !errors.Is(err, endpoint.ErrNotFound) {
					t.Fatalf("At(%q) error = %v, want ErrNotFound", tt.path, err)
				}

				return
			}

			if err != nil {
				t.Fatalf("At(%q) error = %v", tt.path, err)
			}

			if m.Value != tt.value {
				t.Errorf("At(%q) value = %q, want %q", tt.path, m.Value, tt.value)
			}

			if len(m.Params) != len(tt.params) {
				t.Errorf("At(%q) params = %v, want %v", tt.path, m.Params, tt.params)
			}

			for key, want := range tt.params {
				got, ok := m.Params.Get(key)

				if !ok || got != want {
					t.Errorf("At(%q) Params.Get(%q) = %q, %v; want %q, true", tt.path, key, got, ok, want)
				}
			}
		})
	}
}

func TestInsertCatchAllErrors(t *testing.T) {
	tests := []struct {
		name  string
		setup []string
		path  string
		want  error
	}{
		{"empty name", nil, "/files/*", endpoint.ErrEmptyParamName},
		{"not last", nil, "/files/*path/x", endpoint.ErrCatchAllNotLast},
		{"second catch-all", nil, "/a/*x/*y", endpoint.ErrCatchAllNotLast},
		{"duplicate", []string{"/files/*path"}, "/files/*path", endpoint.ErrDuplicateRoute},
		{"conflict", []string{"/files/*path"}, "/files/*rest", endpoint.ErrParamConflict},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := endpoint.New[int]()

			for _, path := range tt.setup {
				mustInsert(t, r, path, 1)
			}

			err := r.Insert(tt.path, 2)

			if err == nil {
				t.Fatalf("Insert(%q) succeeded, want %v", tt.path, tt.want)
			}

			if !errors.Is(err, tt.want) {
				t.Errorf("Insert(%q) error = %v, want %v", tt.path, err, tt.want)
			}

			if !strings.Contains(err.Error(), tt.path) {
				t.Errorf("Insert(%q) error = %q, want it to include the route", tt.path, err)
			}
		})
	}
}

// TestTokenCharsAsLiterals checks that ':' and '*' are ordinary characters
// away from a segment boundary.
func TestTokenCharsAsLiterals(t *testing.T) {
	r := endpoint.New[int]()

	mustInsert(t, r, "/files*path", 1)
	mustInsert(t, r, "/a:b", 2)

	for path, want := range map[string]int{"/files*path": 1, "/a:b": 2} {
		m, err := r.At(path)

		if err != nil {
			t.Fatalf("At(%q) error = %v", path, err)
		}

		if m.Value != want {
			t.Errorf("At(%q) value = %d, want %d", path, m.Value, want)
		}
	}

	for _, path := range []string{"/files", "/a"} {
		if _, err := r.At(path); !errors.Is(err, endpoint.ErrNotFound) {
			t.Errorf("At(%q) error = %v, want ErrNotFound", path, err)
		}
	}
}

// TestAtBacktracking covers paths where a static branch matches at first but
// fails deeper, so matching must back up into a parameter or catch-all
// branch that can still match.
func TestAtBacktracking(t *testing.T) {
	r := endpoint.New[string]()

	mustInsert(t, r, "/users/list", "list")
	mustInsert(t, r, "/users/:id/posts", "posts")
	mustInsert(t, r, "/users/:id/comments", "comments")
	mustInsert(t, r, "/a/b/c/d", "abcd")
	mustInsert(t, r, "/a/:x/y/d", "axyd")
	mustInsert(t, r, "/files/list", "files-list")
	mustInsert(t, r, "/files/*path", "files-catchall")

	tests := []struct {
		path   string
		value  string
		params map[string]string
		found  bool
	}{
		// The static branch matches all the way.
		{"/users/list", "list", nil, true},
		// The static branch fails at the next segment; the param branch wins.
		{"/users/list/posts", "posts", map[string]string{"id": "list"}, true},
		{"/users/list/comments", "comments", map[string]string{"id": "list"}, true},
		// Backtracking across several static segments.
		{"/a/b/c/d", "abcd", nil, true},
		{"/a/b/y/d", "axyd", map[string]string{"x": "b"}, true},
		// The param branch fails too; the catch-all below the failed static
		// branch wins.
		{"/files/list", "files-list", nil, true},
		{"/files/list/deep/path", "files-catchall", map[string]string{"path": "list/deep/path"}, true},
		{"/files/list.txt", "files-catchall", map[string]string{"path": "list.txt"}, true},
		// Every branch fails: no match.
		{"/a/b/c/x", "", nil, false},
		{"/users/list/extra", "", nil, false},
	}

	for _, tt := range tests {
		t.Run(strings.ReplaceAll(tt.path, "/", "_"), func(t *testing.T) {
			m, err := r.At(tt.path)

			if !tt.found {
				if !errors.Is(err, endpoint.ErrNotFound) {
					t.Fatalf("At(%q) error = %v, want ErrNotFound", tt.path, err)
				}

				return
			}

			if err != nil {
				t.Fatalf("At(%q) error = %v", tt.path, err)
			}

			if m.Value != tt.value {
				t.Errorf("At(%q) value = %q, want %q", tt.path, m.Value, tt.value)
			}

			if len(m.Params) != len(tt.params) {
				t.Errorf("At(%q) params = %v, want %v", tt.path, m.Params, tt.params)
			}

			for key, want := range tt.params {
				got, ok := m.Params.Get(key)

				if !ok || got != want {
					t.Errorf("At(%q) Params.Get(%q) = %q, %v; want %q, true", tt.path, key, got, ok, want)
				}
			}
		})
	}
}

// TestAtManyParams exercises the fallback for routes with more captures than
// the inline array holds.
func TestAtManyParams(t *testing.T) {
	r := endpoint.New[int]()

	mustInsert(t, r, "/a/:p1/:p2/:p3/:p4/:p5/:p6/:p7/:p8/:p9", 1)

	m, err := r.At("/a/1/2/3/4/5/6/7/8/9")

	if err != nil {
		t.Fatalf("At error = %v", err)
	}

	if len(m.Params) != 9 {
		t.Fatalf("len(params) = %d, want 9", len(m.Params))
	}

	for i := 1; i <= 9; i++ {
		key := fmt.Sprint("p", i)
		want := fmt.Sprint(i)
		got, ok := m.Params.Get(key)

		if !ok || got != want {
			t.Errorf("Get(%q) = %q, %v; want %q, true", key, got, ok, want)
		}
	}
}

// The tests below compare At against a brute-force reference implementation of
// the matching rules documented in README.md.
//
// refMatch implements those rules for a single route: whether the route
// matches a path, the parameters it captures in route order, and a per-segment
// specificity vector (static = 2, parameter = 1, catch-all = 0). The winner
// among all matching routes is the one with the lexicographically greatest
// vector, and the shorter vector wins when one is a prefix of the other,
// because a route that ends exactly is more specific than one that continues
// with a catch-all.
//
// TestAtMatchesReference and FuzzAtMatchesReference register random route sets
// and compare Router.At against that reference, which catches regressions in
// precedence, backtracking, and parameter bookkeeping that tables of fixed
// cases can miss.

// refParam is one capture produced by the reference matcher.
type refParam struct {
	name  string
	value string
}

// refTokenIndex returns the index of the first ':' or '*' that starts a new
// segment in route, or -1 if there is none.
func refTokenIndex(route string) int {
	for i := 0; i < len(route); i++ {
		if c := route[i]; (c == ':' || c == '*') && (i == 0 || route[i-1] == '/') {
			return i
		}
	}

	return -1
}

// refMatch implements the documented matching rules for a single route. It
// returns the captures in route order and the route's specificity vector.
func refMatch(route, path string) ([]refParam, []int, bool) {
	var (
		params []refParam
		spec   []int
		i      int
		rest   = route
	)

	for rest != "" {
		tok := refTokenIndex(rest)
		chunk := rest

		if tok >= 0 {
			chunk = rest[:tok]
		}

		if !strings.HasPrefix(path[i:], chunk) {
			return nil, nil, false
		}

		i += len(chunk)

		// Literal segments are worth 2. A chunk ends with '/' exactly when a
		// token follows it, and that slash is a separator, not a segment.
		n := strings.Count(chunk, "/")
		if strings.HasSuffix(chunk, "/") {
			n--
		}

		for range n {
			spec = append(spec, 2)
		}

		if tok < 0 {
			break
		}

		rest = rest[tok:]
		catchAll := rest[0] == '*'

		seg := rest[1:]
		name, next := seg, ""

		if j := strings.IndexAny(seg, "/:*"); j >= 0 {
			name, next = seg[:j], seg[j:]
		}

		rest = next

		if catchAll {
			if rest != "" {
				return nil, nil, false // malformed; never generated
			}

			params = append(params, refParam{name, path[i:]})
			spec = append(spec, 0)
			i = len(path)

			break
		}

		// A parameter matches exactly one non-empty segment.
		segment := path[i:]
		if segment == "" || segment[0] == '/' {
			return nil, nil, false
		}

		if j := strings.IndexByte(segment, '/'); j >= 0 {
			params = append(params, refParam{name, segment[:j]})
			spec = append(spec, 1)
			i += j
		} else {
			params = append(params, refParam{name, segment})
			spec = append(spec, 1)
			i = len(path)
		}
	}

	if i != len(path) {
		return nil, nil, false
	}

	return params, spec, true
}

// moreSpecific reports whether a beats b.
func moreSpecific(a, b []int) bool {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}

	return len(a) < len(b)
}

// referenceMatch returns the route that wins for path under the documented
// rules, along with the parameters it captures.
func referenceMatch(routes []string, path string) (string, []refParam, bool) {
	var (
		pattern string
		params  []refParam
		best    []int
		found   bool
	)

	for _, route := range routes {
		captured, spec, ok := refMatch(route, path)
		if !ok {
			continue
		}

		if !found || moreSpecific(spec, best) {
			pattern, params, best, found = route, captured, spec, true
		}
	}

	return pattern, params, found
}

var (
	genStatics  = []string{"a", "b", "c", "aa", "ab", "abc", "x1"}
	genParams   = []string{"p", "q", "r"}
	genCatch    = []string{"z", "w"}
	genLits     = []string{"1", "hi", "q", "a", "b", "ab"}
	genPathSegs = []string{"a", "b", "c", "x1", "aa", "ab", "abc", "1", "hi", "q"}
)

// genRoute returns a random well-formed route of one to six segments, ending in
// a static segment, a parameter, or a catch-all.
func genRoute(rng *rand.Rand) string {
	n := 1 + rng.IntN(6)

	segs := make([]string, 0, n+1)
	for range n {
		if rng.IntN(2) == 0 {
			segs = append(segs, genStatics[rng.IntN(len(genStatics))])
		} else {
			segs = append(segs, ":"+genParams[rng.IntN(len(genParams))])
		}
	}

	if rng.IntN(3) == 0 {
		if rng.IntN(2) == 0 {
			segs = segs[:len(segs)-1]
		}

		segs = append(segs, "*"+genCatch[rng.IntN(len(genCatch))])
	}

	return "/" + strings.Join(segs, "/")
}

// instantiate turns route into a concrete path, replacing parameters with
// random non-empty segments and a catch-all with random text.
func instantiate(rng *rand.Rand, route string) string {
	segs := strings.Split(route[1:], "/")

	for i, seg := range segs {
		switch {
		case strings.HasPrefix(seg, ":"):
			segs[i] = genLits[rng.IntN(len(genLits))]
		case strings.HasPrefix(seg, "*"):
			switch rng.IntN(3) {
			case 0:
				segs[i] = ""
			case 1:
				segs[i] = genLits[rng.IntN(len(genLits))]
			default:
				segs[i] = genLits[rng.IntN(len(genLits))] + "/" + genLits[rng.IntN(len(genLits))]
			}
		}
	}

	return "/" + strings.Join(segs, "/")
}

// genPath returns a random path, either an instantiated route or random
// segments, including empty ones, which exercise the rule that a parameter
// never matches an empty segment.
func genPath(rng *rand.Rand, routes []string) string {
	if rng.IntN(2) == 0 {
		return instantiate(rng, routes[rng.IntN(len(routes))])
	}

	parts := make([]string, 1+rng.IntN(5))
	for i := range parts {
		if rng.IntN(4) == 0 {
			parts[i] = ""
		} else {
			parts[i] = genPathSegs[rng.IntN(len(genPathSegs))]
		}
	}

	return "/" + strings.Join(parts, "/")
}

// genRouteSet returns up to ten distinct random routes.
func genRouteSet(rng *rand.Rand) []string {
	routes := make([]string, 0, 10)
	seen := map[string]bool{}

	for range 1 + rng.IntN(10) {
		route := genRoute(rng)
		if !seen[route] {
			seen[route] = true
			routes = append(routes, route)
		}
	}

	return routes
}

// checkMatch compares one lookup against the reference matcher.
func checkMatch(t *testing.T, r *endpoint.Router[string], routes []string, path string) {
	t.Helper()

	wantPattern, wantParams, found := referenceMatch(routes, path)

	m, err := r.At(path)

	if !found {
		if !errors.Is(err, endpoint.ErrNotFound) {
			t.Fatalf("At(%q) = %q, %v; want ErrNotFound (routes %v)", path, m.Value, err, routes)
		}

		return
	}

	if err != nil {
		t.Fatalf("At(%q) error = %v; want %q (routes %v)", path, err, wantPattern, routes)
	}

	if m.Value != wantPattern {
		t.Fatalf("At(%q) value = %q; want %q (routes %v)", path, m.Value, wantPattern, routes)
	}

	if len(m.Params) != len(wantParams) {
		t.Fatalf("At(%q) params = %v; want %v (route %q)", path, m.Params, wantParams, wantPattern)
	}

	for i, want := range wantParams {
		if m.Params[i].Key != want.name || m.Params[i].Value != want.value {
			t.Fatalf("At(%q) params[%d] = %v; want %s=%s (route %q)", path, i, m.Params[i], want.name, want.value, wantPattern)
		}
	}

	m2, err2 := r.At(path)
	if err2 != nil || m2.Value != m.Value || len(m2.Params) != len(m.Params) {
		t.Fatalf("At(%q) is not deterministic: (%v, %v) then (%v, %v)", path, m, err, m2, err2)
	}
}

// checkRouteSet registers a random route set in two routers, in forward and
// reverse order, and checks a batch of lookups against the reference. It also
// checks that rejected routes do not change what the routers already matched.
func checkRouteSet(t *testing.T, rng *rand.Rand) {
	t.Helper()

	routes := genRouteSet(rng)

	r := endpoint.New[string]()

	for _, route := range routes {
		if err := r.Insert(route, route); err != nil {
			// Sets that conflict are covered by TestInsertParamErrors and
			// TestInsertCatchAllErrors.
			return
		}
	}

	rev := endpoint.New[string]()
	for i := len(routes) - 1; i >= 0; i-- {
		if err := rev.Insert(routes[i], routes[i]); err != nil {
			t.Fatalf("reverse insert of %q failed: %v", routes[i], err)
		}
	}

	paths := make([]string, 0, 2*len(routes)+6)
	for _, route := range routes {
		paths = append(paths, instantiate(rng, route), instantiate(rng, route))
	}

	for range 6 {
		paths = append(paths, genPath(rng, routes))
	}

	for _, path := range paths {
		checkMatch(t, r, routes, path)
		checkMatch(t, rev, routes, path)
	}

	for _, bad := range []string{"", "nope", "/a/:", "/a/*b/c", "/*x/y"} {
		if err := r.Insert(bad, bad); err == nil {
			t.Fatalf("insert of malformed route %q succeeded", bad)
		}
	}

	if err := r.Insert(routes[0], routes[0]); !errors.Is(err, endpoint.ErrDuplicateRoute) {
		t.Fatalf("duplicate insert of %q error = %v", routes[0], err)
	}

	for _, path := range paths {
		checkMatch(t, r, routes, path)
	}
}

// newRNG returns a deterministic generator for seed.
func newRNG(seed int64) *rand.Rand {
	return rand.New(rand.NewPCG(uint64(seed), uint64(seed)^0x9e3779b97f4a7c15))
}

// TestAtMatchesReference compares At against the reference matcher on random
// route sets. The generator is deterministic, so failures reproduce.
func TestAtMatchesReference(t *testing.T) {
	rng := newRNG(1)

	for range 20000 {
		checkRouteSet(t, rng)
	}
}

// FuzzAtMatchesReference runs the same comparison on a seed drawn from the
// fuzz corpus. go test runs the seeds below; go test
// -fuzz=FuzzAtMatchesReference explores further.
func FuzzAtMatchesReference(f *testing.F) {
	f.Add(int64(1))
	f.Add(int64(2))
	f.Add(int64(-7919))

	f.Fuzz(func(t *testing.T, seed int64) {
		checkRouteSet(t, newRNG(seed))
	})
}
