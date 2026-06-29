package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"golang.org/x/term"
)

var errPromptInputInterrupted = errors.New("输入被中断")

// promptLineReader 为交互模式提供最小行编辑能力。
//
// 在真实终端里它会显式处理 Backspace；在测试和管道输入里，它会清理输入文本中
// 已经出现的 Backspace 控制字符。
type promptLineReader struct {
	stdin      io.Reader
	stdout     io.Writer
	buffered   *bufio.Reader
	stdinFile  *os.File
	stdoutFile *os.File
	terminal   bool
}

// newPromptLineReader 根据 stdin/stdout 判断是否启用终端行编辑。
func newPromptLineReader(stdin io.Reader, stdout io.Writer) *promptLineReader {
	reader := &promptLineReader{
		stdin:    stdin,
		stdout:   stdout,
		buffered: bufio.NewReader(stdin),
	}
	stdinFile, stdinOK := stdin.(*os.File)
	stdoutFile, stdoutOK := stdout.(*os.File)
	if stdinOK && stdoutOK &&
		term.IsTerminal(int(stdinFile.Fd())) &&
		term.IsTerminal(int(stdoutFile.Fd())) {
		reader.stdinFile = stdinFile
		reader.stdoutFile = stdoutFile
		reader.terminal = true
	}
	return reader
}

// ReadLine 读取一行用户输入。
func (reader *promptLineReader) ReadLine(ctx context.Context, prompt string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if reader.terminal {
		return reader.readTerminalLine(ctx, prompt)
	}
	return reader.readBufferedLine(ctx, prompt)
}

// readBufferedLine 处理非 TTY 输入，适合测试、管道和重定向。
func (reader *promptLineReader) readBufferedLine(ctx context.Context, prompt string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if _, err := fmt.Fprint(reader.stdout, prompt); err != nil {
		return "", err
	}
	line, err := reader.buffered.ReadString('\n')
	if err != nil {
		if errors.Is(err, io.EOF) && line != "" {
			return applyBackspace(strings.TrimRight(line, "\r\n")), nil
		}
		return "", err
	}
	return applyBackspace(strings.TrimRight(line, "\r\n")), nil
}

// readTerminalLine 在真实终端中进入 raw mode，逐字符处理输入。
func (reader *promptLineReader) readTerminalLine(ctx context.Context, prompt string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	oldState, err := term.MakeRaw(int(reader.stdinFile.Fd()))
	if err != nil {
		return "", err
	}
	defer func() {
		_ = term.Restore(int(reader.stdinFile.Fd()), oldState)
	}()

	if _, err := fmt.Fprint(reader.stdout, prompt); err != nil {
		return "", err
	}
	input := bufio.NewReader(reader.stdinFile)
	var chars []rune
	for {
		r, _, err := input.ReadRune()
		if err != nil {
			return "", err
		}
		switch r {
		case '\r', '\n':
			_, _ = fmt.Fprint(reader.stdout, "\r\n")
			return string(chars), nil
		case 3: // Ctrl+C
			_, _ = fmt.Fprint(reader.stdout, "^C\r\n")
			return "", errPromptInputInterrupted
		case 4: // Ctrl+D
			if len(chars) == 0 {
				_, _ = fmt.Fprint(reader.stdout, "\r\n")
				return "", io.EOF
			}
		case '\b', 0x7f:
			if len(chars) > 0 {
				chars = chars[:len(chars)-1]
				repaintPromptLine(reader.stdout, prompt, chars)
			}
		case 0x1b:
			// 忽略方向键、Delete 等 ESC 序列，保持最小实现可预测。
			continue
		default:
			if r == utf8.RuneError {
				continue
			}
			if r >= 0x20 {
				chars = append(chars, r)
				repaintPromptLine(reader.stdout, prompt, chars)
			}
		}
	}
}

// applyBackspace 根据 Backspace / Ctrl-H 控制字符修正输入内容。
func applyBackspace(text string) string {
	var out []rune
	for _, r := range text {
		switch r {
		case '\b', 0x7f:
			if len(out) > 0 {
				out = out[:len(out)-1]
			}
		default:
			out = append(out, r)
		}
	}
	return string(out)
}

// repaintPromptLine 清空当前行并重新绘制 prompt 和输入内容。
func repaintPromptLine(out io.Writer, prompt string, chars []rune) {
	_, _ = fmt.Fprintf(out, "\r\033[K%s%s", prompt, string(chars))
}

// isPromptInputDone 判断交互输入是否正常结束。
func isPromptInputDone(err error) bool {
	return errors.Is(err, io.EOF) || errors.Is(err, errPromptInputInterrupted)
}
