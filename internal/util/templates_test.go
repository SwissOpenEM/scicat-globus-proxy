package util

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

type testContext struct {
	Name string
}

func TestTypedTemplate(t *testing.T) {

	tplStr := `Name: {{ .Name }}`
	expected := `Name: Tintin`
	context := testContext{"Tintin"}
	tpl, err := NewTypedTemplate[testContext](tplStr)
	assert.Nil(t, err)

	// ExecuteStr
	result, err := tpl.ExecuteStr(context)
	assert.Nil(t, err)
	assert.Equal(t, expected, result)

	// Execute
	var buf bytes.Buffer
	err = tpl.Execute(&buf, context)
	assert.Nil(t, err)
	assert.Equal(t, expected, buf.String())

	// Direct ExecuteTemplate
	result, err = ExecuteTemplate(tplStr, context)
	assert.Nil(t, err)
	assert.Equal(t, expected, result)
}

func TestReplace(t *testing.T) {
	tplStr := `Name: {{ replace .Name "in" "om" }}`
	expected := `Name: Tomtom`
	context := testContext{"Tintin"}
	tpl, err := NewTypedTemplate[testContext](tplStr)
	assert.Nil(t, err)

	result, err := tpl.ExecuteStr(context)
	assert.Nil(t, err)

	assert.Equal(t, expected, result)

	result, err = ExecuteTemplate(tplStr, context)
	assert.Nil(t, err)
	assert.Equal(t, expected, result)
}
