package builtin

import (
	"fmt"
	"strings"
)

// ValidateSelectOnly 对模型生成的 SQL 做代码层只读校验。
//
// 生产环境还应使用数据库只读账号、查询超时和行数限制。
func ValidateSelectOnly(query string) error {
	q := strings.TrimSpace(strings.ToLower(query))
	if !strings.HasPrefix(q, "select") {
		return fmt.Errorf("只允许 SELECT 查询")
	}
	trimmed := strings.TrimSuffix(q, ";")
	if strings.Contains(trimmed, ";") {
		return fmt.Errorf("禁止多条语句")
	}
	for _, keyword := range []string{"insert", "update", "delete", "drop", "alter", "truncate", "grant"} {
		if strings.Contains(q, keyword) {
			return fmt.Errorf("检测到禁止的关键字: %s", keyword)
		}
	}
	return nil
}
