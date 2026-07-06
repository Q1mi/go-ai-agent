package rag

import (
	"reflect"
	"testing"
)

func TestRRF(t *testing.T) {
	tests := []struct {
		name     string
		rankings [][]string
		k        int
		want     []string
	}{
		{
			name: "merge vector and keyword rankings",
			rankings: [][]string{
				{"a", "b", "c"},
				{"b", "d", "a"},
			},
			k:    60,
			want: []string{"b", "a", "d", "c"},
		},
		{
			name: "ignore empty ids and sort ties by id",
			rankings: [][]string{
				{"b", "", "a"},
				{"c"},
			},
			k:    1,
			want: []string{"b", "c", "a"},
		},
		{
			name:     "empty rankings",
			rankings: nil,
			k:        60,
			want:     []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RRF(tt.rankings, tt.k)
			if got == nil {
				got = []string{}
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("RRF()=%v, want %v", got, tt.want)
			}
		})
	}
}
