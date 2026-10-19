package endpoint

import "strings"

// nodeKind distinguishes the roles a node can play in the tree.
type nodeKind uint8

const (
	kindStatic   nodeKind = iota // matches a literal path fragment
	kindParam                    // matches exactly one non-empty path segment
	kindCatchAll                 // matches the rest of the path, possibly empty
)

// node is a node of the radix tree.
//
// Static nodes own a compressed path fragment (prefix). Param and catch-all
// nodes have an empty prefix: their parent captures the text they match from
// the input path before descending into them.
//
// Static children are stored in children, keyed by the first byte of their
// prefix, which is kept in the parallel indices string. A node has at most one
// param and at most one catch-all child. value/hasValue record the route that
// ends exactly at this node; hasValue is explicit because T may be a zero
// value.
type node[T any] struct {
	kind   nodeKind
	prefix string
	name   string

	indices  string
	children []*node[T]

	param    *node[T]
	catchAll *node[T]

	value    T
	hasValue bool
}

// insert registers value for the remainder of a route.
//
// path is the part of the route that has not been consumed yet; it is empty or
// starts with '/' because every route starts with '/'.
func (n *node[T]) insert(path string, value T) error {
	if path == "" {
		if n.hasValue {
			return ErrDuplicateRoute
		}

		n.value = value
		n.hasValue = true

		return nil
	}

	if i := tokenIndex(path); i >= 0 {
		return n.insertToken(path[:i], path[i:], value)
	}

	return n.insertStatic(path, value)
}

// tokenIndex returns the index of the first ":" or "*" that starts a new
// segment, or -1 if there is none. Only tokens at a segment boundary count;
// anywhere else ":" and "*" are ordinary characters.
func tokenIndex(path string) int {
	for i := 0; i < len(path); i++ {
		if c := path[i]; (c == ':' || c == '*') && (i == 0 || path[i-1] == '/') {
			return i
		}
	}

	return -1
}

// parseToken splits a parameter token (":name" or "*name") from the rest of
// the path, which must start with ':' or '*'. The name is the longest run of
// characters that are not '/', ':' or '*'. catchAll reports whether the token
// was introduced by '*'.
func parseToken(path string) (name, rest string, catchAll bool) {
	catchAll = path[0] == '*'
	seg := path[1:]

	if i := strings.IndexAny(seg, "/:*"); i >= 0 {
		return seg[:i], seg[i:], catchAll
	}

	return seg, "", catchAll
}

// insertToken registers value for a route whose next segment is a parameter.
// pre is the static part before the token; token starts with ':' or '*'.
func (n *node[T]) insertToken(pre, token string, value T) error {
	mid := n.descendStatic(pre)
	name, rest, catchAll := parseToken(token)

	if name == "" {
		return ErrEmptyParamName
	}

	if catchAll {
		if rest != "" {
			return ErrCatchAllNotLast
		}

		if mid.catchAll == nil {
			mid.catchAll = &node[T]{kind: kindCatchAll, name: name}
		} else if mid.catchAll.name != name {
			return ErrParamConflict
		}

		if mid.catchAll.hasValue {
			return ErrDuplicateRoute
		}

		mid.catchAll.value = value
		mid.catchAll.hasValue = true

		return nil
	}

	if rest != "" && rest[0] != '/' {
		return ErrInvalidRoute
	}

	if mid.param == nil {
		mid.param = &node[T]{kind: kindParam, name: name}
	} else if mid.param.name != name {
		return ErrParamConflict
	}

	return mid.param.insert(rest, value)
}

// insertStatic registers value at the end of chunk, which must contain no
// parameter tokens.
func (n *node[T]) insertStatic(chunk string, value T) error {
	leaf := n.descendStatic(chunk)

	if leaf.hasValue {
		return ErrDuplicateRoute
	}

	leaf.value = value
	leaf.hasValue = true

	return nil
}

// descendStatic walks to the node exactly at the end of chunk, creating nodes
// and splitting existing ones where prefixes diverge. chunk must not contain
// parameter tokens.
func (n *node[T]) descendStatic(chunk string) *node[T] {
	for chunk != "" {
		i := n.staticChildIndex(chunk[0])

		if i < 0 {
			leaf := &node[T]{prefix: chunk}

			n.addStaticChild(leaf)

			return leaf
		}

		child := n.children[i]
		common := commonPrefixLen(child.prefix, chunk)

		if common == len(child.prefix) {
			// The whole child prefix matches: keep descending.
			n = child
			chunk = chunk[common:]

			continue
		}

		// The child's prefix extends past the common prefix: split it into a
		// middle node (the common prefix) and the child (the remainder).
		mid := &node[T]{prefix: child.prefix[:common]}

		child.prefix = child.prefix[common:]

		mid.addStaticChild(child)

		n.children[i] = mid

		if common == len(chunk) {
			return mid
		}

		leaf := &node[T]{prefix: chunk[common:]}

		mid.addStaticChild(leaf)

		return leaf
	}

	return n
}

// staticChildIndex returns the position in children of the static child whose
// prefix starts with b, or -1 if there is none.
func (n *node[T]) staticChildIndex(b byte) int {
	return strings.IndexByte(n.indices, b)
}

// addStaticChild attaches c, whose prefix must not be empty, as a static child.
func (n *node[T]) addStaticChild(c *node[T]) {
	n.indices += c.prefix[:1]
	n.children = append(n.children, c)
}

// commonPrefixLen returns the length of the longest common prefix of a and b.
func commonPrefixLen(a, b string) int {
	n := min(len(a), len(b))

	for i := range n {
		if a[i] != b[i] {
			return i
		}
	}

	return n
}

// search matches path against the subtree rooted at n, where n's own prefix
// (if any) still sits at the start of path. It returns the node holding the
// matched route's value, the number of parameters the route captured, and
// whether a route matched. Captures are written to params[depth], but only
// while they fit; extra captures are counted and skipped.
func (n *node[T]) search(path string, params []Param, depth int) (*node[T], int, bool) {
	if n.kind == kindStatic {
		if !strings.HasPrefix(path, n.prefix) {
			return nil, 0, false
		}

		path = path[len(n.prefix):]
	}

	if path == "" && n.hasValue {
		return n, depth, true
	}

	if path != "" {
		if i := n.staticChildIndex(path[0]); i >= 0 {
			if m, d, ok := n.children[i].search(path, params, depth); ok {
				return m, d, true
			}
		}

		if n.param != nil {
			// A parameter matches up to the next '/', but never an empty
			// segment (i == 0).
			if i := strings.IndexByte(path, '/'); i != 0 {
				seg, rest := path, ""

				if i > 0 {
					seg, rest = path[:i], path[i:]
				}

				if depth < len(params) {
					params[depth] = Param{Key: n.param.name, Value: seg}
				}

				if m, d, ok := n.param.search(rest, params, depth+1); ok {
					return m, d, true
				}
			}
		}
	}

	if n.catchAll != nil {
		// A catch-all takes the rest of the path, which may be empty.
		if depth < len(params) {
			params[depth] = Param{Key: n.catchAll.name, Value: path}
		}

		return n.catchAll, depth + 1, true
	}

	return nil, 0, false
}
