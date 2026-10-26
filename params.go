package endpoint

// Param is one captured parameter: the name from the route and the path text
// it matched. Value is a substring of the path passed to At, never a copy, so
// retaining a Param keeps the whole path string alive.
type Param struct {
	Key   string
	Value string
}

// Params holds the parameters captured by a lookup, in the order they appear
// in the matched route.
type Params []Param

// Get returns the value captured for the parameter with the given name, and
// reports whether that parameter was captured. If a route uses the same name
// more than once, Get returns the first one.
func (p Params) Get(name string) (string, bool) {
	for _, param := range p {
		if param.Key == name {
			return param.Value, true
		}
	}

	return "", false
}
