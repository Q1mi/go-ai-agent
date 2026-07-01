package calc

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"
)

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

// Format 把整数结果输出为整数形式，保留小数结果的有效位。
func Format(value float64) string {
	if math.Trunc(value) == value {
		return strconv.FormatInt(int64(value), 10)
	}
	return strconv.FormatFloat(value, 'f', -1, 64)
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

// Normalize 去掉表达式两端空白。
func Normalize(expr string) string {
	return strings.TrimSpace(expr)
}
