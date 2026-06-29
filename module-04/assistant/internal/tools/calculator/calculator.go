package calculator

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"

	"github.com/q1mi/assistant/internal/schema"
)

// Args 是 calculator 工具的 JSON 参数。
type Args struct {
	Expr string `json:"expr" description:"要计算的四则运算表达式，例如 1+2*3"`
}

// Tool 对应 M04 配套练习的 calculator 工具。
// 参数 Schema 由 schema.Generate(Args{}) 生成，对应课件 2.8 与 4.2 的组合使用。
type Tool struct {
	parameters json.RawMessage
}

// New 创建 calculator 工具，并生成参数 Schema。
func New() (*Tool, error) {
	parameters, err := schema.Generate(Args{})
	if err != nil {
		return nil, err
	}
	return &Tool{parameters: schema.MustJSON(parameters)}, nil
}

// MustNew 创建 calculator 工具，失败时 panic。
func MustNew() *Tool {
	tool, err := New()
	if err != nil {
		panic(err)
	}
	return tool
}

// Name 返回工具名。
func (tool *Tool) Name() string {
	return "calculator"
}

// Description 返回工具说明。
func (tool *Tool) Description() string {
	return "计算只包含数字、括号、+、-、*、/ 的算术表达式"
}

// Parameters 返回工具参数 Schema。
func (tool *Tool) Parameters() json.RawMessage {
	return tool.parameters
}

// Call 执行 calculator 工具调用。
func (tool *Tool) Call(_ context.Context, raw json.RawMessage) (string, error) {
	var args Args
	if err := json.Unmarshal(raw, &args); err != nil {
		return "", fmt.Errorf("解析 calculator 参数: %w", err)
	}
	expr := strings.TrimSpace(args.Expr)
	if expr == "" {
		return "", fmt.Errorf("expr 不能为空")
	}
	value, err := Eval(expr)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s = %s", expr, formatFloat(value)), nil
}

// Eval 计算只包含数字、括号和四则运算符的表达式。
func Eval(expr string) (float64, error) {
	parser := expressionParser{input: []rune(expr)}
	value, err := parser.parseExpression()
	if err != nil {
		return 0, err
	}
	parser.skipSpace()
	if parser.pos != len(parser.input) {
		return 0, fmt.Errorf("表达式在位置 %d 处有多余内容", parser.pos)
	}
	if math.IsInf(value, 0) || math.IsNaN(value) {
		return 0, fmt.Errorf("计算结果无效")
	}
	return value, nil
}

type expressionParser struct {
	input []rune
	pos   int
}

// parseExpression 解析加法和减法层级。
func (parser *expressionParser) parseExpression() (float64, error) {
	value, err := parser.parseTerm()
	if err != nil {
		return 0, err
	}
	for {
		parser.skipSpace()
		switch parser.peek() {
		case '+':
			parser.pos++
			next, err := parser.parseTerm()
			if err != nil {
				return 0, err
			}
			value += next
		case '-':
			parser.pos++
			next, err := parser.parseTerm()
			if err != nil {
				return 0, err
			}
			value -= next
		default:
			return value, nil
		}
	}
}

// parseTerm 解析乘法和除法层级。
func (parser *expressionParser) parseTerm() (float64, error) {
	value, err := parser.parseFactor()
	if err != nil {
		return 0, err
	}
	for {
		parser.skipSpace()
		switch parser.peek() {
		case '*':
			parser.pos++
			next, err := parser.parseFactor()
			if err != nil {
				return 0, err
			}
			value *= next
		case '/':
			parser.pos++
			next, err := parser.parseFactor()
			if err != nil {
				return 0, err
			}
			if next == 0 {
				return 0, fmt.Errorf("除数不能为 0")
			}
			value /= next
		default:
			return value, nil
		}
	}
}

// parseFactor 解析符号、括号和数字。
func (parser *expressionParser) parseFactor() (float64, error) {
	parser.skipSpace()
	switch parser.peek() {
	case '+':
		parser.pos++
		return parser.parseFactor()
	case '-':
		parser.pos++
		value, err := parser.parseFactor()
		return -value, err
	case '(':
		parser.pos++
		value, err := parser.parseExpression()
		if err != nil {
			return 0, err
		}
		parser.skipSpace()
		if parser.peek() != ')' {
			return 0, fmt.Errorf("缺少右括号")
		}
		parser.pos++
		return value, nil
	default:
		return parser.parseNumber()
	}
}

// parseNumber 解析浮点数字面量。
func (parser *expressionParser) parseNumber() (float64, error) {
	parser.skipSpace()
	start := parser.pos
	dotSeen := false
	for parser.pos < len(parser.input) {
		r := parser.input[parser.pos]
		if unicode.IsDigit(r) {
			parser.pos++
			continue
		}
		if r == '.' && !dotSeen {
			dotSeen = true
			parser.pos++
			continue
		}
		break
	}
	if start == parser.pos {
		return 0, fmt.Errorf("位置 %d 需要数字", parser.pos)
	}
	value, err := strconv.ParseFloat(string(parser.input[start:parser.pos]), 64)
	if err != nil {
		return 0, fmt.Errorf("解析数字: %w", err)
	}
	return value, nil
}

// skipSpace 跳过表达式中的空白字符。
func (parser *expressionParser) skipSpace() {
	for parser.pos < len(parser.input) && unicode.IsSpace(parser.input[parser.pos]) {
		parser.pos++
	}
}

// peek 查看当前字符，越界时返回 0。
func (parser *expressionParser) peek() rune {
	if parser.pos >= len(parser.input) {
		return 0
	}
	return parser.input[parser.pos]
}

// formatFloat 把整数结果格式化为不带小数的字符串。
func formatFloat(value float64) string {
	if math.Abs(value-math.Round(value)) < 1e-9 {
		return strconv.FormatInt(int64(math.Round(value)), 10)
	}
	return strconv.FormatFloat(value, 'f', -1, 64)
}
