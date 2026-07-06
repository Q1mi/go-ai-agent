package mas

import (
	"context"
	"reflect"
	"sort"
	"testing"
)

func TestStage(t *testing.T) {
	ctx := context.Background()
	in := make(chan int)
	go func() {
		defer close(in)
		for _, value := range []int{1, 2, 3, 4} {
			in <- value
		}
	}()

	out := Stage(ctx, in, 2, func(ctx context.Context, value int) (int, error) {
		return value * value, nil
	})

	var got []int
	for value := range out {
		got = append(got, value)
	}
	sort.Ints(got)
	want := []int{1, 4, 9, 16}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Stage()=%v, want %v", got, want)
	}
}
