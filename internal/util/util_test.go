package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type user struct {
	Name    string
	Profile struct {
		AccessGroups []string
	}
}

func TestCheckProperty(t *testing.T) {

	testuser := user{
		"Bob",
		struct {
			AccessGroups []string
		}{
			[]string{"group1", "group2"},
		},
	}

	cases := []struct {
		name     string
		data     any
		path     string
		value    any
		expected bool
		throwErr bool
	}{
		{"valid name", testuser, "Name", "Bob", true, false},
		{"invalid field", testuser, "Invalid", "Bob", false, true},
		{"valid nested slice", testuser, "Profile.AccessGroups", "group2", true, false},
		{"missing element", testuser, "Profile.AccessGroups", "group3", false, false},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			result, err := CheckProperty(test.data, test.path, test.value)

			if test.throwErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, test.expected, result)
		})
	}
}
