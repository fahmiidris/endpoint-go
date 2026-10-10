package endpoint_test

import (
	"testing"

	endpoint "github.com/fahmiidris/endpoint-go"
)

func TestParamsGet(t *testing.T) {
	params := endpoint.Params{
		{Key: "id", Value: "7"},
		{Key: "post", Value: "9"},
		{Key: "id", Value: "42"},
	}

	tests := []struct {
		name   string
		params endpoint.Params
		key    string
		want   string
		found  bool
	}{
		{"single", params, "post", "9", true},
		{"first of duplicates wins", params, "id", "7", true},
		{"missing key", params, "comment", "", false},
		{"empty params", endpoint.Params{}, "id", "", false},
		{"nil params", nil, "id", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := tt.params.Get(tt.key)

			if got != tt.want || ok != tt.found {
				t.Errorf("Get(%q) = %q, %v; want %q, %v", tt.key, got, ok, tt.want, tt.found)
			}
		})
	}
}
