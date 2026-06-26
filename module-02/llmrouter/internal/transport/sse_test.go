package transport

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

func TestParseSSEAggregatesDataLines(t *testing.T) {
	input := "" +
		": heartbeat\n" +
		"event: content_block_delta\n" +
		"id: 42\n" +
		"data: {\"first\":1,\n" +
		"data: \"second\":2}\n" +
		"\n" +
		"data: [DONE]\n"

	var events []SSEEvent
	err := ParseSSE(context.Background(), strings.NewReader(input), func(event SSEEvent) error {
		events = append(events, event)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	want := []SSEEvent{
		{
			Event: "content_block_delta",
			ID:    "42",
			Data:  []byte("{\"first\":1,\n\"second\":2}"),
		},
		{Data: []byte("[DONE]")},
	}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %#v, want %#v", events, want)
	}
}
