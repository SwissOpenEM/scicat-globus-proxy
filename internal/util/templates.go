// Utilities for templating strings more easily
package util

import (
	"bytes"
	"io"
	"strings"
	"text/template"
)

// Define a template with a type-checked data type T
type TypedTemplate[T any] struct {
	template.Template
}

func (tpl *TypedTemplate[T]) Execute(wr io.Writer, data T) error {
	return tpl.Template.Execute(wr, data)
}

func (tpl *TypedTemplate[T]) ExecuteStr(data T) (string, error) {
	var buf bytes.Buffer
	err := tpl.Execute(&buf, data)
	return buf.String(), err
}

// Functions to make available in all templates
var templateFuncs = template.FuncMap{
	"replace": func(s string, query string, repl string) string {
		return strings.ReplaceAll(s, query, repl)
	},
}

func NewTypedTemplate[T any](templateStr string) (*TypedTemplate[T], error) {
	tpl, err := template.New("").Funcs(templateFuncs).Parse(templateStr)

	if err != nil {
		return nil, err
	}
	return &TypedTemplate[T]{
		*tpl,
	}, nil
}

// Helper function to execute a template once
func ExecuteTemplate(templateStr string, context any) (string, error) {
	tpl, err := template.New(templateStr).Funcs(templateFuncs).Parse(templateStr)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	err = tpl.Execute(&buf, context)
	return buf.String(), err
}
