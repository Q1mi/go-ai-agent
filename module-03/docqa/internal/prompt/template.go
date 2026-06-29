package prompt

import (
	"bytes"
	"text/template"
)

// Template 是对 text/template 的薄封装。
//
// 它统一启用 missingkey=error，避免模板字段写错时悄悄渲染为空。
type Template struct {
	tmpl *template.Template
}

// New 创建一个 Prompt 模板。
func New(name, text string) (*Template, error) {
	tmpl, err := template.New(name).Option("missingkey=error").Parse(text)
	if err != nil {
		return nil, err
	}
	return &Template{tmpl: tmpl}, nil
}

// Render 用传入数据渲染模板文本。
func (tmpl *Template) Render(data any) (string, error) {
	var buffer bytes.Buffer
	if err := tmpl.tmpl.Execute(&buffer, data); err != nil {
		return "", err
	}
	return buffer.String(), nil
}
