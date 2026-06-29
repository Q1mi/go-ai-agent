package plan

import (
	"reflect"
	"testing"
)

func TestLevels(t *testing.T) {
	tests := []struct {
		name    string
		plan    Plan
		want    [][]string
		wantErr bool
	}{
		{
			name: "linear and parallel",
			plan: Plan{Tasks: []Task{
				{ID: "fetch"},
				{ID: "calc"},
				{ID: "answer", DependsOn: []string{"fetch", "calc"}},
			}},
			want: [][]string{{"calc", "fetch"}, {"answer"}},
		},
		{
			name: "missing dependency",
			plan: Plan{Tasks: []Task{
				{ID: "answer", DependsOn: []string{"missing"}},
			}},
			wantErr: true,
		},
		{
			name: "cycle",
			plan: Plan{Tasks: []Task{
				{ID: "a", DependsOn: []string{"b"}},
				{ID: "b", DependsOn: []string{"a"}},
			}},
			wantErr: true,
		},
		{
			name: "duplicate id",
			plan: Plan{Tasks: []Task{
				{ID: "a"},
				{ID: "a"},
			}},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Levels(tt.plan)
			if tt.wantErr {
				if err == nil {
					t.Fatal("期望错误")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("Levels() = %#v, want %#v", got, tt.want)
			}
		})
	}
}
