package util

import (
	"fmt"
	"reflect"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Dynamically resolve a dot-separated property path and check if the result matches a value
func CheckProperty(data any, path string, value any) (bool, error) {
	splitPath := strings.Split(path, ".")
	return checkProperty(data, splitPath, value)
}

// Resolve the property path within data. If path is valid, check that the value either
// matches or is contained within the matched array. If path doesn't resolve, return an
// error.
func checkProperty(data any, path []string, value any) (bool, error) {
	// base case, fully resolved
	if len(path) == 0 {

		data_mirror := reflect.Indirect(reflect.ValueOf(data))
		switch data_mirror.Kind() {
		case reflect.Array, reflect.Slice:
			// For slices, check if the slice contains our value
			value_mirror := reflect.ValueOf(value)

			for i := 0; i < data_mirror.Len(); i++ {
				if data_mirror.Index(i).Interface() == value_mirror.Interface() {
					return true, nil
				}
			}
			return false, nil
		default:
			// Other types must match the value exactly
			return data == value, nil
		}
	}

	// resolve the first path item & recurse
	fieldName := path[0]
	v := reflect.Indirect(reflect.ValueOf(data))
	switch v.Kind() {
	case reflect.Struct:
		// Struct fields follow Go naming (exported/capitalized), while the
		// configured path uses the lowerCamelCase convention of the JSON tags.
		newData := v.FieldByName(Capitalize(fieldName))
		if newData.Kind() == reflect.Invalid {
			return false, fmt.Errorf("can't get field %s", fieldName)
		}
		return checkProperty(newData.Interface(), path[1:], value)
	case reflect.Map:
		// Maps (e.g. arbitrary OIDC claims) are keyed exactly as they appear in
		// the source JSON, so look them up without capitalizing.
		newData := v.MapIndex(reflect.ValueOf(fieldName))
		if !newData.IsValid() {
			return false, fmt.Errorf("can't get field %s", fieldName)
		}
		return checkProperty(newData.Interface(), path[1:], value)
	default:
		return false, fmt.Errorf("can't get field %s", fieldName)
	}
}

// Capitalize the first letter of a string
// Uses unicode title case rules. If the first character is not capitalizable, the string is returned unchanged.
func Capitalize(text string) string {
	if text == "" {
		return text
	}
	r, size := utf8.DecodeRuneInString(text)
	if r == utf8.RuneError {
		return text
	}
	return string(unicode.ToTitle(r)) + text[size:]
}
