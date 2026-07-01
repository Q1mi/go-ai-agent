package builtin

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"

	"github.com/q1mi/mcptools/internal/tool"
)

// CalcArgs 是 calc 工具参数。
type CalcArgs struct {
	Expr string `json:"expr" desc:"四则运算表达式，例如 1+2*3"`
}

// NewCalcTool 创建四则运算工具。
func NewCalcTool() tool.Tool {
	return tool.NewTypedTool("calc", "计算只包含数字、括号、+、-、*、/ 的算术表达式", func(_ context.Context, args CalcArgs) (string, error) {
		expr := strings.TrimSpace(args.Expr)
		if expr == "" {
			return "", fmt.Errorf("expr 不能为空")
		}
		value, err := Eval(expr)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s = %s", expr, formatFloat(value)), nil
	})
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

func (parser *expressionParser) parseExpression() (float64, error) {
	value, err := parser.parseTerm()
	if err != nil {
		return 0, err
	}
	for {
		parser.skipSpace()
		if parser.match('+') {
			right, err := parser.parseTerm()
			if err != nil {
				return 0, err
			}
			value += right
			continue
		}
		if parser.match('-') {
			right, err := parser.parseTerm()
			if err != nil {
				return 0, err
			}
			value -= right
			continue
		}
		return value, nil
	}
}

func (parser *expressionParser) parseTerm() (float64, error) {
	value, err := parser.parseFactor()
	if err != nil {
		return 0, err
	}
	for {
		parser.skipSpace()
		if parser.match('*') {
			right, err := parser.parseFactor()
			if err != nil {
				return 0, err
			}
			value *= right
			continue
		}
		if parser.match('/') {
			right, err := parser.parseFactor()
			if err != nil {
				return 0, err
			}
			if right == 0 {
				return 0, fmt.Errorf("除数不能为 0")
			}
			value /= right
			continue
		}
		return value, nil
	}
}

func (parser *expressionParser) parseFactor() (float64, error) {
	parser.skipSpace()
	if parser.match('+') {
		return parser.parseFactor()
	}
	if parser.match('-') {
		value, err := parser.parseFactor()
		return -value, err
	}
	if parser.match('(') {
		value, err := parser.parseExpression()
		if err != nil {
			return 0, err
		}
		parser.skipSpace()
		if !parser.match(')') {
			return 0, fmt.Errorf("缺少右括号")
		}
		return value, nil
	}
	return parser.parseNumber()
}

func (parser *expressionParser) parseNumber() (float64, error) {
	parser.skipSpace()
	start := parser.pos
	for parser.pos < len(parser.input) {
		r := parser.input[parser.pos]
		if !unicode.IsDigit(r) && r != '.' {
			break
		}
		parser.pos++
	}
	if start == parser.pos {
		return 0, fmt.Errorf("位置 %d 处需要数字", parser.pos)
	}
	value, err := strconv.ParseFloat(string(parser.input[start:parser.pos]), 64)
	if err != nil {
		return 0, fmt.Errorf("解析数字失败: %w", err)
	}
	return value, nil
}

func (parser *expressionParser) skipSpace() {
	for parser.pos < len(parser.input) && unicode.IsSpace(parser.input[parser.pos]) {
		parser.pos++
	}
}

func (parser *expressionParser) match(want rune) bool {
	if parser.pos < len(parser.input) && parser.input[parser.pos] == want {
		parser.pos++
		return true
	}
	return false
}

func formatFloat(value float64) string {
	if math.Trunc(value) == value {
		return strconv.FormatInt(int64(value), 10)
	}
	return strconv.FormatFloat(value, 'f', -1, 64)
}
