package endpoint_test

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	endpoint "github.com/fahmiidris/endpoint-go"
)

func TestInsertInvalidRoute(t *testing.T) {
	r := endpoint.New[int]()

	for _, path := range []string{"", "users", "users/1"} {
		err := r.Insert(path, 1)

		if err == nil {
			t.Fatalf("Insert(%q) succeeded, want ErrInvalidRoute", path)
		}

		if !errors.Is(err, endpoint.ErrInvalidRoute) {
			t.Errorf("Insert(%q) error = %v, want ErrInvalidRoute", path, err)
		}

		if !strings.Contains(err.Error(), path) {
			t.Errorf("Insert(%q) error = %q, want it to include the route", path, err)
		}
	}
}

func TestZeroValueFound(t *testing.T) {
	r := endpoint.New[int]()

	mustInsert(t, r, "/zero", 0)

	m, err := r.At("/zero")

	if err != nil {
		t.Fatalf("At error = %v", err)
	}

	if m.Value != 0 {
		t.Errorf("value = %d, want 0", m.Value)
	}
}

func TestAtEmptyRouter(t *testing.T) {
	r := endpoint.New[int]()

	for _, path := range []string{"/", "/users", ""} {
		if _, err := r.At(path); !errors.Is(err, endpoint.ErrNotFound) {
			t.Errorf("At(%q) error = %v, want ErrNotFound", path, err)
		}
	}
}

func TestAtConcurrent(t *testing.T) {
	r := endpoint.New[int]()

	mustInsert(t, r, "/users/list", 1)
	mustInsert(t, r, "/users/:id", 2)
	mustInsert(t, r, "/files/*path", 3)

	var wg sync.WaitGroup

	for range 8 {
		wg.Go(func() {
			for range 500 {
				if m, err := r.At("/users/42"); err != nil || m.Value != 2 {
					t.Errorf("At(%q) = %v, %v; want value 2, nil", "/users/42", m.Value, err)
					return
				}

				if m, err := r.At("/files/a/b"); err != nil || m.Value != 3 {
					t.Errorf("At(%q) = %v, %v; want value 3, nil", "/files/a/b", m.Value, err)
					return
				}
			}
		})
	}

	wg.Wait()
}

// sinkInt and sinkString keep lookup results alive so the compiler cannot
// discard the calls being measured.
var (
	sinkInt    int
	sinkString string
)

// TestAtDoesNotAllocate asserts that ordinary lookups allocate nothing, using
// the same measurement the benchmarks report.
func TestAtDoesNotAllocate(t *testing.T) {
	if mode := testing.CoverMode(); mode != "" {
		t.Skipf("coverage mode %q disables the inlining this test relies on", mode)
	}

	r := endpoint.New[int]()

	mustInsert(t, r, "/users/list", 1)
	mustInsert(t, r, "/users/:id", 2)
	mustInsert(t, r, "/files/*path", 3)

	lookups := []struct {
		name string
		path string
	}{
		{"static", "/users/list"},
		{"param", "/users/42"},
		{"catch-all", "/files/a/b/c"},
	}
	for _, tt := range lookups {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := r.At(tt.path); err != nil {
				t.Fatalf("At(%q) error = %v", tt.path, err)
			}

			allocs := testing.AllocsPerRun(1000, func() {
				m, _ := r.At(tt.path)
				sinkInt = m.Value

				for _, p := range m.Params {
					sinkString = p.Value
				}
			})

			if allocs != 0 {
				t.Errorf("At(%q) allocated %v times per call, want 0", tt.path, allocs)
			}
		})
	}
}

func BenchmarkAt_Static(b *testing.B) {
	r := endpoint.New[int]()

	mustInsertB(b, r, "/", 0)
	mustInsertB(b, r, "/users", 1)
	mustInsertB(b, r, "/users/list", 2)
	mustInsertB(b, r, "/users/list/active", 3)
	mustInsertB(b, r, "/app", 4)
	mustInsertB(b, r, "/apple", 5)

	b.ReportAllocs()

	for b.Loop() {
		m, err := r.At("/users/list/active")

		if err != nil {
			b.Fatal(err)
		}

		sinkInt = m.Value
	}
}

func BenchmarkAt_Params(b *testing.B) {
	r := endpoint.New[int]()

	mustInsertB(b, r, "/users/:id", 1)
	mustInsertB(b, r, "/users/:id/posts", 2)
	mustInsertB(b, r, "/users/:id/posts/:post", 3)

	b.ReportAllocs()

	for b.Loop() {
		m, err := r.At("/users/42/posts/7")

		if err != nil {
			b.Fatal(err)
		}

		id, _ := m.Params.Get("id")
		sinkString = id
	}
}

func mustInsertB[T any](b *testing.B, r *endpoint.Router[T], path string, value T) {
	b.Helper()

	if err := r.Insert(path, value); err != nil {
		b.Fatalf("Insert(%q) error: %v", path, err)
	}
}

func Example() {
	r := endpoint.New[string]()

	for _, path := range []string{"/users", "/users/:id", "/files/*path"} {
		if err := r.Insert(path, path); err != nil {
			panic(err)
		}
	}

	m, _ := r.At("/users/42")
	id, _ := m.Params.Get("id")

	fmt.Println(m.Value, id)

	m, _ = r.At("/files/notes/2024/todo.txt")
	path, _ := m.Params.Get("path")

	fmt.Println(m.Value, path)

	_, err := r.At("/unknown")

	fmt.Println(err)

	// Output:
	// /users/:id 42
	// /files/*path notes/2024/todo.txt
	// endpoint: no matching route
}
