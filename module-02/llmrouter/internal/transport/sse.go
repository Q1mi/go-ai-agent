package transport

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
)

type SSEEvent struct {
	Event string
	ID    string
	Data  []byte
}

func ParseSSE(ctx context.Context, reader io.Reader, onEvent func(SSEEvent) error) error {
	if reader == nil {
		return fmt.Errorf("SSE reader 不能为空")
	}
	if onEvent == nil {
		return fmt.Errorf("SSE callback 不能为空")
	}

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	var event SSEEvent
	var dataLines []string
	flush := func() error {
		if len(dataLines) == 0 {
			event = SSEEvent{}
			return nil
		}
		event.Data = []byte(strings.Join(dataLines, "\n"))
		if err := onEvent(event); err != nil {
			return err
		}
		event = SSEEvent{}
		dataLines = dataLines[:0]
		return nil
	}

	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return err
		}

		line := strings.TrimSuffix(scanner.Text(), "\r")
		if line == "" {
			if err := flush(); err != nil {
				return err
			}
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}

		field, value, found := strings.Cut(line, ":")
		if !found {
			field = line
			value = ""
		} else {
			value = strings.TrimPrefix(value, " ")
		}

		switch field {
		case "event":
			event.Event = value
		case "id":
			event.ID = value
		case "data":
			dataLines = append(dataLines, value)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("读取 SSE: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return flush()
}
