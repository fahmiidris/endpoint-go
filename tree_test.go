package endpoint_test

import (
	"errors"
	"fmt"
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
				if !errors.Is(err, endpoint.ERROR_NOT_FOUND) {
					t.Fatalf("At(%q) error = %v, want ERROR_NOT_FOUND", tt.path, err)
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

			if _, err := r.At("/apples"); !errors.Is(err, endpoint.ERROR_NOT_FOUND) {
				t.Errorf(`At("/apples") error = %v, want ERROR_NOT_FOUND`, err)
			}
		})
	}
}

func TestInsertDuplicateStatic(t *testing.T) {
	r := endpoint.New[int]()

	mustInsert(t, r, "/app", 1)

	err := r.Insert("/app", 2)

	if !errors.Is(err, endpoint.ERROR_DUPLICATE_ROUTE) {
		t.Errorf("Insert duplicate error = %v, want ERROR_DUPLICATE_ROUTE", err)
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
				if !errors.Is(err, endpoint.ERROR_NOT_FOUND) {
					t.Fatalf("At(%q) error = %v, want ERROR_NOT_FOUND", tt.path, err)
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
		{"empty name", nil, "/users/:", endpoint.ERROR_EMPTY_PARAM_NAME},
		{"empty name at root", nil, "/:", endpoint.ERROR_EMPTY_PARAM_NAME},
		{"misplaced token", nil, "/users/:id:x", endpoint.ERROR_INVALID_ROUTE},
		{"duplicate", []string{"/users/:id"}, "/users/:id", endpoint.ERROR_DUPLICATE_ROUTE},
		{"conflict", []string{"/users/:id"}, "/users/:name", endpoint.ERROR_PARAM_CONFLICT},
		{"conflict before suffix", []string{"/users/:id"}, "/users/:name/posts", endpoint.ERROR_PARAM_CONFLICT},
		{"conflict beneath shared prefix", []string{"/a/:x/b/:z"}, "/a/:x/b/:y", endpoint.ERROR_PARAM_CONFLICT},
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
				if !errors.Is(err, endpoint.ERROR_NOT_FOUND) {
					t.Fatalf("At(%q) error = %v, want ERROR_NOT_FOUND", tt.path, err)
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
		{"empty name", nil, "/files/*", endpoint.ERROR_EMPTY_PARAM_NAME},
		{"not last", nil, "/files/*path/x", endpoint.ERROR_CATCH_ALL_NOT_LAST},
		{"second catch-all", nil, "/a/*x/*y", endpoint.ERROR_CATCH_ALL_NOT_LAST},
		{"duplicate", []string{"/files/*path"}, "/files/*path", endpoint.ERROR_DUPLICATE_ROUTE},
		{"conflict", []string{"/files/*path"}, "/files/*rest", endpoint.ERROR_PARAM_CONFLICT},
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
		if _, err := r.At(path); !errors.Is(err, endpoint.ERROR_NOT_FOUND) {
			t.Errorf("At(%q) error = %v, want ERROR_NOT_FOUND", path, err)
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
				if !errors.Is(err, endpoint.ERROR_NOT_FOUND) {
					t.Fatalf("At(%q) error = %v, want ERROR_NOT_FOUND", tt.path, err)
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
